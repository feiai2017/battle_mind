package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/feiai2017/battle_mind/internal/model"
)

func TestToRuleEvent_CastAction(t *testing.T) {
	amount := 14.0
	sourceName := "Toxic Lance"

	event, ok := ToRuleEvent(model.ReportEvent{
		Time:       1,
		Type:       "SKILL_CAST",
		SourceName: &sourceName,
		Amount:     &amount,
		Tags:       []string{"cast"},
	})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.EventType != EventTypeCast {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	if event.SkillName != "Toxic Lance" {
		t.Fatalf("unexpected skill name: %s", event.SkillName)
	}
	if event.Target != "" {
		t.Fatalf("unexpected target: %q", event.Target)
	}
	if event.Value != 14 {
		t.Fatalf("unexpected value: %v", event.Value)
	}
}

func TestToRuleEvent_SkillHit(t *testing.T) {
	targetName := "Enemy 1"
	sourceName := "Toxic Lance"
	amount := 65.0

	event, ok := ToRuleEvent(model.ReportEvent{
		Time:       1,
		Type:       "SKILL_CAST",
		SourceName: &sourceName,
		TargetName: &targetName,
		Amount:     &amount,
		Tags:       []string{"hit"},
	})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.EventType != EventTypeSkillHit {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	if event.Target != "Enemy 1" {
		t.Fatalf("unexpected target: %s", event.Target)
	}
}

func TestToRuleEvent_DotApply(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "DOT_APPLY"})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.EventType != EventTypeDotApply {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}

func TestToRuleEvent_DotTick(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "DOT_TICK"})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.EventType != EventTypeDotTick {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}

func TestToRuleEvent_DotBurst(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "DOT_BURST"})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.EventType != EventTypeDotBurst {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}

func TestToRuleEvent_ResourceGain(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "RESOURCE_GAIN"})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.EventType != EventTypeResourceGain {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}

func TestToRuleEvent_ResourceSpend(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "RESOURCE_SPEND"})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.EventType != EventTypeResourceSpend {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}

func TestToRuleEvent_NilAmountFallsBackToZero(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "DOT_TICK"})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.Value != 0 {
		t.Fatalf("unexpected value: %v", event.Value)
	}
}

func TestToRuleEvent_NilNamesFallBackToEmpty(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "DOT_APPLY"})
	if !ok {
		t.Fatal("expected event to be kept")
	}
	if event.SkillName != "" {
		t.Fatalf("unexpected skill name: %q", event.SkillName)
	}
	if event.Target != "" {
		t.Fatalf("unexpected target: %q", event.Target)
	}
}

func TestToRuleEvent_SkillDecisionFiltered(t *testing.T) {
	_, ok := ToRuleEvent(model.ReportEvent{Type: "SKILL_DECISION"})
	if ok {
		t.Fatal("expected skill decision to be filtered out")
	}
}

func TestToRuleEvent_UnknownTypeKept(t *testing.T) {
	event, ok := ToRuleEvent(model.ReportEvent{Type: "SOMETHING_NEW"})
	if !ok {
		t.Fatal("expected unknown event to be kept")
	}
	if event.EventType != EventTypeUnknown {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}

func TestNormalizeReportEvents_SkillCastSplit(t *testing.T) {
	sourceName := "Toxic Lance"
	targetName := "Enemy 1"
	castAmount := 14.0
	hitAmount := 65.0

	events := NormalizeReportEvents([]model.ReportEvent{
		{
			Time:       1,
			Type:       "SKILL_CAST",
			SourceName: &sourceName,
			Amount:     &castAmount,
			Tags:       []string{"cast"},
		},
		{
			Time:       1,
			Type:       "SKILL_CAST",
			SourceName: &sourceName,
			TargetName: &targetName,
			Amount:     &hitAmount,
			Tags:       []string{"hit"},
		},
	})

	if len(events) != 2 {
		t.Fatalf("unexpected event count: %d", len(events))
	}
	if events[0].EventType != EventTypeCast {
		t.Fatalf("unexpected first event type: %s", events[0].EventType)
	}
	if events[1].EventType != EventTypeSkillHit {
		t.Fatalf("unexpected second event type: %s", events[1].EventType)
	}
}

func TestNormalizeRuleEvents_ParsesExistingBattleReportEvents(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "battle-report", "battle-report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read battle report failed: %v", err)
	}

	var report model.BattleReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal battle report failed: %v", err)
	}
	if len(report.Events) == 0 {
		t.Fatal("expected battle report events")
	}

	events := NormalizeRuleEvents(report)
	if len(events) == 0 {
		t.Fatal("expected normalized rule events")
	}
	if events[0].Timestamp != report.Events[0].Time {
		t.Fatalf("unexpected first timestamp: %v", events[0].Timestamp)
	}
}
