package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/service"
)

type stubAnalyzeService struct {
	called    int
	result    model.AnalyzeResult
	err       error
	last      model.AnalyzeRequest
	modelName string
}

func (s *stubAnalyzeService) Analyze(_ context.Context, req model.AnalyzeRequest) (model.AnalyzeResult, error) {
	s.called++
	s.last = req
	return s.result, s.err
}

func (s *stubAnalyzeService) ModelName() string {
	if s.modelName == "" {
		return "stub-model"
	}
	return s.modelName
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "request timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	return &buf, func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	}
}

func TestAnalyze_InvalidJSONReturnsUnifiedError(t *testing.T) {
	svc := &stubAnalyzeService{}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp.Error.Code != model.ErrCodeInvalidJSON {
		t.Fatalf("unexpected code: %s", resp.Error.Code)
	}
	if svc.called != 0 {
		t.Fatalf("service should not be called")
	}
}

func TestAnalyze_InvalidRequestDoesNotCallService(t *testing.T) {
	svc := &stubAnalyzeService{}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"   \n\t "}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp.Error.Code != model.ErrCodeEmptyLogText {
		t.Fatalf("unexpected code: %s", resp.Error.Code)
	}
	if svc.called != 0 {
		t.Fatalf("service should not be called")
	}
}

func TestAnalyze_ValidRequestCallsService(t *testing.T) {
	svc := &stubAnalyzeService{
		result: model.AnalyzeResult{
			Summary: "ok",
		},
	}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{
		"log_text":"  test log  ",
		"battle_type":"boss_pve",
		"build_tags":["dot","burst"],
		"notes":"memo"
	}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if svc.called != 1 {
		t.Fatalf("service should be called once, got %d", svc.called)
	}
	if svc.last.LogText != "test log" {
		t.Fatalf("expected log_text to be normalized, got %q", svc.last.LogText)
	}
}

func TestAnalyze_StructuredRequestWithoutLogTextCallsService(t *testing.T) {
	svc := &stubAnalyzeService{
		result: model.AnalyzeResult{
			Summary: "ok",
		},
	}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{
		"metadata":{
			"battle_type":"boss_pve",
			"build_tags":["dot","burst"],
			"notes":"converted"
		},
		"summary":{
			"win":true,
			"duration":78,
			"likely_reason":"rotation is slow"
		},
		"metrics":{
			"damage_by_source":{
				"dot":120.5,
				"direct":80
			},
			"skill_usage":{
				"contagion_wave":9
			}
		},
		"diagnosis":[
			{
				"code":"LOW_SURVIVAL",
				"severity":"warn",
				"message":"hp too low"
			}
		]
	}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if svc.called != 1 {
		t.Fatalf("service should be called once, got %d", svc.called)
	}
	if svc.last.BattleType != "boss_pve" {
		t.Fatalf("expected battle_type to be lifted from metadata, got %q", svc.last.BattleType)
	}
	if len(svc.last.BuildTags) != 2 || svc.last.BuildTags[0] != "dot" {
		t.Fatalf("unexpected build_tags: %#v", svc.last.BuildTags)
	}
}

func TestAnalyze_ModelJSONRepairFailedReturnsUnifiedError(t *testing.T) {
	svc := &stubAnalyzeService{err: service.ErrModelJSONRepairFailed}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"test log"}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp.Error.Code != model.ErrCodeInvalidModelJSON {
		t.Fatalf("unexpected code: %s", resp.Error.Code)
	}
	if resp.Error.Message != "model output is not valid JSON and could not be repaired" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestAnalyze_SuccessLogsRequestSummaryAndHeader(t *testing.T) {
	logBuffer, restore := captureLogs(t)
	defer restore()

	svc := &stubAnalyzeService{
		modelName: "deepseek-chat",
		result: model.AnalyzeResult{
			Summary: "ok",
		},
	}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"test log","battle_type":"boss_pve"}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	requestID := rec.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatalf("expected %s header", requestIDHeader)
	}

	logText := logBuffer.String()
	if !strings.Contains(logText, "component=analyze_request_log event=request_completed") {
		t.Fatalf("missing request summary log: %s", logText)
	}
	if !strings.Contains(logText, "request_id="+requestID) {
		t.Fatalf("missing request_id in logs: %s", logText)
	}
	if !strings.Contains(logText, `model_name="deepseek-chat"`) {
		t.Fatalf("missing model_name in logs: %s", logText)
	}
	if !strings.Contains(logText, `error_reason="NONE"`) {
		t.Fatalf("missing success error_reason in logs: %s", logText)
	}
	if !strings.Contains(logText, "duration_ms=") {
		t.Fatalf("missing duration_ms in logs: %s", logText)
	}
}

func TestAnalyze_EmptyLogRequestLogsReason(t *testing.T) {
	logBuffer, restore := captureLogs(t)
	defer restore()

	svc := &stubAnalyzeService{modelName: "deepseek-chat"}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"   "}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if rec.Header().Get(requestIDHeader) == "" {
		t.Fatalf("expected %s header", requestIDHeader)
	}
	if !strings.Contains(logBuffer.String(), `error_reason="EMPTY_LOG_TEXT"`) {
		t.Fatalf("missing EMPTY_LOG_TEXT in logs: %s", logBuffer.String())
	}
}

func TestAnalyze_LogTooLongRequestLogsReason(t *testing.T) {
	logBuffer, restore := captureLogs(t)
	defer restore()

	svc := &stubAnalyzeService{modelName: "deepseek-chat"}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"`+strings.Repeat("x", model.MaxLogTextLength+1)+`"}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !strings.Contains(logBuffer.String(), `error_reason="LOG_TOO_LONG"`) {
		t.Fatalf("missing LOG_TOO_LONG in logs: %s", logBuffer.String())
	}
}

func TestAnalyze_ModelTimeoutLogsReason(t *testing.T) {
	logBuffer, restore := captureLogs(t)
	defer restore()

	svc := &stubAnalyzeService{
		modelName: "deepseek-chat",
		err:       timeoutError{},
	}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"test log"}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !strings.Contains(logBuffer.String(), `error_reason="MODEL_TIMEOUT"`) {
		t.Fatalf("missing MODEL_TIMEOUT in logs: %s", logBuffer.String())
	}
}

func TestAnalyze_InvalidModelJSONLogsReason(t *testing.T) {
	logBuffer, restore := captureLogs(t)
	defer restore()

	svc := &stubAnalyzeService{
		modelName: "deepseek-chat",
		err:       service.ErrModelJSONRepairFailed,
	}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"test log"}`))
	rec := httptest.NewRecorder()

	h.Analyze(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !strings.Contains(logBuffer.String(), `error_reason="RESULT_JSON_REPAIR_FAILED"`) {
		t.Fatalf("missing RESULT_JSON_REPAIR_FAILED in logs: %s", logBuffer.String())
	}
}

func TestMapAnalyzeError_Timeout(t *testing.T) {
	status, appErr, reason := mapAnalyzeError(timeoutError{})
	if status != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", status)
	}
	if appErr.Code != model.ErrCodeAnalyzeFailed {
		t.Fatalf("unexpected code: %s", appErr.Code)
	}
	if reason != logErrorReasonTimeout {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestMapAnalyzeError_NetTimeoutWrapped(t *testing.T) {
	err := errors.New("outer: " + timeoutError{}.Error())
	var netErr net.Error = timeoutError{}
	if !netErr.Timeout() {
		t.Fatal("expected timeout")
	}

	status, _, reason := mapAnalyzeError(timeoutWrappedError{inner: err})
	if status != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", status)
	}
	if reason != logErrorReasonTimeout {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

type timeoutWrappedError struct {
	inner error
}

func (e timeoutWrappedError) Error() string   { return e.inner.Error() }
func (e timeoutWrappedError) Timeout() bool   { return true }
func (e timeoutWrappedError) Temporary() bool { return true }

func TestAnalyze_RequestCompletedLogContainsDuration(t *testing.T) {
	logBuffer, restore := captureLogs(t)
	defer restore()

	svc := &stubAnalyzeService{
		modelName: "deepseek-chat",
		result:    model.AnalyzeResult{Summary: "ok"},
	}
	h := New(svc)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"log_text":"test log"}`))
	rec := httptest.NewRecorder()

	start := time.Now()
	h.Analyze(rec, req)
	if time.Since(start) < 0 {
		t.Fatal("unexpected clock")
	}
	if !strings.Contains(logBuffer.String(), "duration_ms=") {
		t.Fatalf("missing duration_ms in logs: %s", logBuffer.String())
	}
}
