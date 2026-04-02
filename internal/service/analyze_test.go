package service

import (
	"errors"
	"testing"
)

func TestParseAnalyzeResult_ValidJSON(t *testing.T) {
	raw := `{"summary":"rotation breaks in late phase","problems":["skill gap"],"suggestions":["check cooldown flow"]}`

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if result.Summary != "rotation breaks in late phase" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(result.Problems) != 1 || result.Problems[0] != "skill gap" {
		t.Fatalf("unexpected problems: %#v", result.Problems)
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != "check cooldown flow" {
		t.Fatalf("unexpected suggestions: %#v", result.Suggestions)
	}
}

func TestParseAnalyzeResult_CodeFenceJSON(t *testing.T) {
	raw := "```json\n{\"summary\":\"ok\",\"problems\":[\"p1\"],\"suggestions\":[\"s1\"]}\n```"

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if result.Summary != "ok" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
}

func TestParseAnalyzeResult_InvalidJSON(t *testing.T) {
	raw := "not a json output"

	_, err := parseAnalyzeResult(raw)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !errors.Is(err, ErrInvalidLLMJSON) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAnalyzeResult_NormalizeMissingArrays(t *testing.T) {
	raw := `{"summary":"only summary"}`

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if result.Problems == nil {
		t.Fatalf("problems should be normalized to empty slice")
	}
	if result.Suggestions == nil {
		t.Fatalf("suggestions should be normalized to empty slice")
	}
}

func TestParseAnalyzeResult_EmptySummary(t *testing.T) {
	raw := `{"summary":"   ","problems":["p1"],"suggestions":["s1"]}`

	_, err := parseAnalyzeResult(raw)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !errors.Is(err, ErrInvalidAnalyzeResult) {
		t.Fatalf("unexpected error: %v", err)
	}
}
