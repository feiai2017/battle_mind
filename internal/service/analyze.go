package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/feiai2017/battle_mind/internal/llm"
	"github.com/feiai2017/battle_mind/internal/model"
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

func (s *AnalyzeService) Analyze(ctx context.Context, req model.AnalyzeRequest) (model.AnalyzeResult, error) {
	if s == nil || s.client == nil {
		return model.AnalyzeResult{}, fmt.Errorf("analyze service is not initialized")
	}
	if err := req.NormalizeAndValidate(); err != nil {
		return model.AnalyzeResult{}, fmt.Errorf("validate analyze request: %w", err)
	}

	requestID := llm.RequestIDFromContext(ctx)
	log.Printf("component=analyze_service request_id=%s event=analyze_start", requestID)

	prompt, err := buildAnalyzePrompt(req)
	if err != nil {
		log.Printf("component=analyze_service request_id=%s event=prompt_build_failed error=%q", requestID, err.Error())
		return model.AnalyzeResult{}, fmt.Errorf("build analyze prompt: %w", err)
	}
	log.Printf(
		"component=analyze_service request_id=%s event=prompt_built prompt_len=%d build_tags_count=%d",
		requestID,
		len(prompt),
		len(req.BuildTags),
	)

	text, err := s.client.Generate(ctx, prompt)
	if err != nil {
		log.Printf("component=analyze_service request_id=%s event=generate_failed error=%q", requestID, err.Error())
		return model.AnalyzeResult{}, fmt.Errorf("generate analyze text: %w", err)
	}

	result, err := s.parseAnalyzeResultWithRepair(ctx, text)
	if err != nil {
		log.Printf(
			"component=analyze_service request_id=%s event=analyze_failed error=%q raw_text=%q",
			requestID,
			err.Error(),
			shrinkForLog(text, 256),
		)
		return model.AnalyzeResult{}, err
	}

	result.RawText = text
	log.Printf(
		"component=analyze_service request_id=%s event=analyze_done summary_len=%d issues=%d suggestions=%d raw_text_len=%d",
		requestID,
		len(result.Summary),
		len(result.Issues),
		len(result.Suggestions),
		len(text),
	)

	return result, nil
}

func (s *AnalyzeService) parseAnalyzeResultWithRepair(ctx context.Context, raw string) (model.AnalyzeResult, error) {
	result, err := parseAnalyzeResult(raw)
	if err == nil {
		return result, nil
	}

	requestID := llm.RequestIDFromContext(ctx)
	log.Printf(
		"component=analyze_service request_id=%s event=parse_failed attempt=1 error=%q raw_text=%q",
		requestID,
		err.Error(),
		shrinkForLog(raw, 256),
	)

	repairPrompt := buildRepairJSONPrompt(raw)
	log.Printf(
		"component=analyze_service request_id=%s event=repair_started prompt_len=%d raw_text_len=%d",
		requestID,
		len(repairPrompt),
		len(strings.TrimSpace(raw)),
	)

	repairedText, repairErr := s.client.Generate(ctx, repairPrompt)
	if repairErr != nil {
		log.Printf(
			"component=analyze_service request_id=%s event=repair_generate_failed error=%q",
			requestID,
			repairErr.Error(),
		)
		return model.AnalyzeResult{}, fmt.Errorf("%w: model output is not valid JSON and repair also failed", ErrModelJSONRepairFailed)
	}

	result, err = parseAnalyzeResult(repairedText)
	if err != nil {
		log.Printf(
			"component=analyze_service request_id=%s event=repair_parse_failed attempt=2 error=%q repaired_text=%q",
			requestID,
			err.Error(),
			shrinkForLog(repairedText, 256),
		)
		return model.AnalyzeResult{}, fmt.Errorf("%w: model output is not valid JSON and could not be repaired", ErrModelJSONRepairFailed)
	}

	log.Printf(
		"component=analyze_service request_id=%s event=repair_succeeded repaired_text_len=%d",
		requestID,
		len(strings.TrimSpace(repairedText)),
	)
	return result, nil
}

func buildAnalyzePrompt(req model.AnalyzeRequest) (string, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("You are a game battle analysis assistant.\n")
	builder.WriteString("Read the normalized battle input and return JSON only.\n")
	builder.WriteString("Do not output explanations, markdown, or code fences.\n\n")
	builder.WriteString("Output schema:\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"summary\": \"one sentence summary\",\n")
	builder.WriteString("  \"issues\": [\n")
	builder.WriteString("    {\n")
	builder.WriteString("      \"title\": \"issue title\",\n")
	builder.WriteString("      \"description\": \"issue description\",\n")
	builder.WriteString("      \"severity\": \"low|medium|high\",\n")
	builder.WriteString("      \"evidence\": [\"evidence 1\", \"evidence 2\"]\n")
	builder.WriteString("    }\n")
	builder.WriteString("  ],\n")
	builder.WriteString("  \"suggestions\": [\"suggestion 1\", \"suggestion 2\"]\n")
	builder.WriteString("}\n\n")
	builder.WriteString("Requirements:\n")
	builder.WriteString("1. summary must be a non-empty string.\n")
	builder.WriteString("2. issues must be an array.\n")
	builder.WriteString("3. each issue must include title, description, severity, evidence.\n")
	builder.WriteString("4. severity must be one of low, medium, high.\n")
	builder.WriteString("5. suggestions must be an array of strings.\n")
	builder.WriteString("6. return valid JSON even when information is limited.\n\n")
	builder.WriteString("Input notes:\n")
	builder.WriteString("- log_text may be present for legacy callers.\n")
	builder.WriteString("- metadata, summary, metrics, diagnosis are the primary normalized analysis input.\n")
	builder.WriteString("- Use all available fields to produce the result.\n\n")
	builder.WriteString("Normalized battle input:\n")
	builder.Write(reqJSON)
	return builder.String(), nil
}

func buildRepairJSONPrompt(raw string) string {
	var builder strings.Builder
	builder.WriteString("The following model output should have been valid JSON for AnalyzeResult but it is invalid.\n")
	builder.WriteString("Repair it into valid JSON without changing the intended meaning.\n")
	builder.WriteString("Return JSON only.\n")
	builder.WriteString("Do not output markdown, explanations, comments, or code fences.\n\n")
	builder.WriteString("Required schema:\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"summary\": \"string\",\n")
	builder.WriteString("  \"issues\": [\n")
	builder.WriteString("    {\n")
	builder.WriteString("      \"title\": \"string\",\n")
	builder.WriteString("      \"description\": \"string\",\n")
	builder.WriteString("      \"severity\": \"low|medium|high\",\n")
	builder.WriteString("      \"evidence\": [\"string\"]\n")
	builder.WriteString("    }\n")
	builder.WriteString("  ],\n")
	builder.WriteString("  \"suggestions\": [\"string\"]\n")
	builder.WriteString("}\n\n")
	builder.WriteString("If the original output used legacy field \"problems\", keep the same meaning but convert it into \"issues\".\n")
	builder.WriteString("Invalid model output:\n")
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

	cutSet := "，。,；;：:"
	for _, separator := range cutSet {
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

func shrinkForLog(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "...(truncated)"
}
