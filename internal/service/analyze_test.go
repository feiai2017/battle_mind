package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/output"
	"github.com/feiai2017/battle_mind/internal/rules"
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

func TestParseAnalyzeResult_ModelSuggestionBlock(t *testing.T) {
	raw := `{"summary":"DOT 体系存在断档风险。","suggestions":["检查循环"],"risks":["高压战斗下更明显"]}`

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if result.Summary != "DOT 体系存在断档风险。" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != "检查循环" {
		t.Fatalf("unexpected suggestions: %#v", result.Suggestions)
	}
	if len(result.Risks) != 1 || result.Risks[0] != "高压战斗下更明显" {
		t.Fatalf("unexpected risks: %#v", result.Risks)
	}
}

func TestParseAnalyzeResult_CodeFenceJSON(t *testing.T) {
	raw := "```json\n{\"summary\":\"ok\",\"suggestions\":[\"s1\"],\"risks\":[\"r1\"]}\n```"

	result, err := parseAnalyzeResult(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if result.Summary != "ok" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
}

func TestParseAnalyzeResult_LeadingAndTrailingText(t *testing.T) {
	raw := "这里是分析结果：\n{\"summary\":\"ok\",\"suggestions\":[\"s1\"],\"risks\":[\"r1\"]}\n后面还有解释文字"

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
	if result.Suggestions == nil {
		t.Fatalf("suggestions should be normalized to empty slice")
	}
	if result.Risks == nil {
		t.Fatalf("risks should be normalized to empty slice")
	}
}

func TestParseAnalyzeResult_EmptySummary(t *testing.T) {
	raw := `{"summary":"   ","suggestions":["s1"],"risks":["r1"]}`

	_, err := parseAnalyzeResult(raw)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !errors.Is(err, ErrInvalidAnalyzeResult) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildAnalyzePrompt_UsesRuleSummaryAndReportContext(t *testing.T) {
	ruleSummary := rules.RuleSummary{
		HardFindings: []rules.RuleFinding{
			{Code: rules.FindingLongCastIdleGap, Message: "存在较长技能施法空窗"},
		},
		SuspiciousSignals: []rules.RuleFinding{
			{Code: rules.SignalLowBurstFrequency, Message: "爆发技能施放次数偏少，值得关注"},
		},
	}

	prompt, err := buildAnalyzePrompt(model.AnalyzeInput{
		Request: model.AnalyzeRequest{
			LogText:    "battle log",
			BattleType: "boss_pve",
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
	}, ruleSummary)
	if err != nil {
		t.Fatalf("build prompt failed: %v", err)
	}
	if !strings.Contains(prompt, "[Hard Findings]") {
		t.Fatalf("prompt should contain hard findings: %s", prompt)
	}
	if !strings.Contains(prompt, "[Suspicious Signals]") {
		t.Fatalf("prompt should contain suspicious signals: %s", prompt)
	}
	if !strings.Contains(prompt, "[Battle Report Context]") {
		t.Fatalf("prompt should contain report context: %s", prompt)
	}
	if strings.Contains(prompt, "\"issues\"") {
		t.Fatalf("prompt should not ask model to generate issues: %s", prompt)
	}
}

func TestAnalyzeService_NoRepairWhenFirstParseSucceeds(t *testing.T) {
	generator := &stubGenerator{
		responses: []string{
			`{"summary":"DOT 体系存在断档风险。","suggestions":["检查循环"],"risks":["高压环境下问题会被放大"]}`,
		},
	}
	service := &AnalyzeService{client: generator}
	firstSkill := "毒蚀穿刺"
	secondSkill := "裂蚀绽放"

	report := model.BattleReport{
		Events: []model.ReportEvent{
			{Time: 7.0, Type: "SKILL_CAST", SourceName: &firstSkill},
			{Time: 25.7, Type: "SKILL_CAST", SourceName: &secondSkill},
		},
	}
	result, err := service.Analyze(context.Background(), model.AnalyzeInput{
		Request: model.AnalyzeRequest{LogText: "battle log"},
		Report:  &report,
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if generator.calls != 1 {
		t.Fatalf("expected one model call, got %d", generator.calls)
	}
	if len(result.RuleFindings.Findings) == 0 {
		t.Fatal("expected rule findings to be built by program")
	}
	if result.ModelSuggestions.Summary == "" {
		t.Fatal("expected model suggestions summary")
	}
}

func TestAnalyzeService_RepairSuccess(t *testing.T) {
	generator := &stubGenerator{
		responses: []string{
			`{"summary":"broken","suggestions":["s1"],"risks":["r1"]`,
			`{"summary":"fixed","suggestions":["s1"],"risks":["r1"]}`,
		},
	}
	service := &AnalyzeService{client: generator}

	result, err := service.Analyze(context.Background(), model.AnalyzeInput{
		Request: model.AnalyzeRequest{LogText: "battle log"},
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if generator.calls != 2 {
		t.Fatalf("expected two model calls, got %d", generator.calls)
	}
	if result.ModelSuggestions.Summary != "fixed" {
		t.Fatalf("unexpected summary: %s", result.ModelSuggestions.Summary)
	}
	if result.RawText != generator.responses[0] {
		t.Fatalf("raw_text should keep original model output")
	}
}

func TestAnalyzeService_RepairFailure(t *testing.T) {
	generator := &stubGenerator{
		responses: []string{
			`{"summary":"broken","suggestions":[`,
			`still not json`,
		},
	}
	service := &AnalyzeService{client: generator}

	_, err := service.Analyze(context.Background(), model.AnalyzeInput{
		Request: model.AnalyzeRequest{LogText: "battle log"},
	})
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

func TestAnalyzeService_EmptyRuleSummaryStillReturnsTwoParts(t *testing.T) {
	generator := &stubGenerator{
		responses: []string{
			`{"summary":"当前没有明确规则异常，建议继续观察。","suggestions":[],"risks":[]}`,
		},
	}
	service := &AnalyzeService{client: generator}

	result, err := service.Analyze(context.Background(), model.AnalyzeInput{
		Request: model.AnalyzeRequest{LogText: "battle log"},
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if result.RuleFindings.Findings == nil {
		t.Fatal("expected initialized rule findings")
	}
	if result.ModelSuggestions.Summary == "" {
		t.Fatal("expected model suggestion summary")
	}
}

func TestNormalizeModelSuggestionBlock_UsesOutputShape(t *testing.T) {
	result := output.BuildAnalyzeOutput(rules.RuleSummary{}, output.ModelSuggestionBlock{
		Summary:     " summary ",
		Suggestions: []string{" s1 ", ""},
		Risks:       []string{" r1 ", ""},
	})

	if result.ModelSuggestions.Summary != "summary" {
		t.Fatalf("unexpected summary: %q", result.ModelSuggestions.Summary)
	}
	if len(result.ModelSuggestions.Suggestions) != 1 || result.ModelSuggestions.Suggestions[0] != "s1" {
		t.Fatalf("unexpected suggestions: %#v", result.ModelSuggestions.Suggestions)
	}
	if len(result.ModelSuggestions.Risks) != 1 || result.ModelSuggestions.Risks[0] != "r1" {
		t.Fatalf("unexpected risks: %#v", result.ModelSuggestions.Risks)
	}
}
