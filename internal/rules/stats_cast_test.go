package rules

import "testing"

func TestCountSkillCasts_UsesOnlyCastEventsAndSkipsEmptyNames(t *testing.T) {
	events := []RuleEvent{
		{Timestamp: 1.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 1.1, EventType: EventTypeSkillHit, SkillName: "毒蚀穿刺"},
		{Timestamp: 2.0, EventType: EventTypeCast, SkillName: "毒蚀穿刺"},
		{Timestamp: 3.0, EventType: EventTypeCast, SkillName: "裂蚀绽放"},
		{Timestamp: 4.0, EventType: EventTypeCast, SkillName: "传染波"},
		{Timestamp: 5.0, EventType: EventTypeCast, SkillName: "传染波"},
		{Timestamp: 6.0, EventType: EventTypeBasicAttack, SkillName: "普攻"},
		{Timestamp: 7.0, EventType: EventTypeDotTick, SkillName: "毒蚀穿刺"},
		{Timestamp: 8.0, EventType: EventTypeCast, SkillName: ""},
	}

	counts := CountSkillCasts(events)

	if len(counts) != 3 {
		t.Fatalf("unexpected count size: %d", len(counts))
	}
	if counts["毒蚀穿刺"] != 2 {
		t.Fatalf("unexpected 毒蚀穿刺 count: %d", counts["毒蚀穿刺"])
	}
	if counts["裂蚀绽放"] != 1 {
		t.Fatalf("unexpected 裂蚀绽放 count: %d", counts["裂蚀绽放"])
	}
	if counts["传染波"] != 2 {
		t.Fatalf("unexpected 传染波 count: %d", counts["传染波"])
	}
	if _, ok := counts[""]; ok {
		t.Fatal("expected empty skill name to be skipped")
	}
}
