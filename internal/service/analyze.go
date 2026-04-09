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
	"github.com/feiai2017/battle_mind/internal/prompt"
	"github.com/feiai2017/battle_mind/internal/rules"
)

var (
	ErrInvalidLLMJSON        = errors.New("invalid llm json output")
	ErrInvalidAnalyzeResult  = errors.New("invalid analyze result")
	ErrModelJSONRepairFailed = errors.New("model json repair failed")
)

type llmAnalyzeResult struct {
	Summary     string            `json:"summary"`
	Issues      []llmAnalyzeIssue `json:"issues"`
	Problems    []string          `json:"problems"`
	Suggestions []string          `json:"suggestions"`
}

type llmAnalyzeIssue struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Evidence    []string `json:"evidence"`
}

type textGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type modelNamer interface {
	ModelName() string
}

// AnalyzeService handles prompt building, model invocation, JSON parsing, and one repair retry.
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

func (s *AnalyzeService) Analyze(ctx context.Context, input model.AnalyzeInput) (model.AnalyzeResult, error) {
	if s == nil || s.client == nil {
		return model.AnalyzeResult{}, fmt.Errorf("analyze service is not initialized")
	}

	req := input.Request
	if err := req.NormalizeAndValidate(); err != nil {
		return model.AnalyzeResult{}, fmt.Errorf("validate analyze request: %w", err)
	}
	input.Request = req

	requestID := llm.RequestIDFromContext(ctx)
	logging.LogFields("analyze_service", "analyze_start", requestID, map[string]any{
		"has_report": input.Report != nil,
	})

	promptText, err := buildAnalyzePrompt(input)
	if err != nil {
		logging.LogFields("analyze_service", "prompt_build_fail", requestID, map[string]any{
			"error": err.Error(),
		})
		return model.AnalyzeResult{}, fmt.Errorf("build analyze prompt: %w", err)
	}
	logging.LogFields("analyze_service", "prompt_built", requestID, map[string]any{
		"build_tags_count": len(req.BuildTags),
		"has_report":       input.Report != nil,
		"prompt_len":       len(promptText),
	})
	logging.LogTextBlock("analyze_service", "llm_prompt", requestID, promptText)

	text, err := s.client.Generate(ctx, promptText)
	if err != nil {
		logging.LogFields("analyze_service", "generate_fail", requestID, map[string]any{
			"error": err.Error(),
		})
		return model.AnalyzeResult{}, fmt.Errorf("generate analyze text: %w", err)
	}
	logging.LogTextBlock("analyze_service", "llm_raw_response", requestID, text)

	result, err := s.parseAnalyzeResultWithRepair(ctx, text)
	if err != nil {
		logging.LogFields("analyze_service", "analyze_fail", requestID, map[string]any{
			"error":        err.Error(),
			"raw_text_len": len(text),
		})
		return model.AnalyzeResult{}, err
	}

	result.RawText = text
	logging.LogFields("analyze_service", "analyze_done", requestID, map[string]any{
		"issues":       len(result.Issues),
		"raw_text_len": len(text),
		"summary_len":  len(result.Summary),
		"suggestions":  len(result.Suggestions),
	})
	logging.LogJSONBlock("analyze_service", "normalized_result", requestID, result)

	return result, nil
}

func (s *AnalyzeService) parseAnalyzeResultWithRepair(ctx context.Context, raw string) (model.AnalyzeResult, error) {
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
		return model.AnalyzeResult{}, fmt.Errorf("%w: model output is not valid JSON and repair also failed", ErrModelJSONRepairFailed)
	}
	logging.LogTextBlock("analyze_service", "llm_repair_response", requestID, repairedText)

	result, err = parseAnalyzeResult(repairedText)
	if err != nil {
		logging.LogFields("analyze_service", "repair_parse_fail", requestID, map[string]any{
			"attempt":           2,
			"error":             err.Error(),
			"repaired_text_len": len(strings.TrimSpace(repairedText)),
		})
		return model.AnalyzeResult{}, fmt.Errorf("%w: model output is not valid JSON and could not be repaired", ErrModelJSONRepairFailed)
	}

	logging.LogFields("analyze_service", "repair_done", requestID, map[string]any{
		"repaired_text_len": len(strings.TrimSpace(repairedText)),
	})
	return result, nil
}

func buildAnalyzePrompt(input model.AnalyzeInput) (string, error) {
	ruleSummary := rules.BuildRuleSummary(nil)
	if input.Report != nil {
		ruleSummary = rules.BuildRuleSummary(rules.NormalizeRuleEvents(*input.Report))
	}

	return prompt.BuildAnalyzePrompt(prompt.AnalyzePromptInput{
		Request:     input.Request,
		Report:      input.Report,
		RuleSummary: ruleSummary,
	}), nil
}

func buildRepairJSONPrompt(raw string) string {
	var builder strings.Builder
	builder.WriteString("下面这段模型输出本应是 AnalyzeResult 的合法 JSON，但当前格式非法。\n")
	builder.WriteString("请在不改变原始语义的前提下修复为合法 JSON。\n")
	builder.WriteString("只输出 JSON。\n")
	builder.WriteString("不要输出 markdown、解释、注释或代码块。\n\n")
	builder.WriteString("目标格式：\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"summary\": \"字符串\",\n")
	builder.WriteString("  \"issues\": [\n")
	builder.WriteString("    {\n")
	builder.WriteString("      \"title\": \"字符串\",\n")
	builder.WriteString("      \"description\": \"字符串\",\n")
	builder.WriteString("      \"severity\": \"low|medium|high\",\n")
	builder.WriteString("      \"evidence\": [\"字符串\"]\n")
	builder.WriteString("    }\n")
	builder.WriteString("  ],\n")
	builder.WriteString("  \"suggestions\": [\"字符串\"]\n")
	builder.WriteString("}\n\n")
	builder.WriteString("如果原始输出使用了旧字段 \"problems\"，请保持语义不变并转换成 \"issues\"。\n")
	builder.WriteString("所有自然语言字段必须使用简体中文。\n")
	builder.WriteString("待修复的模型输出：\n")
	builder.WriteString(strings.TrimSpace(raw))
	return builder.String()
}

func parseAnalyzeResult(raw string) (model.AnalyzeResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.AnalyzeResult{}, fmt.Errorf("%w: empty response text", ErrInvalidLLMJSON)
	}

	parsed, err := unmarshalLLMAnalyzeResult(raw)
	if err != nil {
		cleaned := extractJSON(raw)
		if cleaned == "" {
			return model.AnalyzeResult{}, fmt.Errorf("%w: %v", ErrInvalidLLMJSON, err)
		}
		parsed, err = unmarshalLLMAnalyzeResult(cleaned)
		if err != nil {
			return model.AnalyzeResult{}, fmt.Errorf("%w: %v", ErrInvalidLLMJSON, err)
		}
	}

	result := model.AnalyzeResult{
		Summary:     parsed.Summary,
		Suggestions: parsed.Suggestions,
	}
	if len(parsed.Issues) > 0 {
		result.Issues = convertLLMIssues(parsed.Issues)
	} else if len(parsed.Problems) > 0 {
		result.Issues = convertProblemsToIssues(parsed.Problems, parsed.Summary)
	}

	if err := normalizeAnalyzeResult(&result); err != nil {
		return model.AnalyzeResult{}, err
	}

	return result, nil
}

func unmarshalLLMAnalyzeResult(raw string) (llmAnalyzeResult, error) {
	var result llmAnalyzeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return llmAnalyzeResult{}, err
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

func convertLLMIssues(items []llmAnalyzeIssue) []model.AnalyzeIssue {
	if len(items) == 0 {
		return nil
	}

	result := make([]model.AnalyzeIssue, 0, len(items))
	for _, item := range items {
		result = append(result, model.AnalyzeIssue{
			Title:       item.Title,
			Description: item.Description,
			Severity:    item.Severity,
			Evidence:    item.Evidence,
		})
	}
	return result
}

func convertProblemsToIssues(problems []string, summary string) []model.AnalyzeIssue {
	if len(problems) == 0 {
		return nil
	}

	result := make([]model.AnalyzeIssue, 0, len(problems))
	for _, problem := range problems {
		problem = strings.TrimSpace(problem)
		if problem == "" {
			continue
		}
		result = append(result, model.AnalyzeIssue{
			Title:       deriveIssueTitle(problem),
			Description: problem,
			Severity:    inferSeverityFromProblem(problem),
			Evidence:    buildEvidence(problem, summary, nil),
		})
	}
	return result
}

func normalizeAnalyzeResult(result *model.AnalyzeResult) error {
	result.Summary = strings.TrimSpace(result.Summary)
	result.Suggestions = normalizeStringSlice(result.Suggestions)
	result.Issues = normalizeIssues(result.Issues, result.Summary)

	if result.Issues == nil {
		result.Issues = []model.AnalyzeIssue{}
	}
	if result.Suggestions == nil {
		result.Suggestions = []string{}
	}
	if result.Summary == "" {
		return fmt.Errorf("%w: summary is required", ErrInvalidAnalyzeResult)
	}
	return nil
}

func normalizeIssues(items []model.AnalyzeIssue, summary string) []model.AnalyzeIssue {
	if len(items) == 0 {
		return nil
	}

	result := make([]model.AnalyzeIssue, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		description := strings.TrimSpace(item.Description)
		if title == "" && description == "" {
			continue
		}
		if title == "" {
			title = deriveIssueTitle(description)
		}
		if description == "" {
			description = title
		}

		result = append(result, model.AnalyzeIssue{
			Title:       title,
			Description: description,
			Severity:    normalizeSeverity(item.Severity),
			Evidence:    buildEvidence(description, summary, item.Evidence),
		})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func inferSeverityFromProblem(problem string) string {
	if containsAny(problem, "过低", "中断", "停止", "严重", "风险高", "不再") {
		return "high"
	}
	if containsAny(problem, "不足", "偏低", "偏高", "过长", "较慢") {
		return "medium"
	}
	return "low"
}

func deriveIssueTitle(problem string) string {
	problem = strings.TrimSpace(problem)
	if problem == "" {
		return ""
	}

	for _, separator := range []rune{'，', '。', '；', '：'} {
		if index := strings.IndexRune(problem, separator); index > 0 {
			return strings.TrimSpace(problem[:index])
		}
	}

	if len([]rune(problem)) > 18 {
		runes := []rune(problem)
		return strings.TrimSpace(string(runes[:18]))
	}
	return problem
}

func buildEvidence(problem, summary string, existing []string) []string {
	evidence := normalizeStringSlice(existing)
	if len(evidence) > 0 {
		return evidence
	}

	fallback := []string{}
	problem = strings.TrimSpace(problem)
	summary = strings.TrimSpace(summary)

	if problem != "" {
		fallback = append(fallback, problem)
	}
	if summary != "" && summary != problem {
		fallback = append(fallback, summary)
	}
	if len(fallback) == 0 {
		fallback = append(fallback, "由模型总结结果推断")
	}
	return fallback
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

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}
