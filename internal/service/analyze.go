package service

import (
	"bytes"
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
	ErrInvalidLLMJSON       = errors.New("invalid llm json output")
	ErrInvalidAnalyzeResult = errors.New("invalid analyze result")
)

const (
	maxPromptDiagnosisItems = 20
	maxPromptDetailsChars   = 512
)

type analyzePromptInput struct {
	Metadata  analyzePromptMetadata    `json:"metadata"`
	Summary   model.BattleSummary      `json:"summary"`
	Metrics   model.BattleMetrics      `json:"metrics"`
	Diagnosis []analyzePromptDiagnosis `json:"diagnosis,omitempty"`
}

type analyzePromptMetadata struct {
	BattleType     string   `json:"battle_type"`
	BuildTags      []string `json:"build_tags,omitempty"`
	FloorID        string   `json:"floor_id,omitempty"`
	NotableRules   []string `json:"notable_rules,omitempty"`
	FloorModifiers []string `json:"floor_modifiers,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

type analyzePromptDiagnosis struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Details  string `json:"details,omitempty"`
}

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

// AnalyzeService 串联 prompt 组装与模型调用。
type AnalyzeService struct {
	client *llm.Client
}

func NewAnalyzeService(client *llm.Client) *AnalyzeService {
	return &AnalyzeService{client: client}
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
		"component=analyze_service request_id=%s event=prompt_built prompt_len=%d diagnosis_count=%d",
		requestID,
		len(prompt),
		len(req.Diagnosis),
	)

	text, err := s.client.Generate(ctx, prompt)
	if err != nil {
		log.Printf("component=analyze_service request_id=%s event=generate_failed error=%q", requestID, err.Error())
		return model.AnalyzeResult{}, fmt.Errorf("generate analyze text: %w", err)
	}

	result, err := parseAnalyzeResult(text)
	if err != nil {
		log.Printf(
			"component=analyze_service request_id=%s event=parse_failed error=%q raw_text=%q",
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

func buildAnalyzePrompt(req model.AnalyzeRequest) (string, error) {
	promptInput := buildPromptInput(req)
	reqJSON, err := json.Marshal(promptInput)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("你是一个游戏战斗日志分析助手。")
	builder.WriteString("请阅读下面的规范化战斗输入，并只输出 JSON。")
	builder.WriteString("不要输出解释，不要输出 markdown 代码块，不要输出额外前后缀。")
	builder.WriteString("\n\n输出格式：")
	builder.WriteString("\n{")
	builder.WriteString("\n  \"summary\": \"一句话总结\",")
	builder.WriteString("\n  \"issues\": [")
	builder.WriteString("\n    {")
	builder.WriteString("\n      \"title\": \"问题标题\",")
	builder.WriteString("\n      \"description\": \"问题描述\",")
	builder.WriteString("\n      \"severity\": \"low|medium|high\",")
	builder.WriteString("\n      \"evidence\": [\"证据1\", \"证据2\"]")
	builder.WriteString("\n    }")
	builder.WriteString("\n  ],")
	builder.WriteString("\n  \"suggestions\": [\"建议1\", \"建议2\"]")
	builder.WriteString("\n}")
	builder.WriteString("\n\n要求：")
	builder.WriteString("\n1. summary 必须是字符串且不能为空。")
	builder.WriteString("\n2. issues 必须是数组。")
	builder.WriteString("\n3. 每个 issue 必须包含 title、description、severity、evidence。")
	builder.WriteString("\n4. severity 只能是 low、medium、high。")
	builder.WriteString("\n5. suggestions 必须是字符串数组。")
	builder.WriteString("\n6. 即使信息不足也要返回合法 JSON。")
	builder.WriteString("\n7. 不要输出 markdown 代码块。")
	builder.WriteString("\n\n规范化战斗输入：\n")
	builder.Write(reqJSON)
	return builder.String(), nil
}

func buildPromptInput(req model.AnalyzeRequest) analyzePromptInput {
	input := analyzePromptInput{
		Metadata: analyzePromptMetadata{
			BattleType:     req.Metadata.BattleType,
			BuildTags:      req.Metadata.BuildTags,
			FloorID:        req.Metadata.FloorID,
			NotableRules:   req.Metadata.NotableRules,
			FloorModifiers: req.Metadata.FloorModifiers,
			Notes:          req.Metadata.Notes,
		},
		Summary: req.Summary,
		Metrics: req.Metrics,
	}

	diagnosis := req.Diagnosis
	if len(diagnosis) > maxPromptDiagnosisItems {
		diagnosis = diagnosis[:maxPromptDiagnosisItems]
	}
	if len(diagnosis) > 0 {
		input.Diagnosis = make([]analyzePromptDiagnosis, 0, len(diagnosis))
		for _, item := range diagnosis {
			input.Diagnosis = append(input.Diagnosis, analyzePromptDiagnosis{
				Code:     item.Code,
				Severity: item.Severity,
				Message:  item.Message,
				Details:  diagnosisDetailsPreview(item.Details),
			})
		}
	}

	return input
}

func diagnosisDetailsPreview(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		trimmed = compact.String()
	}
	if len(trimmed) <= maxPromptDetailsChars {
		return trimmed
	}
	return trimmed[:maxPromptDetailsChars] + "...(truncated)"
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
		return jsonObject
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
