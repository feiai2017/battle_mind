package rules

import "testing"

func TestHasLongDotGap_UsesOnlyDotEvents(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 2.0, EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{Timestamp: 5.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 6.0, EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺"},
		{Timestamp: 10.0, EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
	}

	if !HasLongDotGap(events, 5.0) {
		t.Fatal("expected long dot gap")
	}

	maxGap, ok := MaxDotIdleGap(events)
	if !ok {
		t.Fatal("expected max dot gap")
	}
	if maxGap.Duration != 8.0 {
		t.Fatalf("unexpected max dot gap: %+v", maxGap)
	}
}

func TestHasLongDotGap_ContinuousDotActivity(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeDotApply, SkillName: "传染波"},
		{Timestamp: 2.0, EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{Timestamp: 3.5, EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{Timestamp: 4.2, EventType: EventTypeDotBurst, SkillName: "裂蚀绽放"},
	}

	if HasLongDotGap(events, 5.0) {
		t.Fatal("did not expect long dot gap")
	}
}

func TestBuildDotIdleGaps_InsufficientDotEvents(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeDotApply, SkillName: "传染波"},
		{Timestamp: 2.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	gaps := BuildDotIdleGaps(events)
	if gaps != nil {
		t.Fatalf("expected nil gaps, got: %+v", gaps)
	}
	if HasLongDotGap(events, 5.0) {
		t.Fatal("did not expect long dot gap with fewer than two dot events")
	}
}
