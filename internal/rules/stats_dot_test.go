package rules

import "testing"

func TestCountDotEvents_OnlyDotApply(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotApply, SkillName: "传染波"},
	}

	count := CountDotEvents(events)
	countsByType := CountDotEventsByType(events)

	if count != 1 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotApply] != 1 {
		t.Fatalf("unexpected dot_apply count: %d", countsByType[EventTypeDotApply])
	}
}

func TestCountDotEvents_OnlyDotTick(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
	}

	count := CountDotEvents(events)
	countsByType := CountDotEventsByType(events)

	if count != 3 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotTick] != 3 {
		t.Fatalf("unexpected dot_tick count: %d", countsByType[EventTypeDotTick])
	}
}

func TestCountDotEvents_OnlyDotBurst(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotBurst, SkillName: "裂蚀绽放"},
		{EventType: EventTypeDotBurst, SkillName: "裂蚀绽放"},
	}

	count := CountDotEvents(events)
	countsByType := CountDotEventsByType(events)

	if count != 2 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotBurst] != 2 {
		t.Fatalf("unexpected dot_burst count: %d", countsByType[EventTypeDotBurst])
	}
}

func TestCountDotEvents_MixedDotTypes(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotApply},
		{EventType: EventTypeDotApply},
		{EventType: EventTypeDotTick},
		{EventType: EventTypeDotTick},
		{EventType: EventTypeDotTick},
		{EventType: EventTypeDotTick},
		{EventType: EventTypeDotTick},
		{EventType: EventTypeDotBurst},
	}

	count := CountDotEvents(events)
	countsByType := CountDotEventsByType(events)

	if count != 8 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotApply] != 2 {
		t.Fatalf("unexpected dot_apply count: %d", countsByType[EventTypeDotApply])
	}
	if countsByType[EventTypeDotTick] != 5 {
		t.Fatalf("unexpected dot_tick count: %d", countsByType[EventTypeDotTick])
	}
	if countsByType[EventTypeDotBurst] != 1 {
		t.Fatalf("unexpected dot_burst count: %d", countsByType[EventTypeDotBurst])
	}
}

func TestCountDotEvents_IgnoresNonDotEvents(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeBasicAttack, SkillName: "普攻"},
		{EventType: EventTypeBuffGain, SkillName: "增益"},
		{EventType: EventTypeResourceGain, SkillName: "资源"},
		{EventType: EventTypeResourceSpend, SkillName: "资源"},
		{EventType: EventTypeEnemyHit, SkillName: "敌方打击"},
		{EventType: EventTypeUnknown, SkillName: "未知"},
	}

	count := CountDotEvents(events)
	countsByType := CountDotEventsByType(events)

	if count != 0 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotApply] != 0 || countsByType[EventTypeDotTick] != 0 || countsByType[EventTypeDotBurst] != 0 {
		t.Fatalf("unexpected counts by type: %+v", countsByType)
	}
}

func TestCountDotEvents_MixedDotAndNonDotEvents(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeCast, SkillName: "传染波"},
		{EventType: EventTypeDotApply, SkillName: "传染波"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeSkillHit, SkillName: "传染波"},
		{EventType: EventTypeDotBurst, SkillName: "裂蚀绽放"},
	}

	count := CountDotEvents(events)
	countsByType := CountDotEventsByType(events)

	if count != 6 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotApply] != 1 || countsByType[EventTypeDotTick] != 4 || countsByType[EventTypeDotBurst] != 1 {
		t.Fatalf("unexpected counts by type: %+v", countsByType)
	}
}

func TestCountDotEvents_EmptySkillNameStillCountsTotal(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotTick, SkillName: ""},
	}

	count := CountDotEvents(events)
	countsByType := CountDotEventsByType(events)
	countsBySkill := CountDotEventsBySkill(events)

	if count != 1 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotTick] != 1 {
		t.Fatalf("unexpected dot_tick count: %d", countsByType[EventTypeDotTick])
	}
	if len(countsBySkill) != 0 {
		t.Fatalf("expected empty counts by skill, got: %+v", countsBySkill)
	}
}

func TestCountDotEvents_EmptyInput(t *testing.T) {
	count := CountDotEvents(nil)
	countsByType := CountDotEventsByType(nil)
	countsBySkill := CountDotEventsBySkill(nil)

	if count != 0 {
		t.Fatalf("unexpected dot count: %d", count)
	}
	if countsByType[EventTypeDotApply] != 0 || countsByType[EventTypeDotTick] != 0 || countsByType[EventTypeDotBurst] != 0 {
		t.Fatalf("unexpected counts by type: %+v", countsByType)
	}
	if len(countsBySkill) != 0 {
		t.Fatalf("expected empty counts by skill, got: %+v", countsBySkill)
	}
}

func TestCountDotEventsBySkill_GroupsBySkill(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotApply, SkillName: "传染波"},
		{EventType: EventTypeDotApply, SkillName: "传染波"},
		{EventType: EventTypeDotBurst, SkillName: "裂蚀绽放"},
	}

	countsBySkill := CountDotEventsBySkill(events)

	if countsBySkill["毒蚀穿刺"] != 3 {
		t.Fatalf("unexpected 毒蚀穿刺 count: %d", countsBySkill["毒蚀穿刺"])
	}
	if countsBySkill["传染波"] != 2 {
		t.Fatalf("unexpected 传染波 count: %d", countsBySkill["传染波"])
	}
	if countsBySkill["裂蚀绽放"] != 1 {
		t.Fatalf("unexpected 裂蚀绽放 count: %d", countsBySkill["裂蚀绽放"])
	}
}

func TestCountDotEvents_IsNotDeduplicated(t *testing.T) {
	events := []RuleEvent{
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
	}

	count := CountDotEvents(events)
	if count != 5 {
		t.Fatalf("unexpected dot count: %d", count)
	}
}
