package rules

import "sort"

// BuildDotIdleGaps calculates idle gaps between adjacent DOT-active events.
// DOT continuity only considers apply/tick/burst activity and ignores other
// event categories so the rule stays scoped to DOT uptime itself.
func BuildDotIdleGaps(events []RuleEvent) []IdleGap {
	dotTimes := make([]float64, 0, len(events))
	for _, event := range events {
		if !isDotActiveEventType(event.EventType) {
			continue
		}
		dotTimes = append(dotTimes, event.Timestamp)
	}

	if len(dotTimes) < 2 {
		return nil
	}

	sort.Float64s(dotTimes)

	gaps := make([]IdleGap, 0, len(dotTimes)-1)
	for i := 1; i < len(dotTimes); i++ {
		start := dotTimes[i-1]
		end := dotTimes[i]
		gaps = append(gaps, IdleGap{
			Start:    start,
			End:      end,
			Duration: end - start,
		})
	}

	return gaps
}

// MaxDotIdleGap returns the longest gap between adjacent DOT-active events.
func MaxDotIdleGap(events []RuleEvent) (IdleGap, bool) {
	gaps := BuildDotIdleGaps(events)
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

// HasLongDotGap reports whether DOT activity has any gap at or above threshold.
func HasLongDotGap(events []RuleEvent, threshold float64) bool {
	maxGap, ok := MaxDotIdleGap(events)
	if !ok {
		return false
	}
	return maxGap.Duration >= threshold
}

func isDotActiveEventType(eventType string) bool {
	switch eventType {
	case EventTypeDotApply, EventTypeDotTick, EventTypeDotBurst:
		return true
	default:
		return false
	}
}
