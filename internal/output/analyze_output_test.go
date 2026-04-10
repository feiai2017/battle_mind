package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/feiai2017/battle_mind/internal/rules"
)

func TestBuildRuleFindingsOutput_UsesHardFindingsAndMetrics(t *testing.T) {
	summary := rules.RuleSummary{
		HardFindings: []rules.RuleFinding{
			{
				Code:     rules.FindingLongCastIdleGap,
				Severity: rules.SeverityWarning,
				Message:  "存在较长技能施法空窗",
				Evidence: map[string]interface{}{"duration": 18.7},
			},
		},
		Metrics: rules.RuleSummaryMetric{
			SkillCasts: map[string]int{"毒蚀穿刺": 17},
		},
	}

	result := BuildRuleFindingsOutput(summary)

	if len(result.Findings) != 1 {
		t.Fatalf("unexpected findings length: %d", len(result.Findings))
	}
	if result.Findings[0].Code != rules.FindingLongCastIdleGap {
		t.Fatalf("unexpected finding code: %s", result.Findings[0].Code)
	}
	if result.Metrics.SkillCasts["毒蚀穿刺"] != 17 {
		t.Fatalf("unexpected metrics: %#v", result.Metrics.SkillCasts)
	}
}

func TestBuildRuleFindingsOutput_PreservesHardFindingsOrder(t *testing.T) {
	summary := rules.RuleSummary{
		HardFindings: []rules.RuleFinding{
			{Code: rules.FindingLongCastIdleGap},
			{Code: rules.FindingLowDotActivity},
		},
	}

	result := BuildRuleFindingsOutput(summary)

	if len(result.Findings) != 2 {
		t.Fatalf("unexpected findings length: %d", len(result.Findings))
	}
	if result.Findings[0].Code != rules.FindingLongCastIdleGap || result.Findings[1].Code != rules.FindingLowDotActivity {
		t.Fatalf("unexpected findings order: %#v", result.Findings)
	}
}

func TestBuildRuleFindingsOutput_DoesNotMixSuspiciousSignals(t *testing.T) {
	summary := rules.RuleSummary{
		HardFindings: []rules.RuleFinding{
			{Code: rules.FindingNoBurstEvent},
		},
		SuspiciousSignals: []rules.RuleFinding{
			{Code: rules.SignalLowBurstFrequency},
		},
	}

	result := BuildRuleFindingsOutput(summary)

	if len(result.Findings) != 1 {
		t.Fatalf("unexpected findings length: %d", len(result.Findings))
	}
	if result.Findings[0].Code != rules.FindingNoBurstEvent {
		t.Fatalf("unexpected finding code: %s", result.Findings[0].Code)
	}
}

func TestBuildAnalyzeOutput_EmptyRuleSummaryStillStable(t *testing.T) {
	result := BuildAnalyzeOutput(rules.RuleSummary{}, ModelSuggestionBlock{
		Summary: "当前未发现明确规则异常，建议结合上下文继续观察。",
	})

	if result.RuleFindings.Findings == nil {
		t.Fatal("expected findings to be initialized")
	}
	if result.ModelSuggestions.Summary == "" {
		t.Fatal("expected summary to be preserved")
	}
}

func TestAnalyzeOutput_JSONShapeStable(t *testing.T) {
	result := BuildAnalyzeOutput(rules.RuleSummary{
		HardFindings: []rules.RuleFinding{
			{Code: rules.FindingLongCastIdleGap},
		},
	}, ModelSuggestionBlock{
		Summary:     "存在循环断档风险。",
		Suggestions: []string{"优先检查爆发时机。"},
		Risks:       []string{"高压战斗中可能放大问题。"},
	})

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	text := string(encoded)
	for _, key := range []string{`"rule_findings"`, `"model_suggestions"`, `"findings"`, `"metrics"`, `"summary"`, `"suggestions"`, `"risks"`} {
		if !strings.Contains(text, key) {
			t.Fatalf("missing key %s in %s", key, text)
		}
	}
	if strings.Index(text, `"rule_findings"`) > strings.Index(text, `"model_suggestions"`) {
		t.Fatalf("expected rule_findings before model_suggestions: %s", text)
	}
	if strings.Index(text, `"findings"`) > strings.Index(text, `"metrics"`) {
		t.Fatalf("expected findings before metrics: %s", text)
	}
}
