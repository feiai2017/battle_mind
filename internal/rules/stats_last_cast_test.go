package rules

import "testing"

func TestLastSkillCastTime_SingleCast(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	last, ok := LastSkillCastTime(events, "毒蚀穿刺")
	if !ok {
		t.Fatal("expected skill cast time to exist")
	}
	if last != 1.0 {
		t.Fatalf("unexpected last cast time: %v", last)
	}
}

func TestLastSkillCastTime_MultipleCastsUsesMaxTimestamp(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 4.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 7.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	last, ok := LastSkillCastTime(events, "毒蚀穿刺")
	if !ok {
		t.Fatal("expected skill cast time to exist")
	}
	if last != 7.0 {
		t.Fatalf("unexpected last cast time: %v", last)
	}
}

func TestBuildLastSkillCastTimes_MultipleSkills(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 4.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 7.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 2.8, EventType: EventTypeCast, SkillName: "传染波"},
		{Timestamp: 9.7, EventType: EventTypeCast, SkillName: "传染波"},
		{Timestamp: 7.9, EventType: EventTypeCast, SkillName: "裂蚀绽放"},
		{Timestamp: 16.8, EventType: EventTypeCast, SkillName: "裂蚀绽放"},
	}

	lastCastTimes := BuildLastSkillCastTimes(events)

	if len(lastCastTimes) != 3 {
		t.Fatalf("unexpected result size: %d", len(lastCastTimes))
	}
	if lastCastTimes["毒蚀穿刺"] != 7.0 {
		t.Fatalf("unexpected 毒蚀穿刺 last cast: %v", lastCastTimes["毒蚀穿刺"])
	}
	if lastCastTimes["传染波"] != 9.7 {
		t.Fatalf("unexpected 传染波 last cast: %v", lastCastTimes["传染波"])
	}
	if lastCastTimes["裂蚀绽放"] != 16.8 {
		t.Fatalf("unexpected 裂蚀绽放 last cast: %v", lastCastTimes["裂蚀绽放"])
	}
}

func TestLastSkillCastTime_IgnoresSkillHitAndDotTick(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 7.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 7.0, EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺", Target: "敌人1"},
		{Timestamp: 7.1, EventType: EventTypeDotTick, SkillName: "毒蚀穿刺", Target: "敌人1"},
	}

	last, ok := LastSkillCastTime(events, "毒蚀穿刺")
	if !ok {
		t.Fatal("expected skill cast time to exist")
	}
	if last != 7.0 {
		t.Fatalf("unexpected last cast time: %v", last)
	}
}

func TestBuildLastSkillCastTimes_IgnoresNonCastEvents(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeBasicAttack, SkillName: "普攻"},
		{Timestamp: 2.0, EventType: EventTypeDotApply, SkillName: "传染"},
		{Timestamp: 3.0, EventType: EventTypeDotTick, SkillName: "传染"},
		{Timestamp: 4.0, EventType: EventTypeDotBurst, SkillName: "传染"},
		{Timestamp: 5.0, EventType: EventTypeResourceGain, SkillName: "专注"},
		{Timestamp: 6.0, EventType: EventTypeResourceSpend, SkillName: "专注"},
		{Timestamp: 7.0, EventType: EventTypeEnemyHit, SkillName: "敌方打击"},
	}

	lastCastTimes := BuildLastSkillCastTimes(events)
	if len(lastCastTimes) != 0 {
		t.Fatalf("expected empty result, got: %+v", lastCastTimes)
	}
}

func TestLastSkillCastTime_SkipsEmptySkillName(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 12.0, EventType: EventTypeCast, SkillName: ""},
	}

	lastCastTimes := BuildLastSkillCastTimes(events)
	if len(lastCastTimes) != 0 {
		t.Fatalf("expected empty result, got: %+v", lastCastTimes)
	}

	last, ok := LastSkillCastTime(events, "")
	if ok || last != 0 {
		t.Fatalf("expected no result, got last=%v ok=%v", last, ok)
	}
}

func TestLastSkillCastTime_NotFound(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	last, ok := LastSkillCastTime(events, "裂蚀绽放")
	if ok || last != 0 {
		t.Fatalf("expected no result, got last=%v ok=%v", last, ok)
	}
}

func TestLastSkillCastTime_EmptyInput(t *testing.T) {
	lastCastTimes := BuildLastSkillCastTimes(nil)
	if lastCastTimes == nil {
		t.Fatal("expected empty map, got nil")
	}
	if len(lastCastTimes) != 0 {
		t.Fatalf("unexpected result size: %d", len(lastCastTimes))
	}

	last, ok := LastSkillCastTime(nil, "毒蚀穿刺")
	if ok || last != 0 {
		t.Fatalf("expected no result, got last=%v ok=%v", last, ok)
	}
}

func TestLastSkillCastTime_UnorderedInputStillUsesMaxTimestamp(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 7.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 4.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	last, ok := LastSkillCastTime(events, "毒蚀穿刺")
	if !ok {
		t.Fatal("expected skill cast time to exist")
	}
	if last != 7.0 {
		t.Fatalf("unexpected last cast time: %v", last)
	}
}

func TestBuildSkillLastCastStats_SortsByTimeThenName(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 9.7, EventType: EventTypeCast, SkillName: "传染波"},
		{Timestamp: 16.8, EventType: EventTypeCast, SkillName: "裂蚀绽放"},
		{Timestamp: 16.8, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
	}

	stats := BuildSkillLastCastStats(events)

	if len(stats) != 3 {
		t.Fatalf("unexpected stats size: %d", len(stats))
	}
	if stats[0].SkillName != "毒蚀穿刺" || stats[0].LastCast != 16.8 {
		t.Fatalf("unexpected first stat: %+v", stats[0])
	}
	if stats[1].SkillName != "裂蚀绽放" || stats[1].LastCast != 16.8 {
		t.Fatalf("unexpected second stat: %+v", stats[1])
	}
	if stats[2].SkillName != "传染波" || stats[2].LastCast != 9.7 {
		t.Fatalf("unexpected third stat: %+v", stats[2])
	}
}
