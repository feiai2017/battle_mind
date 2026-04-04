package model

import (
	"strings"
	"testing"
)

func TestAnalyzeRequestValidation_EmptyLogText(t *testing.T) {
	req := AnalyzeRequest{
		LogText: "",
	}

	err := req.NormalizeAndValidate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if err.Code != ErrCodeEmptyLogText {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestAnalyzeRequestValidation_WhitespaceLogText(t *testing.T) {
	req := AnalyzeRequest{
		LogText: "   \n\t ",
	}

	err := req.NormalizeAndValidate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if err.Code != ErrCodeEmptyLogText {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestAnalyzeRequestValidation_LogTextTooLong(t *testing.T) {
	req := AnalyzeRequest{
		LogText: strings.Repeat("x", MaxLogTextLength+1),
	}

	err := req.NormalizeAndValidate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if err.Code != ErrCodeLogTooLong {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestAnalyzeRequestValidation_InvalidBattleType(t *testing.T) {
	req := AnalyzeRequest{
		LogText:    "some log",
		BattleType: "xxx",
	}

	err := req.NormalizeAndValidate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if err.Code != ErrCodeInvalidBattleType {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestAnalyzeRequestValidation_BuildTagsContainsEmpty(t *testing.T) {
	req := AnalyzeRequest{
		LogText:   "some log",
		BuildTags: []string{"dot", "   "},
	}

	err := req.NormalizeAndValidate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if err.Code != ErrCodeInvalidBuildTags {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestAnalyzeRequestValidation_BuildTagsTooMany(t *testing.T) {
	req := AnalyzeRequest{
		LogText: "some log",
	}
	for i := 0; i < MaxBuildTagsCount+1; i++ {
		req.BuildTags = append(req.BuildTags, "tag")
	}

	err := req.NormalizeAndValidate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if err.Code != ErrCodeInvalidBuildTags {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestAnalyzeRequestValidation_NotesTooLong(t *testing.T) {
	req := AnalyzeRequest{
		LogText: "some log",
		Notes:   strings.Repeat("n", MaxNotesLength+1),
	}

	err := req.NormalizeAndValidate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if err.Code != ErrCodeNotesTooLong {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestAnalyzeRequestValidation_ValidRequest(t *testing.T) {
	req := AnalyzeRequest{
		LogText:    "  battle log  ",
		BattleType: "boss_pve",
		BuildTags:  []string{" dot ", "burst"},
		Notes:      "  keep this  ",
	}

	err := req.NormalizeAndValidate()
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if req.LogText != "battle log" {
		t.Fatalf("unexpected normalized log_text: %q", req.LogText)
	}
	if len(req.BuildTags) != 2 || req.BuildTags[0] != "dot" || req.BuildTags[1] != "burst" {
		t.Fatalf("unexpected normalized build_tags: %#v", req.BuildTags)
	}
	if req.Notes != "keep this" {
		t.Fatalf("unexpected normalized notes: %q", req.Notes)
	}
}
