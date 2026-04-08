package rules

// CountDotEvents counts DOT-related event occurrences only.
// This tracks event volume, not DOT damage totals, because the current rule
// layer first cares about whether DOT effects are actually running.
func CountDotEvents(events []RuleEvent) int {
	counts := CountDotEventsByType(events)
	return counts[EventTypeDotApply] + counts[EventTypeDotTick] + counts[EventTypeDotBurst]
}

// CountDotEventsByType counts only DOT apply/tick/burst events.
// Non-DOT events are intentionally ignored so cast rhythm and DOT effect
// activity remain separate rule dimensions.
func CountDotEventsByType(events []RuleEvent) map[string]int {
	counts := map[string]int{
		EventTypeDotApply: 0,
		EventTypeDotTick:  0,
		EventTypeDotBurst: 0,
	}

	for _, event := range events {
		switch event.EventType {
		case EventTypeDotApply, EventTypeDotTick, EventTypeDotBurst:
			counts[event.EventType]++
		}
	}

	return counts
}

// CountDotEventsBySkill counts DOT apply/tick/burst events grouped by skill.
// Empty skill names are skipped to avoid unstable grouping keys in skill-level stats.
func CountDotEventsBySkill(events []RuleEvent) map[string]int {
	counts := make(map[string]int)

	for _, event := range events {
		switch event.EventType {
		case EventTypeDotApply, EventTypeDotTick, EventTypeDotBurst:
			if event.SkillName == "" {
				continue
			}
			counts[event.SkillName]++
		}
	}

	return counts
}
