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
	ErrInvalidLLMJSON       = errors.New("invalid llm json output")
	ErrInvalidAnalyzeResult = errors.New("invalid analyze result")
)

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
		"component=analyze_service request_id=%s event=analyze_done summary_len=%d problems=%d suggestions=%d raw_text_len=%d",
		requestID,
		len(result.Summary),
		len(result.Problems),
		len(result.Suggestions),
		len(text),
	)

	return result, nil
}

func buildAnalyzePrompt(req model.AnalyzeRequest) (string, error) {
	reqJSON, err := json.Marshal(req)
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
	builder.WriteString("\n  \"problems\": [\"问题1\", \"问题2\"],")
	builder.WriteString("\n  \"suggestions\": [\"建议1\", \"建议2\"]")
	builder.WriteString("\n}")
	builder.WriteString("\n\n要求：")
	builder.WriteString("\n1. summary 必须是字符串且不能为空。")
	builder.WriteString("\n2. problems 必须是字符串数组。")
	builder.WriteString("\n3. suggestions 必须是字符串数组。")
	builder.WriteString("\n4. 即使信息不足也要返回合法 JSON。")
	builder.WriteString("\n\n规范化战斗输入：\n")
	builder.Write(reqJSON)
	return builder.String(), nil
}

func parseAnalyzeResult(raw string) (model.AnalyzeResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.AnalyzeResult{}, fmt.Errorf("%w: empty response text", ErrInvalidLLMJSON)
	}

	parsed, err := unmarshalAnalyzeResult(raw)
	if err != nil {
		cleaned := extractJSON(raw)
		if cleaned == "" {
			return model.AnalyzeResult{}, fmt.Errorf("%w: %v", ErrInvalidLLMJSON, err)
		}
		parsed, err = unmarshalAnalyzeResult(cleaned)
		if err != nil {
			return model.AnalyzeResult{}, fmt.Errorf("%w: %v", ErrInvalidLLMJSON, err)
		}
	}

	if err := normalizeAnalyzeResult(&parsed); err != nil {
		return model.AnalyzeResult{}, err
	}

	return parsed, nil
}

func unmarshalAnalyzeResult(raw string) (model.AnalyzeResult, error) {
	var result model.AnalyzeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return model.AnalyzeResult{}, err
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

func normalizeAnalyzeResult(result *model.AnalyzeResult) error {
	result.Summary = strings.TrimSpace(result.Summary)
	result.Problems = normalizeStringSlice(result.Problems)
	result.Suggestions = normalizeStringSlice(result.Suggestions)

	if result.Problems == nil {
		result.Problems = []string{}
	}
	if result.Suggestions == nil {
		result.Suggestions = []string{}
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
	return result
}

func shrinkForLog(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "...(truncated)"
}
