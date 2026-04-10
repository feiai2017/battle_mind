package prompt

import (
	"strings"
	"testing"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/rules"
)

func TestBuildAnalyzePrompt_ContainsRuleSummarySections(t *testing.T) {
	prompt := BuildAnalyzePrompt(AnalyzePromptInput{
		Report:      &model.BattleReport{},
		RuleSummary: rules.RuleSummary{},
	})

	for _, section := range []string{"[Hard Findings]", "[Suspicious Signals]", "[Rule Metrics]"} {
		if !strings.Contains(prompt, section) {
			t.Fatalf("missing section %s in prompt: %s", section, prompt)
		}
	}
}

func TestBuildAnalyzePrompt_RuleSummaryBeforeReportContext(t *testing.T) {
	prompt := BuildAnalyzePrompt(AnalyzePromptInput{
		Report: &model.BattleReport{},
		RuleSummary: rules.RuleSummary{
			HardFindings: []rules.RuleFinding{{Code: rules.FindingLongCastIdleGap, Message: "存在较长技能施法空窗"}},
		},
	})

	ruleIndex := strings.Index(prompt, "[Hard Findings]")
	reportIndex := strings.Index(prompt, "[Battle Report Context]")
	if ruleIndex == -1 || reportIndex == -1 {
		t.Fatalf("missing expected sections: %s", prompt)
	}
	if ruleIndex >= reportIndex {
		t.Fatalf("rule summary should appear before report context")
	}
}

func TestBuildAnalyzePrompt_EmptyRuleSummaryStillStable(t *testing.T) {
	prompt := BuildAnalyzePrompt(AnalyzePromptInput{
		Report:      &model.BattleReport{},
		RuleSummary: rules.RuleSummary{},
	})

	if !strings.Contains(prompt, "[Hard Findings]\n- none") {
		t.Fatalf("expected stable empty hard findings section: %s", prompt)
	}
	if !strings.Contains(prompt, "[Suspicious Signals]\n- none") {
		t.Fatalf("expected stable empty suspicious signals section: %s", prompt)
	}
}

func TestBuildAnalyzePrompt_RendersFindingsAndEvidence(t *testing.T) {
	prompt := BuildAnalyzePrompt(AnalyzePromptInput{
		Report: &model.BattleReport{},
		RuleSummary: rules.RuleSummary{
			HardFindings: []rules.RuleFinding{
				{
					Code:     rules.FindingLongCastIdleGap,
					Message:  "存在较长技能施法空窗",
					Evidence: map[string]interface{}{"duration": 18.7, "start": 7.0, "end": 25.7},
				},
				{
					Code:     rules.FindingLowDotActivity,
					Message:  "DOT 相关事件偏少",
					Evidence: map[string]interface{}{"dot_event_count": 4},
				},
			},
		},
	})

	for _, want := range []string{
		rules.FindingLongCastIdleGap,
		"存在较长技能施法空窗",
		"duration: 18.7",
		rules.FindingLowDotActivity,
		"dot_event_count: 4",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %q in prompt: %s", want, prompt)
		}
	}
}

func TestBuildAnalyzePrompt_MetricsOrderStable(t *testing.T) {
	prompt := BuildAnalyzePrompt(AnalyzePromptInput{
		Report: &model.BattleReport{},
		RuleSummary: rules.RuleSummary{
			Metrics: rules.RuleSummaryMetric{
				SkillCasts:          map[string]int{"裂蚀绽放": 1, "毒蚀穿刺": 3, "传染波": 2},
				LastCastTimes:       map[string]float64{"裂蚀绽放": 16.8, "毒蚀穿刺": 46.6, "传染波": 43.7},
				DotEventCount:       53,
				DotEventCountByType: map[string]int{rules.EventTypeDotTick: 31, rules.EventTypeDotBurst: 5, rules.EventTypeDotApply: 17},
				MaxCastIdleGap:      &rules.IdleGap{Duration: 18.7, Start: 7.0, End: 25.7},
			},
		},
	})

	assertInOrder(t, prompt,
		"  - 传染波: 2",
		"  - 毒蚀穿刺: 3",
		"  - 裂蚀绽放: 1",
	)
	assertInOrder(t, prompt,
		"  - 传染波: 43.7",
		"  - 毒蚀穿刺: 46.6",
		"  - 裂蚀绽放: 16.8",
	)
	assertInOrder(t, prompt,
		"  - dot_apply: 17",
		"  - dot_tick: 31",
		"  - dot_burst: 5",
	)
}

func TestBuildAnalyzePrompt_ReportContextUsesKeyFieldsOnly(t *testing.T) {
	report := model.BattleReport{
		ResultSummary: model.ResultSummary{
			Win:          true,
			Duration:     78.3,
			LikelyReason: "胜利但血线过低",
		},
		AggregateMetrics: model.AggregateMetrics{
			SkillUsage:     []model.SkillUsage{{SkillID: "contagion_wave", Casts: 9}},
			DamageBySource: []model.DamageMetric{{Category: "dot", SourceID: "toxic_lance", Damage: 120.5}},
		},
		Diagnosis: []model.RawDiagnosis{{Code: "LOW_SURVIVAL", Severity: "warning", Message: "血线偏低"}},
		Events:    []model.ReportEvent{{Type: "SKILL_CAST"}},
	}

	prompt := BuildAnalyzePrompt(AnalyzePromptInput{Report: &report})

	for _, want := range []string{"[Battle Result]", "win: true", "duration: 78.3", "likely_reason: 胜利但血线过低", "contagion_wave: 9", "toxic_lance (dot): 120.5", "LOW_SURVIVAL (warning): 血线偏低"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %q in prompt: %s", want, prompt)
		}
	}
	if strings.Contains(prompt, "\"events\"") || strings.Contains(prompt, "SKILL_CAST") {
		t.Fatalf("prompt should not dump raw events: %s", prompt)
	}
}

func TestBuildAnalyzePrompt_ContainsBehaviorInstructions(t *testing.T) {
	prompt := BuildAnalyzePrompt(AnalyzePromptInput{Report: &model.BattleReport{}})

	for _, want := range []string{
		"系统已经直接输出 rule_findings，你不需要重复完整罗列规则明确发现",
		"请优先参考 RuleSummary 中的 hard_findings",
		"请把 suspicious_signals 仅作为值得关注的信号，语气必须保守",
		"你的任务只是在这些信息之上生成 model_suggestions",
		"最终只输出合法 JSON",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing instruction %q in prompt: %s", want, prompt)
		}
	}
}

func TestBuildAnalyzePrompt_TargetsModelSuggestionsOnly(t *testing.T) {
	prompt := BuildAnalyzePrompt(AnalyzePromptInput{Report: &model.BattleReport{}})

	if !strings.Contains(prompt, `{"summary":"一句话解释","suggestions":["建议1"],"risks":["风险1"]}`) {
		t.Fatalf("prompt should target model suggestion block only: %s", prompt)
	}
	if strings.Contains(prompt, `"issues"`) {
		t.Fatalf("prompt should not ask model to generate issues: %s", prompt)
	}
}

func assertInOrder(t *testing.T, text string, parts ...string) {
	t.Helper()
	lastIndex := -1
	for _, part := range parts {
		index := strings.Index(text, part)
		if index == -1 {
			t.Fatalf("missing %q in text: %s", part, text)
		}
		if index < lastIndex {
			t.Fatalf("expected %q after previous parts in text: %s", part, text)
		}
		lastIndex = index
	}
}
