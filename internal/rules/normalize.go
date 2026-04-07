package rules

import (
	"strings"

	"github.com/feiai2017/battle_mind/internal/model"
)

// NormalizeRuleEvents projects battle report events into the minimal rule view.
// It keeps the original event order so downstream counters can rely on report order.
func NormalizeRuleEvents(report model.BattleReport) []RuleEvent {
	return NormalizeReportEvents(report.Events)
}

// NormalizeReportEvents converts report events into rule events and filters events
// that the first rule layer should ignore.
func NormalizeReportEvents(events []model.ReportEvent) []RuleEvent {
	if len(events) == 0 {
		return nil
	}

	result := make([]RuleEvent, 0, len(events))
	for _, event := range events {
		ruleEvent, ok := ToRuleEvent(event)
		if !ok {
			continue
		}
		result = append(result, ruleEvent)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ToRuleEvent converts one battle report event into a RuleEvent.
// SKILL_CAST is intentionally split into cast and skill_hit so cast counts
// only track the action event and do not double-count hit-side events.
func ToRuleEvent(event model.ReportEvent) (RuleEvent, bool) {
	eventType, ok := normalizeEventType(event)
	if !ok {
		return RuleEvent{}, false
	}

	return RuleEvent{
		Timestamp: event.Time,
		EventType: eventType,
		SkillName: safeString(event.SourceName),
		Target:    safeString(event.TargetName),
		Value:     safeFloat64(event.Amount),
	}, true
}

func normalizeEventType(event model.ReportEvent) (string, bool) {
	rawType := strings.ToUpper(strings.TrimSpace(event.Type))
	targetName := safeString(event.TargetName)

	switch rawType {
	case "SKILL_DECISION":
		return "", false
	case "SKILL_CAST":
		// Raw reports emit both the cast action and the hit-side record as SKILL_CAST.
		// Treat target-bearing records as hits so later cast counters are not duplicated.
		if targetName != "" || hasTag(event.Tags, "hit") {
			return EventTypeSkillHit, true
		}
		return EventTypeCast, true
	case "BASIC_ATTACK":
		return EventTypeBasicAttack, true
	case "DOT_APPLY":
		return EventTypeDotApply, true
	case "DOT_TICK":
		return EventTypeDotTick, true
	case "DOT_BURST":
		return EventTypeDotBurst, true
	case "BUFF_GAIN":
		return EventTypeBuffGain, true
	case "RESOURCE_GAIN":
		return EventTypeResourceGain, true
	case "RESOURCE_SPEND":
		return EventTypeResourceSpend, true
	case "ENEMY_HIT":
		return EventTypeEnemyHit, true
	default:
		return EventTypeUnknown, true
	}
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func safeFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func hasTag(tags []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), want) {
			return true
		}
	}
	return false
}
