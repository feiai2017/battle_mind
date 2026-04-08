package rules

import (
	"fmt"
	"sort"
)

// IdleGap describes the idle window between two adjacent cast events.
type IdleGap struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Duration float64 `json:"duration"`
}

// BuildCastIdleGaps calculates global cast idle gaps from adjacent cast events.
// This intentionally uses only cast events because the rule cares about active
// skill-release rhythm, not downstream hit or DOT-effect timestamps.
// Unlike skill-grouped stats, empty skill names are still kept here because
// any cast event still means a release happened and should break an idle gap.
func BuildCastIdleGaps(events []RuleEvent) []IdleGap {
	castTimes := make([]float64, 0, len(events))
	for _, event := range events {
		if event.EventType != EventTypeCast {
			continue
		}
		castTimes = append(castTimes, event.Timestamp)
	}

	if len(castTimes) < 2 {
		return nil
	}

	sort.Float64s(castTimes)

	gaps := make([]IdleGap, 0, len(castTimes)-1)
	for i := 1; i < len(castTimes); i++ {
		start := castTimes[i-1]
		end := castTimes[i]
		gaps = append(gaps, IdleGap{
			Start:    start,
			End:      end,
			Duration: end - start,
		})
	}

	return gaps
}

// MaxCastIdleGap returns the longest gap between adjacent cast events.
func MaxCastIdleGap(events []RuleEvent) (IdleGap, bool) {
	gaps := BuildCastIdleGaps(events)
	if len(gaps) == 0 {
		return IdleGap{}, false
	}

	maxGap := gaps[0]
	for i := 1; i < len(gaps); i++ {
		if gaps[i].Duration > maxGap.Duration {
			maxGap = gaps[i]
		}
	}

	return maxGap, true
}

// FilterIdleGapsByThreshold keeps gaps whose duration is at least minDuration.
func FilterIdleGapsByThreshold(gaps []IdleGap, minDuration float64) []IdleGap {
	if len(gaps) == 0 {
		return nil
	}

	filtered := make([]IdleGap, 0, len(gaps))
	for _, gap := range gaps {
		if gap.Duration >= minDuration {
			filtered = append(filtered, gap)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// SummarizeMaxCastIdleGap returns a readable summary for the largest cast gap.
// It only measures adjacent cast-to-cast gaps and intentionally excludes
// battle-start and battle-end edges to keep the definition stable.
func SummarizeMaxCastIdleGap(events []RuleEvent) string {
	gap, ok := MaxCastIdleGap(events)
	if !ok {
		return ""
	}
	return fmt.Sprintf("连续 %.1f 秒无技能释放", gap.Duration)
}
