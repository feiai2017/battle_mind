package rules

import "sort"

// SkillLastCastStat is a stable output view for last-cast statistics.
type SkillLastCastStat struct {
	SkillName string  `json:"skill_name"`
	LastCast  float64 `json:"last_cast"`
}

// BuildLastSkillCastTimes returns the last true cast time for each skill.
// This intentionally tracks the last cast-side action, not the last related
// hit or DOT effect, so later rules can reason about rotation stoppage.
// Empty skill names are skipped because they cannot form a stable group key.
func BuildLastSkillCastTimes(events []RuleEvent) map[string]float64 {
	result := make(map[string]float64)

	for _, event := range events {
		if event.EventType != EventTypeCast {
			continue
		}
		if event.SkillName == "" {
			continue
		}

		last, exists := result[event.SkillName]
		if !exists || event.Timestamp > last {
			result[event.SkillName] = event.Timestamp
		}
	}

	return result
}

// LastSkillCastTime returns the last true cast time for one skill.
// It does not treat skill_hit or dot_tick as a cast because those represent
// downstream effects, not the action timestamp that rules care about.
func LastSkillCastTime(events []RuleEvent, skillName string) (float64, bool) {
	if skillName == "" {
		return 0, false
	}

	var last float64
	found := false

	for _, event := range events {
		if event.EventType != EventTypeCast {
			continue
		}
		if event.SkillName != skillName {
			continue
		}
		if !found || event.Timestamp > last {
			last = event.Timestamp
			found = true
		}
	}

	return last, found
}

// BuildSkillLastCastStats converts last-cast times into a stable ordered slice.
// Results are sorted by last cast descending, then skill name ascending.
func BuildSkillLastCastStats(events []RuleEvent) []SkillLastCastStat {
	lastCastTimes := BuildLastSkillCastTimes(events)
	if len(lastCastTimes) == 0 {
		return nil
	}

	stats := make([]SkillLastCastStat, 0, len(lastCastTimes))
	for skillName, lastCast := range lastCastTimes {
		stats = append(stats, SkillLastCastStat{
			SkillName: skillName,
			LastCast:  lastCast,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].LastCast != stats[j].LastCast {
			return stats[i].LastCast > stats[j].LastCast
		}
		return stats[i].SkillName < stats[j].SkillName
	})

	return stats
}
