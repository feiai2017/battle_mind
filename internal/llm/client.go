package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/feiai2017/battle_mind/internal/config"
)

var (
	ErrEmptyPrompt   = fmt.Errorf("prompt is empty")
	ErrEmptyResponse = fmt.Errorf("model response body is empty")
	ErrTextNotFound  = fmt.Errorf("model response text not found")
)

type contextKey string

const (
	requestIDContextKey      contextKey = "request_id"
	simulationModeContextKey contextKey = "simulation_mode"
)

type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("model api returned status %d: %s", e.StatusCode, e.Body)
}

// Client 封装最小模型文本生成调用。
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func (c *Client) ModelName() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.model)
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, strings.TrimSpace(requestID))
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(requestIDContextKey).(string)
	return strings.TrimSpace(value)
}

func WithSimulationMode(ctx context.Context, mode string) context.Context {
	return context.WithValue(ctx, simulationModeContextKey, strings.TrimSpace(mode))
}

func SimulationModeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(simulationModeContextKey).(string)
	return strings.TrimSpace(value)
}

func NewClient(cfg config.ModelConfig) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("llm config base_url is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("llm config api_key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("llm config model is required")
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:  strings.TrimSpace(cfg.APIKey),
		model:   strings.TrimSpace(cfg.Model),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	requestID := RequestIDFromContext(ctx)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		log.Printf("component=llm_client request_id=%s event=reject reason=empty_prompt", requestID)
		return "", ErrEmptyPrompt
	}
	if SimulationModeFromContext(ctx) == "timeout" {
		log.Printf("component=llm_client request_id=%s event=simulate_timeout model=%s", requestID, c.model)
		return "", simulatedTimeoutError{}
	}
	log.Printf(
		"component=llm_client request_id=%s event=generate_start model=%s prompt_len=%d",
		requestID,
		c.model,
		len(prompt),
	)
	startedAt := time.Now()

	reqBody := chatCompletionsRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role: "system",
				Content: "You are a game battle analysis assistant. " +
					"Provide concise and actionable analysis.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.2,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal model request: %w", err)
	}

	url := buildChatCompletionsURL(c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create model request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf(
			"component=llm_client request_id=%s event=request_failed duration_ms=%d error=%q",
			requestID,
			time.Since(startedAt).Milliseconds(),
			err.Error(),
		)
		return "", fmt.Errorf("request model api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read model response: %w", err)
	}
	if len(bytes.TrimSpace(respBody)) == 0 {
		log.Printf(
			"component=llm_client request_id=%s event=empty_response status=%d duration_ms=%d",
			requestID,
			resp.StatusCode,
			time.Since(startedAt).Milliseconds(),
		)
		return "", ErrEmptyResponse
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf(
			"component=llm_client request_id=%s event=bad_status status=%d duration_ms=%d body=%q",
			requestID,
			resp.StatusCode,
			time.Since(startedAt).Milliseconds(),
			shrinkBody(string(respBody), 256),
		)
		return "", &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       shrinkBody(string(respBody), 512),
		}
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse model response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		log.Printf(
			"component=llm_client request_id=%s event=text_missing duration_ms=%d",
			requestID,
			time.Since(startedAt).Milliseconds(),
		)
		return "", ErrTextNotFound
	}

	text := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if text == "" {
		log.Printf(
			"component=llm_client request_id=%s event=text_empty duration_ms=%d",
			requestID,
			time.Since(startedAt).Milliseconds(),
		)
		return "", ErrTextNotFound
	}

	log.Printf(
		"component=llm_client request_id=%s event=generate_done status=%d duration_ms=%d text_len=%d",
		requestID,
		resp.StatusCode,
		time.Since(startedAt).Milliseconds(),
		len(text),
	)

	return text, nil
}

func buildChatCompletionsURL(baseURL string) string {
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	return baseURL + "/chat/completions"
}

func shrinkBody(body string, max int) string {
	body = strings.TrimSpace(body)
	if max <= 0 || len(body) <= max {
		return body
	}
	return body[:max] + "...(truncated)"
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type simulatedTimeoutError struct{}

func (simulatedTimeoutError) Error() string   { return "simulated model timeout" }
func (simulatedTimeoutError) Timeout() bool   { return true }
func (simulatedTimeoutError) Temporary() bool { return true }

var _ net.Error = simulatedTimeoutError{}
