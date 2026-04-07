package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/service"
)

type stubAnalyzeService struct {
	called int
	result model.AnalyzeResult
	err    error
	last   model.AnalyzeRequest
}

func (s *stubAnalyzeService) Analyze(_ context.Context, req model.AnalyzeRequest) (model.AnalyzeResult, error) {
	s.called++
	s.last = req
	return s.result, s.err
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
