package rules

const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"

	FindingLongCastIdleGap  = "LONG_CAST_IDLE_GAP"
	FindingLowDotActivity   = "LOW_DOT_ACTIVITY"
	FindingNoBurstEvent     = "NO_BURST_EVENT"
	SignalLowBurstFrequency = "LOW_BURST_FREQUENCY"
)

const (
	longCastIdleGapThreshold   = 10.0
	lowDotActivityThreshold    = 5
	lowBurstFrequencyThreshold = 1
	burstSkillName             = "裂蚀绽放"
)

// RuleFinding is a compact rule-layer finding with structured evidence.
type RuleFinding struct {
	Code     string                 `json:"code"`
	Severity string                 `json:"severity"`
	Message  string                 `json:"message"`
	Evidence map[string]interface{} `json:"evidence,omitempty"`
}

// RuleSummaryMetric collects rule-layer evidence so callers do not need to
// recompute scattered stats in prompt builders or diagnosis assemblers.
type RuleSummaryMetric struct {
	SkillCasts          map[string]int     `json:"skill_casts"`
	LastCastTimes       map[string]float64 `json:"last_cast_times"`
	DotEventCount       int                `json:"dot_event_count"`
	DotEventCountByType map[string]int     `json:"dot_event_count_by_type"`
	MaxCastIdleGap      *IdleGap           `json:"max_cast_idle_gap,omitempty"`
}

// RuleSummary is a lightweight rule-layer summary, not another battle report.
// It carries hard findings, softer suspicious signals, and structured metrics.
type RuleSummary struct {
	HardFindings      []RuleFinding     `json:"hard_findings"`
	SuspiciousSignals []RuleFinding     `json:"suspicious_signals"`
	Metrics           RuleSummaryMetric `json:"metrics"`
}

// BuildRuleSummary folds existing rule stats into one stable summary object.
// It intentionally depends only on rule-layer events and helper functions.
func BuildRuleSummary(events []RuleEvent) RuleSummary {
	metrics := buildRuleSummaryMetrics(events)

	return RuleSummary{
		HardFindings:      buildHardFindings(metrics),
		SuspiciousSignals: buildSuspiciousSignals(metrics),
		Metrics:           metrics,
	}
}

func buildRuleSummaryMetrics(events []RuleEvent) RuleSummaryMetric {
	skillCasts := CountSkillCasts(events)
	if skillCasts == nil {
		skillCasts = map[string]int{}
	}

	lastCastTimes := BuildLastSkillCastTimes(events)
	if lastCastTimes == nil {
		lastCastTimes = map[string]float64{}
	}

	dotEventCountByType := CountDotEventsByType(events)
	if dotEventCountByType == nil {
		dotEventCountByType = map[string]int{
			EventTypeDotApply: 0,
			EventTypeDotTick:  0,
			EventTypeDotBurst: 0,
		}
	}

	var maxCastIdleGap *IdleGap
	if gap, ok := MaxCastIdleGap(events); ok {
		gapCopy := gap
		maxCastIdleGap = &gapCopy
	}

	return RuleSummaryMetric{
		SkillCasts:          skillCasts,
		LastCastTimes:       lastCastTimes,
		DotEventCount:       CountDotEvents(events),
		DotEventCountByType: dotEventCountByType,
		MaxCastIdleGap:      maxCastIdleGap,
	}
}

func buildHardFindings(metrics RuleSummaryMetric) []RuleFinding {
	findings := make([]RuleFinding, 0, 3)
	if !hasRuleActivity(metrics) {
		return findings
	}

	if metrics.MaxCastIdleGap != nil && metrics.MaxCastIdleGap.Duration >= longCastIdleGapThreshold {
		findings = append(findings, RuleFinding{
			Code:     FindingLongCastIdleGap,
			Severity: SeverityWarning,
			Message:  "存在较长技能施法空窗",
			Evidence: map[string]interface{}{
				"duration": metrics.MaxCastIdleGap.Duration,
				"start":    metrics.MaxCastIdleGap.Start,
				"end":      metrics.MaxCastIdleGap.End,
			},
		})
	}

	if metrics.DotEventCount <= lowDotActivityThreshold {
		findings = append(findings, RuleFinding{
			Code:     FindingLowDotActivity,
			Severity: SeverityWarning,
			Message:  "DOT 相关事件偏少",
			Evidence: map[string]interface{}{
				"dot_event_count": metrics.DotEventCount,
				"dot_apply":       metrics.DotEventCountByType[EventTypeDotApply],
				"dot_tick":        metrics.DotEventCountByType[EventTypeDotTick],
				"dot_burst":       metrics.DotEventCountByType[EventTypeDotBurst],
			},
		})
	}

	if metrics.DotEventCountByType[EventTypeDotBurst] == 0 {
		findings = append(findings, RuleFinding{
			Code:     FindingNoBurstEvent,
			Severity: SeverityInfo,
			Message:  "未观察到 DOT 爆发事件",
			Evidence: map[string]interface{}{
				"dot_burst_count": metrics.DotEventCountByType[EventTypeDotBurst],
			},
		})
	}

	return findings
}

func buildSuspiciousSignals(metrics RuleSummaryMetric) []RuleFinding {
	signals := make([]RuleFinding, 0, 2)
	if len(metrics.SkillCasts) == 0 {
		return signals
	}

	burstSkillCasts := metrics.SkillCasts[burstSkillName]
	if burstSkillCasts <= lowBurstFrequencyThreshold {
		signals = append(signals, RuleFinding{
			Code:     SignalLowBurstFrequency,
			Severity: SeverityInfo,
			Message:  "爆发技能施放次数偏少，值得关注",
			Evidence: map[string]interface{}{
				"skill_name": burstSkillName,
				"casts":      burstSkillCasts,
			},
		})
	}

	return signals
}

func hasRuleActivity(metrics RuleSummaryMetric) bool {
	return len(metrics.SkillCasts) > 0 || len(metrics.LastCastTimes) > 0 || metrics.DotEventCount > 0 || metrics.MaxCastIdleGap != nil
}
