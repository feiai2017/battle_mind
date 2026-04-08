package rules

import "testing"

func TestCountSkillCasts_CountsOnlyCastEvents(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeBasicAttack, SkillName: "普攻"},
		{EventType: EventTypeDotTick, SkillName: "腐蚀"},
	}

	counts := CountSkillCasts(events)

	if len(counts) != 1 {
		t.Fatalf("unexpected count size: %d", len(counts))
	}
	if counts["毒蚀穿刺"] != 2 {
		t.Fatalf("unexpected cast count: %d", counts["毒蚀穿刺"])
	}
}

func TestCountSkillCasts_SkipsEmptySkillName(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeCast, SkillName: ""},
		{EventType: EventTypeCast, SkillName: "传染波"},
	}

	counts := CountSkillCasts(events)

	if len(counts) != 1 {
		t.Fatalf("unexpected count size: %d", len(counts))
	}
	if _, ok := counts[""]; ok {
		t.Fatal("expected empty skill name to be skipped")
	}
	if counts["传染波"] != 1 {
		t.Fatalf("unexpected cast count: %d", counts["传染波"])
	}
}

func TestCountSkillCasts_KeepsDifferentSkillsSeparated(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeCast, SkillName: "裂蚀绽放"},
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeCast, SkillName: "传染波"},
		{EventType: EventTypeCast, SkillName: "传染波"},
	}

	counts := CountSkillCasts(events)

	if counts["毒蚀穿刺"] != 2 {
		t.Fatalf("unexpected 毒蚀穿刺 count: %d", counts["毒蚀穿刺"])
	}
	if counts["裂蚀绽放"] != 1 {
		t.Fatalf("unexpected 裂蚀绽放 count: %d", counts["裂蚀绽放"])
	}
	if counts["传染波"] != 2 {
		t.Fatalf("unexpected 传染波 count: %d", counts["传染波"])
	}
}

func TestBuildSkillCastStats_SortsByCastsThenName(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeCast, SkillName: "裂蚀绽放"},
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeCast, SkillName: "传染波"},
		{EventType: EventTypeCast, SkillName: "传染波"},
	}

	stats := BuildSkillCastStats(events)

	if len(stats) != 3 {
		t.Fatalf("unexpected stats size: %d", len(stats))
	}
	if stats[0].SkillName != "传染波" || stats[0].Casts != 2 {
		t.Fatalf("unexpected first stat: %+v", stats[0])
	}
	if stats[1].SkillName != "毒蚀穿刺" || stats[1].Casts != 2 {
		t.Fatalf("unexpected second stat: %+v", stats[1])
	}
	if stats[2].SkillName != "裂蚀绽放" || stats[2].Casts != 1 {
		t.Fatalf("unexpected third stat: %+v", stats[2])
	}
}

func TestBuildSkillCastStats_ReturnsNilWhenNoCastEvents(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotApply, SkillName: "传染"},
	}

	stats := BuildSkillCastStats(events)
	if stats != nil {
		t.Fatalf("expected nil stats, got: %+v", stats)
	}
}

func TestCountSkillCasts_CastAndSkillHitAreSeparated(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 1, EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺", Target: "敌人1"},
	}

	counts := CountSkillCasts(events)

	// Later cast-frequency rules must count only the cast-side action event.
	if counts["毒蚀穿刺"] != 1 {
		t.Fatalf("unexpected cast count: %d", counts["毒蚀穿刺"])
	}
}
