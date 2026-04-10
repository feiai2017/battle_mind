package output

import (
	"encoding/json"
	"strings"

	"github.com/feiai2017/battle_mind/internal/rules"
)

type AnalyzeOutput struct {
	RuleFindings     RuleFindingsOutput   `json:"rule_findings"`
	ModelSuggestions ModelSuggestionBlock `json:"model_suggestions"`
	RawText          string               `json:"raw_text,omitempty"`
}

type RuleFindingsOutput struct {
	Findings []rules.RuleFinding     `json:"findings"`
	Metrics  rules.RuleSummaryMetric `json:"metrics"`
}

type ModelSuggestionBlock struct {
	Summary     string   `json:"summary"`
	Suggestions []string `json:"suggestions,omitempty"`
	Risks       []string `json:"risks,omitempty"`
}

func BuildRuleFindingsOutput(summary rules.RuleSummary) RuleFindingsOutput {
	return RuleFindingsOutput{
		Findings: cloneRuleFindings(summary.HardFindings),
		Metrics:  cloneRuleSummaryMetric(summary.Metrics),
	}
}

func BuildAnalyzeOutput(summary rules.RuleSummary, suggestions ModelSuggestionBlock) AnalyzeOutput {
	return AnalyzeOutput{
		RuleFindings:     BuildRuleFindingsOutput(summary),
		ModelSuggestions: normalizeModelSuggestionBlock(suggestions),
	}
}

func normalizeModelSuggestionBlock(block ModelSuggestionBlock) ModelSuggestionBlock {
	block.Summary = strings.TrimSpace(block.Summary)
	block.Suggestions = normalizeStringSlice(block.Suggestions)
	block.Risks = normalizeStringSlice(block.Risks)

	if block.Suggestions == nil {
		block.Suggestions = []string{}
	}
	if block.Risks == nil {
		block.Risks = []string{}
	}
	return block
}

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneRuleFindings(items []rules.RuleFinding) []rules.RuleFinding {
	if len(items) == 0 {
		return []rules.RuleFinding{}
	}

	result := make([]rules.RuleFinding, 0, len(items))
	for _, item := range items {
		result = append(result, rules.RuleFinding{
			Code:     item.Code,
			Severity: item.Severity,
			Message:  item.Message,
			Evidence: cloneInterfaceMap(item.Evidence),
		})
	}
	return result
}

func cloneRuleSummaryMetric(metric rules.RuleSummaryMetric) rules.RuleSummaryMetric {
	cloned := rules.RuleSummaryMetric{
		SkillCasts:          cloneIntMap(metric.SkillCasts),
		LastCastTimes:       cloneFloatMap(metric.LastCastTimes),
		DotEventCount:       metric.DotEventCount,
		DotEventCountByType: cloneIntMap(metric.DotEventCountByType),
	}
	if metric.MaxCastIdleGap != nil {
		gapCopy := *metric.MaxCastIdleGap
		cloned.MaxCastIdleGap = &gapCopy
	}
	if cloned.SkillCasts == nil {
		cloned.SkillCasts = map[string]int{}
	}
	if cloned.LastCastTimes == nil {
		cloned.LastCastTimes = map[string]float64{}
	}
	if cloned.DotEventCountByType == nil {
		cloned.DotEventCountByType = map[string]int{}
	}
	return cloned
}

func cloneIntMap(values map[string]int) map[string]int {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]int, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneFloatMap(values map[string]float64) map[string]float64 {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]float64, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneInterfaceMap(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}

	raw, err := json.Marshal(values)
	if err != nil {
		cloned := make(map[string]interface{}, len(values))
		for key, value := range values {
			cloned[key] = value
		}
		return cloned
	}

	var cloned map[string]interface{}
	if err := json.Unmarshal(raw, &cloned); err != nil {
		fallback := make(map[string]interface{}, len(values))
		for key, value := range values {
			fallback[key] = value
		}
		return fallback
	}
	return cloned
}
