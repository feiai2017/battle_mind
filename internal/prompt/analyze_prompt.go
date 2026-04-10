package prompt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/rules"
)

type AnalyzePromptInput struct {
	Request     model.AnalyzeRequest
	Report      *model.BattleReport
	RuleSummary rules.RuleSummary
}

func BuildAnalyzePrompt(input AnalyzePromptInput) string {
	parts := []string{
		renderInstructionBlock(),
		RenderRuleSummaryPrompt(input.RuleSummary),
	}

	if input.Report != nil {
		parts = append(parts, RenderReportContextPrompt(*input.Report))
	} else {
		parts = append(parts, renderAnalyzeRequestContextPrompt(input.Request))
	}

	return strings.Join(parts, "\n\n")
}

func renderInstructionBlock() string {
	return strings.Join([]string{
		"[Instruction]",
		"你是一个游戏战斗分析助手。",
		"系统已经直接输出 rule_findings，你不需要重复完整罗列规则明确发现。",
		"你会同时看到 RuleSummary 和 battle report 关键上下文。",
		"你的任务只是在这些信息之上生成 model_suggestions，也就是解释、归因、建议和风险提醒。",
		"请优先参考 RuleSummary 中的 hard_findings，把其中已确认的规则事实作为解释基础。",
		"请把 suspicious_signals 仅作为值得关注的信号，语气必须保守，不要当成确定事实。",
		"metrics 是规则证据，可用于解释问题和支持结论。",
		"如果规则没有覆盖到某些细节，再结合 battle report 关键上下文进行补充。",
		"如果 battle report 与 RuleSummary 存在张力，优先尊重 RuleSummary 中的明确规则发现。",
		"不要把没有规则支持的猜测说成确定原因。",
		"不要机械重复 hard_findings，必要时可以简短引用，但重点放在为什么值得关注、建议优先检查什么、可能存在什么风险。",
		"所有自然语言输出字段必须使用简体中文。",
		"最终只输出合法 JSON，不要输出 markdown，不要输出解释，不要输出代码块。",
		"输出格式如下：",
		`{"summary":"一句话解释","suggestions":["建议1"],"risks":["风险1"]}`,
	}, "\n")
}

func RenderRuleSummaryPrompt(summary rules.RuleSummary) string {
	var builder strings.Builder

	builder.WriteString("[Hard Findings]\n")
	writeFindingsSection(&builder, summary.HardFindings)
	builder.WriteString("\n[Suspicious Signals]\n")
	writeFindingsSection(&builder, summary.SuspiciousSignals)
	builder.WriteString("\n[Rule Metrics]\n")
	writeRuleMetrics(&builder, summary.Metrics)

	return builder.String()
}

func RenderReportContextPrompt(report model.BattleReport) string {
	var builder strings.Builder

	builder.WriteString("[Battle Report Context]\n")
	builder.WriteString("[Battle Result]\n")
	builder.WriteString(fmt.Sprintf("- win: %t\n", report.ResultSummary.Win))
	builder.WriteString(fmt.Sprintf("- duration: %.1f\n", report.ResultSummary.Duration))
	builder.WriteString(fmt.Sprintf("- likely_reason: %s\n", safePromptString(report.ResultSummary.LikelyReason)))

	builder.WriteString("\n[Aggregate Metrics]\n")
	builder.WriteString("- skill_usage:\n")
	writeSkillUsage(&builder, report.AggregateMetrics.SkillUsage)
	builder.WriteString("- damage_by_source:\n")
	writeDamageBySource(&builder, report.AggregateMetrics.DamageBySource)

	builder.WriteString("\n[Existing Diagnosis]\n")
	if len(report.Diagnosis) == 0 {
		builder.WriteString("- none\n")
	} else {
		for _, item := range report.Diagnosis {
			builder.WriteString(fmt.Sprintf("- %s (%s): %s\n", safePromptString(item.Code), safePromptString(item.Severity), safePromptString(item.Message)))
		}
	}

	return builder.String()
}

func renderAnalyzeRequestContextPrompt(req model.AnalyzeRequest) string {
	var builder strings.Builder

	builder.WriteString("[Analyze Request Context]\n")
	builder.WriteString(fmt.Sprintf("- battle_type: %s\n", safePromptString(req.BattleType)))
	builder.WriteString(fmt.Sprintf("- build_tags: %s\n", joinOrNone(req.BuildTags)))
	builder.WriteString(fmt.Sprintf("- notes: %s\n", safePromptString(req.Notes)))
	builder.WriteString(fmt.Sprintf("- summary.win: %t\n", req.Summary.Win))
	builder.WriteString(fmt.Sprintf("- summary.duration: %d\n", req.Summary.Duration))
	builder.WriteString(fmt.Sprintf("- summary.likely_reason: %s\n", safePromptString(req.Summary.LikelyReason)))

	builder.WriteString("\n[Request Metrics]\n")
	builder.WriteString("- skill_usage:\n")
	writeSortedIntMap(&builder, req.Metrics.SkillUsage)
	builder.WriteString(fmt.Sprintf("- damage_by_source.dot: %.1f\n", req.Metrics.DamageBySource.DOT))
	builder.WriteString(fmt.Sprintf("- damage_by_source.direct: %.1f\n", req.Metrics.DamageBySource.Direct))
	builder.WriteString(fmt.Sprintf("- damage_by_source.basic_attack: %.1f\n", req.Metrics.DamageBySource.BasicAttack))

	builder.WriteString("\n[Existing Diagnosis]\n")
	if len(req.Diagnosis) == 0 {
		builder.WriteString("- none\n")
	} else {
		for _, item := range req.Diagnosis {
			builder.WriteString(fmt.Sprintf("- %s (%s): %s\n", safePromptString(item.Code), safePromptString(item.Severity), safePromptString(item.Message)))
		}
	}

	return builder.String()
}

func writeFindingsSection(builder *strings.Builder, findings []rules.RuleFinding) {
	if len(findings) == 0 {
		builder.WriteString("- none\n")
		return
	}

	for _, finding := range findings {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", safePromptString(finding.Code), safePromptString(finding.Message)))
		builder.WriteString("  evidence:\n")
		if len(finding.Evidence) == 0 {
			builder.WriteString("    - none\n")
			continue
		}
		keys := sortedInterfaceMapKeys(finding.Evidence)
		for _, key := range keys {
			builder.WriteString(fmt.Sprintf("    - %s: %v\n", key, finding.Evidence[key]))
		}
	}
}

func writeRuleMetrics(builder *strings.Builder, metrics rules.RuleSummaryMetric) {
	builder.WriteString("- skill_casts:\n")
	writeSortedIntMap(builder, metrics.SkillCasts)

	builder.WriteString("- last_cast_times:\n")
	writeSortedFloatMap(builder, metrics.LastCastTimes)

	builder.WriteString(fmt.Sprintf("- dot_event_count: %d\n", metrics.DotEventCount))
	builder.WriteString("- dot_event_count_by_type:\n")
	for _, key := range []string{rules.EventTypeDotApply, rules.EventTypeDotTick, rules.EventTypeDotBurst} {
		builder.WriteString(fmt.Sprintf("  - %s: %d\n", key, metrics.DotEventCountByType[key]))
	}

	builder.WriteString("- max_cast_idle_gap:\n")
	if metrics.MaxCastIdleGap == nil {
		builder.WriteString("  - none\n")
		return
	}
	builder.WriteString(fmt.Sprintf("  - duration: %.1f\n", metrics.MaxCastIdleGap.Duration))
	builder.WriteString(fmt.Sprintf("  - start: %.1f\n", metrics.MaxCastIdleGap.Start))
	builder.WriteString(fmt.Sprintf("  - end: %.1f\n", metrics.MaxCastIdleGap.End))
}

func writeSkillUsage(builder *strings.Builder, items []model.SkillUsage) {
	if len(items) == 0 {
		builder.WriteString("  - none\n")
		return
	}

	sorted := append([]model.SkillUsage(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.TrimSpace(sorted[i].SkillID) < strings.TrimSpace(sorted[j].SkillID)
	})
	for _, item := range sorted {
		builder.WriteString(fmt.Sprintf("  - %s: %d\n", safePromptString(item.SkillID), item.Casts))
	}
}

func writeDamageBySource(builder *strings.Builder, items []model.DamageMetric) {
	if len(items) == 0 {
		builder.WriteString("  - none\n")
		return
	}

	sorted := append([]model.DamageMetric(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		left := strings.TrimSpace(sorted[i].SourceID) + "|" + strings.TrimSpace(sorted[i].Category)
		right := strings.TrimSpace(sorted[j].SourceID) + "|" + strings.TrimSpace(sorted[j].Category)
		return left < right
	})
	for _, item := range sorted {
		builder.WriteString(fmt.Sprintf("  - %s (%s): %.1f\n", safePromptString(item.SourceID), safePromptString(item.Category), item.Damage))
	}
}

func writeSortedIntMap(builder *strings.Builder, values map[string]int) {
	if len(values) == 0 {
		builder.WriteString("  - none\n")
		return
	}
	keys := sortedIntMapKeys(values)
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("  - %s: %d\n", key, values[key]))
	}
}

func writeSortedFloatMap(builder *strings.Builder, values map[string]float64) {
	if len(values) == 0 {
		builder.WriteString("  - none\n")
		return
	}
	keys := sortedFloatMapKeys(values)
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("  - %s: %.1f\n", key, values[key]))
	}
}

func sortedIntMapKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedFloatMapKeys(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedInterfaceMapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func safePromptString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "none"
	}
	return value
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}
