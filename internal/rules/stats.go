package rules

import "sort"

// SkillCastStat is a stable output view for cast-count statistics.
type SkillCastStat struct {
	SkillName string `json:"skill_name"`
	Casts     int    `json:"casts"`
}

// CountSkillCasts counts only true cast action events.
// skill_hit must be excluded because raw battle reports often emit both
// a cast-side event and a hit-side event for the same skill action.
// Empty skill names are skipped because they cannot form a stable group key.
func CountSkillCasts(events []RuleEvent) map[string]int {
	if len(events) == 0 {
		return nil
	}

	counts := make(map[string]int)
	for _, event := range events {
		if event.EventType != EventTypeCast {
			continue
		}
		if event.SkillName == "" {
			continue
		}
		counts[event.SkillName]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

// BuildSkillCastStats converts cast counts into a stable ordered slice.
// Results are sorted by casts descending, then skill name ascending.
func BuildSkillCastStats(events []RuleEvent) []SkillCastStat {
	counts := CountSkillCasts(events)
	if len(counts) == 0 {
		return nil
	}

	stats := make([]SkillCastStat, 0, len(counts))
	for skillName, casts := range counts {
		stats = append(stats, SkillCastStat{
			SkillName: skillName,
			Casts:     casts,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Casts != stats[j].Casts {
			return stats[i].Casts > stats[j].Casts
		}
		return stats[i].SkillName < stats[j].SkillName
	})

	return stats
}
