package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/feiai2017/battle_mind/internal/llm"
	"github.com/feiai2017/battle_mind/internal/logging"
	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/output"
	"github.com/feiai2017/battle_mind/internal/prompt"
	"github.com/feiai2017/battle_mind/internal/rules"
)

var (
	ErrInvalidLLMJSON        = errors.New("invalid llm json output")
	ErrInvalidAnalyzeResult  = errors.New("invalid analyze result")
	ErrModelJSONRepairFailed = errors.New("model json repair failed")
)

type llmModelSuggestionBlock struct {
	Summary     string   `json:"summary"`
	Suggestions []string `json:"suggestions"`
	Risks       []string `json:"risks"`
}

type textGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type modelNamer interface {
	ModelName() string
}

// AnalyzeService handles prompt building, rule-summary assembly, model invocation,
// JSON parsing, and one repair retry.
type AnalyzeService struct {
	client textGenerator
}

func NewAnalyzeService(client *llm.Client) *AnalyzeService {
	return &AnalyzeService{client: client}
}

func (s *AnalyzeService) ModelName() string {
	if s == nil || s.client == nil {
		return ""
	}
	namer, ok := s.client.(modelNamer)
	if !ok {
		return ""
	}
	return strings.TrimSpace(namer.ModelName())
}

func (s *AnalyzeService) Analyze(ctx context.Context, input model.AnalyzeInput) (output.AnalyzeOutput, error) {
	if s == nil || s.client == nil {
		return output.AnalyzeOutput{}, fmt.Errorf("analyze service is not initialized")
	}

	req := input.Request
	if err := req.NormalizeAndValidate(); err != nil {
		return output.AnalyzeOutput{}, fmt.Errorf("validate analyze request: %w", err)
	}
	input.Request = req

	requestID := llm.RequestIDFromContext(ctx)
	ruleSummary := buildRuleSummary(input)
	logging.LogFields("analyze_service", "analyze_start", requestID, map[string]any{
		"has_report":               input.Report != nil,
		"hard_findings_count":      len(ruleSummary.HardFindings),
		"suspicious_signals_count": len(ruleSummary.SuspiciousSignals),
	})

	promptText, err := buildAnalyzePrompt(input, ruleSummary)
	if err != nil {
		logging.LogFields("analyze_service", "prompt_build_fail", requestID, map[string]any{
			"error": err.Error(),
		})
		return output.AnalyzeOutput{}, fmt.Errorf("build analyze prompt: %w", err)
	}
	logging.LogFields("analyze_service", "prompt_built", requestID, map[string]any{
		"build_tags_count":         len(req.BuildTags),
		"has_report":               input.Report != nil,
		"prompt_len":               len(promptText),
		"hard_findings_count":      len(ruleSummary.HardFindings),
		"suspicious_signals_count": len(ruleSummary.SuspiciousSignals),
	})
	logging.LogTextBlock("analyze_service", "llm_prompt", requestID, promptText)

	text, err := s.client.Generate(ctx, promptText)
	if err != nil {
		logging.LogFields("analyze_service", "generate_fail", requestID, map[string]any{
			"error": err.Error(),
		})
		return output.AnalyzeOutput{}, fmt.Errorf("generate analyze text: %w", err)
	}
	logging.LogTextBlock("analyze_service", "llm_raw_response", requestID, text)

	modelSuggestions, err := s.parseAnalyzeResultWithRepair(ctx, text)
	if err != nil {
		logging.LogFields("analyze_service", "analyze_fail", requestID, map[string]any{
			"error":        err.Error(),
			"raw_text_len": len(text),
		})
		return output.AnalyzeOutput{}, err
	}

	result := output.BuildAnalyzeOutput(ruleSummary, modelSuggestions)
	result.RawText = text
	logging.LogFields("analyze_service", "analyze_done", requestID, map[string]any{
		"rule_findings": len(result.RuleFindings.Findings),
		"raw_text_len":  len(text),
		"summary_len":   len(result.ModelSuggestions.Summary),
		"suggestions":   len(result.ModelSuggestions.Suggestions),
		"risks":         len(result.ModelSuggestions.Risks),
	})
	logging.LogJSONBlock("analyze_service", "normalized_result", requestID, result)

	return result, nil
}

func (s *AnalyzeService) parseAnalyzeResultWithRepair(ctx context.Context, raw string) (output.ModelSuggestionBlock, error) {
	result, err := parseAnalyzeResult(raw)
	if err == nil {
		return result, nil
	}

	requestID := llm.RequestIDFromContext(ctx)
	logging.LogFields("analyze_service", "parse_fail", requestID, map[string]any{
		"attempt":      1,
		"error":        err.Error(),
		"raw_text_len": len(strings.TrimSpace(raw)),
	})

	repairPrompt := buildRepairJSONPrompt(raw)
	logging.LogFields("analyze_service", "repair_start", requestID, map[string]any{
		"prompt_len":   len(repairPrompt),
		"raw_text_len": len(strings.TrimSpace(raw)),
	})
	logging.LogTextBlock("analyze_service", "llm_repair_prompt", requestID, repairPrompt)

	repairedText, repairErr := s.client.Generate(ctx, repairPrompt)
	if repairErr != nil {
		logging.LogFields("analyze_service", "repair_generate_fail", requestID, map[string]any{
			"error": repairErr.Error(),
		})
		return output.ModelSuggestionBlock{}, fmt.Errorf("%w: model output is not valid JSON and repair also failed", ErrModelJSONRepairFailed)
	}
	logging.LogTextBlock("analyze_service", "llm_repair_response", requestID, repairedText)

	result, err = parseAnalyzeResult(repairedText)
	if err != nil {
		logging.LogFields("analyze_service", "repair_parse_fail", requestID, map[string]any{
			"attempt":           2,
			"error":             err.Error(),
			"repaired_text_len": len(strings.TrimSpace(repairedText)),
		})
		return output.ModelSuggestionBlock{}, fmt.Errorf("%w: model output is not valid JSON and could not be repaired", ErrModelJSONRepairFailed)
	}

	logging.LogFields("analyze_service", "repair_done", requestID, map[string]any{
		"repaired_text_len": len(strings.TrimSpace(repairedText)),
	})
	return result, nil
}

func buildRuleSummary(input model.AnalyzeInput) rules.RuleSummary {
	if input.Report == nil {
		return rules.BuildRuleSummary(nil)
	}
	return rules.BuildRuleSummary(rules.NormalizeRuleEvents(*input.Report))
}

func buildAnalyzePrompt(input model.AnalyzeInput, ruleSummary rules.RuleSummary) (string, error) {
	return prompt.BuildAnalyzePrompt(prompt.AnalyzePromptInput{
		Request:     input.Request,
		Report:      input.Report,
		RuleSummary: ruleSummary,
	}), nil
}

func buildRepairJSONPrompt(raw string) string {
	var builder strings.Builder
	builder.WriteString("下面这段模型输出本应是 model_suggestions 的合法 JSON，但当前格式非法。\n")
	builder.WriteString("请在不改变原始语义的前提下修复为合法 JSON。\n")
	builder.WriteString("只输出 JSON。\n")
	builder.WriteString("不要输出 markdown、解释、注释或代码块。\n\n")
	builder.WriteString("目标格式：\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"summary\": \"字符串\",\n")
	builder.WriteString("  \"suggestions\": [\"字符串\"],\n")
	builder.WriteString("  \"risks\": [\"字符串\"]\n")
	builder.WriteString("}\n\n")
	builder.WriteString("所有自然语言字段必须使用简体中文。\n")
	builder.WriteString("待修复的模型输出：\n")
	builder.WriteString(strings.TrimSpace(raw))
	return builder.String()
}

func parseAnalyzeResult(raw string) (output.ModelSuggestionBlock, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return output.ModelSuggestionBlock{}, fmt.Errorf("%w: empty response text", ErrInvalidLLMJSON)
	}

	parsed, err := unmarshalLLMAnalyzeResult(raw)
	if err != nil {
		cleaned := extractJSON(raw)
		if cleaned == "" {
			return output.ModelSuggestionBlock{}, fmt.Errorf("%w: %v", ErrInvalidLLMJSON, err)
		}
		parsed, err = unmarshalLLMAnalyzeResult(cleaned)
		if err != nil {
			return output.ModelSuggestionBlock{}, fmt.Errorf("%w: %v", ErrInvalidLLMJSON, err)
		}
	}

	result := output.ModelSuggestionBlock{
		Summary:     parsed.Summary,
		Suggestions: parsed.Suggestions,
		Risks:       parsed.Risks,
	}
	if err := normalizeModelSuggestionBlock(&result); err != nil {
		return output.ModelSuggestionBlock{}, err
	}

	return result, nil
}

func unmarshalLLMAnalyzeResult(raw string) (llmModelSuggestionBlock, error) {
	var result llmModelSuggestionBlock
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return llmModelSuggestionBlock{}, err
	}
	return result, nil
}

func extractJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	codeBlock := stripCodeFence(trimmed)
	if codeBlock != "" {
		trimmed = codeBlock
	}

	if jsonObject := extractFirstJSONObject(trimmed); jsonObject != "" {
		return strings.TrimSpace(jsonObject)
	}
	return ""
}

func stripCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return ""
	}

	withoutPrefix := strings.TrimPrefix(trimmed, "```")
	withoutPrefix = strings.TrimLeft(withoutPrefix, " \t\r\n")
	if strings.HasPrefix(strings.ToLower(withoutPrefix), "json") {
		withoutPrefix = withoutPrefix[4:]
	}

	end := strings.LastIndex(withoutPrefix, "```")
	if end == -1 {
		return strings.TrimSpace(withoutPrefix)
	}
	return strings.TrimSpace(withoutPrefix[:end])
}

func extractFirstJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(raw); i++ {
		ch := raw[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}

	return ""
}

func normalizeModelSuggestionBlock(result *output.ModelSuggestionBlock) error {
	result.Summary = strings.TrimSpace(result.Summary)
	result.Suggestions = normalizeStringSlice(result.Suggestions)
	result.Risks = normalizeStringSlice(result.Risks)

	if result.Suggestions == nil {
		result.Suggestions = []string{}
	}
	if result.Risks == nil {
		result.Risks = []string{}
	}
	if result.Summary == "" {
		return fmt.Errorf("%w: summary is required", ErrInvalidAnalyzeResult)
	}
	return nil
}

func normalizeStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
