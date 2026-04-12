package rules

import "testing"

func TestBuildCastIdleGaps_NoCastEvents(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeDotTick},
		{Timestamp: 2.0, EventType: EventTypeResourceGain},
	}

	gaps := BuildCastIdleGaps(events)
	if gaps != nil {
		t.Fatalf("expected nil gaps, got: %+v", gaps)
	}

	maxGap, ok := MaxCastIdleGap(events)
	if ok {
		t.Fatalf("expected no max gap, got: %+v", maxGap)
	}

	summary := SummarizeMaxCastIdleGap(events)
	if summary != "" {
		t.Fatalf("expected empty summary, got: %q", summary)
	}
}

func TestBuildCastIdleGaps_OnlyOneCast(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	gaps := BuildCastIdleGaps(events)
	if gaps != nil {
		t.Fatalf("expected nil gaps, got: %+v", gaps)
	}

	if _, ok := MaxCastIdleGap(events); ok {
		t.Fatal("expected no max gap")
	}
}

func TestBuildCastIdleGaps_TwoCasts(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast},
		{Timestamp: 5.5, EventType: EventTypeCast},
	}

	gaps := BuildCastIdleGaps(events)
	if len(gaps) != 1 {
		t.Fatalf("unexpected gap count: %d", len(gaps))
	}
	if gaps[0].Duration != 4.5 {
		t.Fatalf("unexpected gap duration: %v", gaps[0].Duration)
	}

	maxGap, ok := MaxCastIdleGap(events)
	if !ok {
		t.Fatal("expected max gap")
	}
	if maxGap.Duration != 4.5 {
		t.Fatalf("unexpected max gap duration: %v", maxGap.Duration)
	}
}

func TestBuildCastIdleGaps_MultipleAscendingCasts(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast},
		{Timestamp: 4.0, EventType: EventTypeCast},
		{Timestamp: 9.5, EventType: EventTypeCast},
		{Timestamp: 12.0, EventType: EventTypeCast},
	}

	gaps := BuildCastIdleGaps(events)
	if len(gaps) != 3 {
		t.Fatalf("unexpected gap count: %d", len(gaps))
	}
	if gaps[0].Duration != 3.0 || gaps[1].Duration != 5.5 || gaps[2].Duration != 2.5 {
		t.Fatalf("unexpected gaps: %+v", gaps)
	}

	maxGap, ok := MaxCastIdleGap(events)
	if !ok {
		t.Fatal("expected max gap")
	}
	if maxGap.Duration != 5.5 {
		t.Fatalf("unexpected max gap duration: %v", maxGap.Duration)
	}
}

func TestBuildCastIdleGaps_UnorderedInputStillWorks(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 9.5, EventType: EventTypeCast},
		{Timestamp: 1.0, EventType: EventTypeCast},
		{Timestamp: 12.0, EventType: EventTypeCast},
		{Timestamp: 4.0, EventType: EventTypeCast},
	}

	gaps := BuildCastIdleGaps(events)
	if len(gaps) != 3 {
		t.Fatalf("unexpected gap count: %d", len(gaps))
	}
	if gaps[0].Duration != 3.0 || gaps[1].Duration != 5.5 || gaps[2].Duration != 2.5 {
		t.Fatalf("unexpected gaps: %+v", gaps)
	}
}

func TestBuildCastIdleGaps_NonCastEventsDoNotFillGap(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast},
		{Timestamp: 6.0, EventType: EventTypeDotTick},
		{Timestamp: 7.0, EventType: EventTypeSkillHit},
		{Timestamp: 10.0, EventType: EventTypeCast},
	}

	gaps := BuildCastIdleGaps(events)
	if len(gaps) != 1 {
		t.Fatalf("unexpected gap count: %d", len(gaps))
	}
	if gaps[0].Duration != 9.0 {
		t.Fatalf("unexpected gap duration: %v", gaps[0].Duration)
	}
}

func TestBuildCastIdleGaps_IgnoresNonCastAndSortsInput(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 9.5, EventType: EventTypeCast, SkillName: "传染波"},
		{Timestamp: 6.0, EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 7.0, EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺"},
		{Timestamp: 4.0, EventType: EventTypeCast, SkillName: "裂蚀绽放"},
	}

	gaps := BuildCastIdleGaps(events)
	if len(gaps) != 2 {
		t.Fatalf("unexpected gap count: %d", len(gaps))
	}
	if gaps[0].Duration != 3.0 || gaps[1].Duration != 5.5 {
		t.Fatalf("unexpected gaps: %+v", gaps)
	}

	maxGap, ok := MaxCastIdleGap(events)
	if !ok {
		t.Fatal("expected max gap")
	}
	if maxGap.Duration != 5.5 {
		t.Fatalf("unexpected max gap: %+v", maxGap)
	}
}

func TestBuildCastIdleGaps_EmptySkillNameStillParticipates(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 2.0, EventType: EventTypeCast, SkillName: ""},
		{Timestamp: 8.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	gaps := BuildCastIdleGaps(events)
	if len(gaps) != 1 {
		t.Fatalf("unexpected gap count: %d", len(gaps))
	}
	if gaps[0].Duration != 6.0 {
		t.Fatalf("unexpected gap duration: %v", gaps[0].Duration)
	}
}

func TestFilterIdleGapsByThreshold(t *testing.T) {
	gaps := []IdleGap{
		{Start: 1.0, End: 4.0, Duration: 3.0},
		{Start: 4.0, End: 16.0, Duration: 12.0},
		{Start: 16.0, End: 23.5, Duration: 7.5},
	}

	filtered := FilterIdleGapsByThreshold(gaps, 8.0)
	if len(filtered) != 1 {
		t.Fatalf("unexpected filtered count: %d", len(filtered))
	}
	if filtered[0].Duration != 12.0 {
		t.Fatalf("unexpected filtered gap: %+v", filtered[0])
	}
}

func TestSummarizeMaxCastIdleGap(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 7.0, EventType: EventTypeCast},
		{Timestamp: 25.0, EventType: EventTypeCast},
	}

	summary := SummarizeMaxCastIdleGap(events)
	if summary != "连续 18.0 秒无技能释放" {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestBuildCastIdleGaps_KeepFloatPrecision(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 7.0, EventType: EventTypeCast},
		{Timestamp: 25.7, EventType: EventTypeCast},
	}

	gaps := BuildCastIdleGaps(events)
	if len(gaps) != 1 {
		t.Fatalf("unexpected gap count: %d", len(gaps))
	}
	if gaps[0].Duration != 18.7 {
		t.Fatalf("unexpected gap duration: %v", gaps[0].Duration)
	}
}
