package service

import (
	"errors"
	"testing"
)

func TestParseAnalyzeResult_LegacyProblemsConverted(t *testing.T) {
	raw := `{
		"summary":"战斗胜利但血线过低，循环效率不足。",
		"problems":["血线过低","战斗时间过长"],
		"suggestions":["提高生存","优化循环"]
	}`

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if result.Summary != "战斗胜利但血线过低，循环效率不足。" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(result.Issues) != 2 {
		t.Fatalf("unexpected issues length: %d", len(result.Issues))
	}
	for i, issue := range result.Issues {
		if issue.Title == "" || issue.Description == "" || issue.Severity == "" {
			t.Fatalf("issue[%d] missing required fields: %#v", i, issue)
		}
		if len(issue.Evidence) == 0 {
			t.Fatalf("issue[%d] evidence should not be empty", i)
		}
	}
	if len(result.Suggestions) != 2 || result.Suggestions[0] != "提高生存" {
		t.Fatalf("unexpected suggestions: %#v", result.Suggestions)
	}
}

func TestParseAnalyzeResult_NewIssuesPassthrough(t *testing.T) {
	raw := `{
		"summary":"循环中断导致输出下降",
		"issues":[
			{
				"title":"血线过低",
				"description":"后半段血线过低，风险高。",
				"severity":"high",
				"evidence":["HP低于阈值"]
			}
		],
		"suggestions":["补生存资源"]
	}`

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("unexpected issues length: %d", len(result.Issues))
	}
	issue := result.Issues[0]
	if issue.Title != "血线过低" {
		t.Fatalf("unexpected issue title: %s", issue.Title)
	}
	if issue.Severity != "high" {
		t.Fatalf("unexpected issue severity: %s", issue.Severity)
	}
	if len(issue.Evidence) != 1 || issue.Evidence[0] != "HP低于阈值" {
		t.Fatalf("unexpected issue evidence: %#v", issue.Evidence)
	}
}

func TestParseAnalyzeResult_SeverityFallback(t *testing.T) {
	raw := `{
		"summary":"summary ok",
		"issues":[
			{"title":"A","description":"desc A","severity":"","evidence":["e1"]},
			{"title":"B","description":"desc B","severity":"critical","evidence":["e2"]}
		],
		"suggestions":["s1"]
	}`

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if result.Issues[0].Severity != "medium" {
		t.Fatalf("unexpected severity for issue[0]: %s", result.Issues[0].Severity)
	}
	if result.Issues[1].Severity != "medium" {
		t.Fatalf("unexpected severity for issue[1]: %s", result.Issues[1].Severity)
	}
}

func TestParseAnalyzeResult_EvidenceFallback(t *testing.T) {
	raw := `{
		"summary":"summary for fallback",
		"issues":[
			{"title":"A","description":"desc A","severity":"low"},
			{"title":"B","description":"desc B","severity":"medium","evidence":[]},
			{"title":"C","description":"desc C","severity":"high","evidence":null}
		],
		"suggestions":["s1"]
	}`

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	for i, issue := range result.Issues {
		if len(issue.Evidence) == 0 {
			t.Fatalf("issue[%d] evidence should be filled", i)
		}
	}
}

func TestParseAnalyzeResult_CodeFenceJSON(t *testing.T) {
	raw := "```json\n{\"summary\":\"ok\",\"issues\":[{\"title\":\"t1\",\"description\":\"d1\",\"severity\":\"low\",\"evidence\":[\"e1\"]}],\"suggestions\":[\"s1\"]}\n```"

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
	if result.Issues == nil {
		t.Fatalf("issues should be normalized to empty slice")
	}
	if result.Suggestions == nil {
		t.Fatalf("suggestions should be normalized to empty slice")
	}
}

func TestParseAnalyzeResult_EmptySummary(t *testing.T) {
	raw := `{"summary":"   ","issues":[{"title":"t","description":"d","severity":"low","evidence":["e"]}],"suggestions":["s1"]}`

	_, err := parseAnalyzeResult(raw)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !errors.Is(err, ErrInvalidAnalyzeResult) {
		t.Fatalf("unexpected error: %v", err)
	}
}
