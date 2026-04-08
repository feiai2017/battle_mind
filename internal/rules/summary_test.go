package rules

import (
	"encoding/json"
	"testing"
)

func TestBuildRuleSummary_EmptyInput(t *testing.T) {
	summary := BuildRuleSummary(nil)

	if summary.HardFindings == nil {
		t.Fatal("expected hard findings slice to be initialized")
	}
	if summary.SuspiciousSignals == nil {
		t.Fatal("expected suspicious signals slice to be initialized")
	}
	if len(summary.HardFindings) != 0 || len(summary.SuspiciousSignals) != 0 {
		t.Fatalf("expected no findings, got hard=%d suspicious=%d", len(summary.HardFindings), len(summary.SuspiciousSignals))
	}
	if summary.Metrics.SkillCasts == nil || len(summary.Metrics.SkillCasts) != 0 {
		t.Fatalf("unexpected skill casts: %+v", summary.Metrics.SkillCasts)
	}
	if summary.Metrics.LastCastTimes == nil || len(summary.Metrics.LastCastTimes) != 0 {
		t.Fatalf("unexpected last cast times: %+v", summary.Metrics.LastCastTimes)
	}
	if summary.Metrics.DotEventCount != 0 {
		t.Fatalf("unexpected dot event count: %d", summary.Metrics.DotEventCount)
	}
	if summary.Metrics.DotEventCountByType == nil {
		t.Fatal("expected dot event count by type to be initialized")
	}
	if summary.Metrics.MaxCastIdleGap != nil {
		t.Fatalf("expected nil max cast idle gap, got: %+v", summary.Metrics.MaxCastIdleGap)
	}
}

func TestBuildRuleSummary_OnlyCastNoDot(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 4.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 6.0, EventType: EventTypeCast, SkillName: "传染波"},
	}

	summary := BuildRuleSummary(events)

	if summary.Metrics.DotEventCount != 0 {
		t.Fatalf("unexpected dot event count: %d", summary.Metrics.DotEventCount)
	}
	assertHasFindingCode(t, summary.HardFindings, FindingLowDotActivity)
	assertHasFindingCode(t, summary.HardFindings, FindingNoBurstEvent)
}

func TestBuildRuleSummary_LongCastIdleGap(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 20.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	summary := BuildRuleSummary(events)

	if summary.Metrics.MaxCastIdleGap == nil {
		t.Fatal("expected max cast idle gap")
	}
	if summary.Metrics.MaxCastIdleGap.Duration != 19.0 {
		t.Fatalf("unexpected max cast idle gap: %+v", summary.Metrics.MaxCastIdleGap)
	}

	finding := mustFindByCode(t, summary.HardFindings, FindingLongCastIdleGap)
	if finding.Evidence["duration"] != 19.0 {
		t.Fatalf("unexpected duration evidence: %+v", finding.Evidence)
	}
	if finding.Evidence["start"] != 1.0 || finding.Evidence["end"] != 20.0 {
		t.Fatalf("unexpected gap evidence: %+v", finding.Evidence)
	}
}

func TestBuildRuleSummary_DotActiveWithBurst(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotApply, SkillName: "传染波"},
		{EventType: EventTypeDotApply, SkillName: "传染波"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotBurst, SkillName: "裂蚀绽放"},
	}

	summary := BuildRuleSummary(events)

	if summary.Metrics.DotEventCount != 6 {
		t.Fatalf("unexpected dot event count: %d", summary.Metrics.DotEventCount)
	}
	if summary.Metrics.DotEventCountByType[EventTypeDotApply] != 2 ||
		summary.Metrics.DotEventCountByType[EventTypeDotTick] != 3 ||
		summary.Metrics.DotEventCountByType[EventTypeDotBurst] != 1 {
		t.Fatalf("unexpected dot event count by type: %+v", summary.Metrics.DotEventCountByType)
	}
	assertNoFindingCode(t, summary.HardFindings, FindingNoBurstEvent)
}

func TestBuildRuleSummary_LowBurstFrequencySignal(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 3.0, EventType: EventTypeCast, SkillName: "传染波"},
		{Timestamp: 5.0, EventType: EventTypeCast, SkillName: "裂蚀绽放"},
	}

	summary := BuildRuleSummary(events)

	signal := mustFindByCode(t, summary.SuspiciousSignals, SignalLowBurstFrequency)
	if signal.Evidence["skill_name"] != burstSkillName {
		t.Fatalf("unexpected skill name evidence: %+v", signal.Evidence)
	}
	if signal.Evidence["casts"] != 1 {
		t.Fatalf("unexpected casts evidence: %+v", signal.Evidence)
	}
}

func TestBuildRuleSummary_JSONShapeStable(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 20.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
	}

	summary := BuildRuleSummary(events)
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal summary failed: %v", err)
	}

	if _, ok := decoded["hard_findings"]; !ok {
		t.Fatal("expected hard_findings field")
	}
	if _, ok := decoded["suspicious_signals"]; !ok {
		t.Fatal("expected suspicious_signals field")
	}
	if _, ok := decoded["metrics"]; !ok {
		t.Fatal("expected metrics field")
	}
}

func assertHasFindingCode(t *testing.T, findings []RuleFinding, code string) {
	t.Helper()
	_ = mustFindByCode(t, findings, code)
}

func assertNoFindingCode(t *testing.T, findings []RuleFinding, code string) {
	t.Helper()
	for _, finding := range findings {
		if finding.Code == code {
			t.Fatalf("did not expect finding code %s", code)
		}
	}
}

func mustFindByCode(t *testing.T, findings []RuleFinding, code string) RuleFinding {
	t.Helper()
	for _, finding := range findings {
		if finding.Code == code {
			return finding
		}
	}
	t.Fatalf("expected finding code %s", code)
	return RuleFinding{}
}
