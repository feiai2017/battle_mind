package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/feiai2017/battle_mind/internal/model"
)

type stubGenerator struct {
	responses []string
	errs      []error
	calls     int
}

func (s *stubGenerator) Generate(_ context.Context, _ string) (string, error) {
	index := s.calls
	s.calls++

	if index < len(s.errs) && s.errs[index] != nil {
		return "", s.errs[index]
	}
	if index < len(s.responses) {
		return s.responses[index], nil
	}
	return "", nil
}

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

func TestParseAnalyzeResult_LeadingAndTrailingText(t *testing.T) {
	raw := "这里是分析结果：\n{\"summary\":\"ok\",\"issues\":[{\"title\":\"t1\",\"description\":\"d1\",\"severity\":\"low\",\"evidence\":[\"e1\"]}],\"suggestions\":[\"s1\"]}\n后面还有解释文字"

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

func TestBuildAnalyzePrompt_UsesStructuredAnalyzeRequest(t *testing.T) {
	prompt, err := buildAnalyzePrompt(model.AnalyzeInput{
		Request: model.AnalyzeRequest{
			Metadata: model.AnalyzeMetadata{
				BattleType: "boss_pve",
				BuildTags:  []string{"dot", "burst"},
			},
			Summary: model.BattleSummary{
				Win:          true,
				Duration:     78,
				LikelyReason: "rotation is slow",
			},
			Metrics: model.BattleMetrics{
				SkillUsage: map[string]int{"contagion_wave": 9},
			},
			Diagnosis: []model.DiagnosisInput{
				{Code: "LOW_SURVIVAL", Severity: "warn", Message: "hp too low"},
			},
		},
		Report: &model.BattleReport{
			ResultSummary: model.ResultSummary{
				Win:          true,
				Duration:     78.3,
				LikelyReason: "rotation is slow",
			},
			AggregateMetrics: model.AggregateMetrics{
				SkillUsage: []model.SkillUsage{{SkillID: "contagion_wave", Casts: 9}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build prompt failed: %v", err)
	}
	if !strings.Contains(prompt, "[Rule Metrics]") {
		t.Fatalf("prompt should contain rule metrics: %s", prompt)
	}
	if !strings.Contains(prompt, "[Battle Report Context]") {
		t.Fatalf("prompt should contain report context: %s", prompt)
	}
	if strings.Contains(prompt, "\"events\"") {
		t.Fatalf("prompt should not dump raw events: %s", prompt)
	}
}

func TestAnalyzeService_NoRepairWhenFirstParseSucceeds(t *testing.T) {
	generator := &stubGenerator{
		responses: []string{
			`{"summary":"ok","issues":[{"title":"t1","description":"d1","severity":"low","evidence":["e1"]}],"suggestions":["s1"]}`,
		},
	}
	service := &AnalyzeService{client: generator}

	result, err := service.Analyze(context.Background(), model.AnalyzeInput{Request: model.AnalyzeRequest{LogText: "battle log"}})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if generator.calls != 1 {
		t.Fatalf("expected one model call, got %d", generator.calls)
	}
	if result.Summary != "ok" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
}

func TestAnalyzeService_RepairSuccess(t *testing.T) {
	generator := &stubGenerator{
		responses: []string{
			`{"summary":"broken","issues":[{"title":"t1","description":"d1","severity":"low","evidence":["e1"]}],"suggestions":["s1"]`,
			`{"summary":"fixed","issues":[{"title":"t1","description":"d1","severity":"low","evidence":["e1"]}],"suggestions":["s1"]}`,
		},
	}
	service := &AnalyzeService{client: generator}

	result, err := service.Analyze(context.Background(), model.AnalyzeInput{Request: model.AnalyzeRequest{LogText: "battle log"}})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if generator.calls != 2 {
		t.Fatalf("expected two model calls, got %d", generator.calls)
	}
	if result.Summary != "fixed" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.RawText != generator.responses[0] {
		t.Fatalf("raw_text should keep original model output")
	}
}

func TestAnalyzeService_RepairFailure(t *testing.T) {
	generator := &stubGenerator{
		responses: []string{
			`{"summary":"broken","issues":[`,
			`still not json`,
		},
	}
	service := &AnalyzeService{client: generator}

	_, err := service.Analyze(context.Background(), model.AnalyzeInput{Request: model.AnalyzeRequest{LogText: "battle log"}})
	if err == nil {
		t.Fatalf("expected analyze error")
	}
	if !errors.Is(err, ErrModelJSONRepairFailed) {
		t.Fatalf("unexpected error: %v", err)
	}
	if generator.calls != 2 {
		t.Fatalf("repair should run exactly once, got %d calls", generator.calls)
	}
}
