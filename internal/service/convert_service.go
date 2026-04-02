package service

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"

	"github.com/feiai2017/battle_mind/internal/model"
)

// ConvertBattleReportToAnalyzeRequest 将 battle report 最小结构转换为 AnalyzeRequest。
func ConvertBattleReportToAnalyzeRequest(report model.BattleReport) model.AnalyzeRequest {
	return model.AnalyzeRequest{
		Metadata: model.AnalyzeMetadata{
			BattleType: report.FloorContext.PressureType,
			BuildTags:  buildTags(report),
			Notes:      buildNotes(report),
		},
		Summary: model.BattleSummary{
			Win:          report.ResultSummary.Win,
			Duration:     int(math.Round(report.ResultSummary.Duration)),
			LikelyReason: report.ResultSummary.LikelyReason,
		},
		Metrics: model.BattleMetrics{
			DamageBySource: aggregateDamage(report.AggregateMetrics.DamageBySource),
			SkillUsage:     mapSkillUsage(report.AggregateMetrics.SkillUsage),
		},
		Diagnosis: mapDiagnosis(report.Diagnosis),
	}
}

func buildTags(report model.BattleReport) []string {
	seen := make(map[string]struct{})
	var tags []string

	addTag := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	addTag(report.BuildContext.Archetype)
	for _, skill := range report.BuildContext.SelectedSkills {
		for _, tag := range skill.Tags {
			addTag(tag)
		}
	}

	return tags
}

func buildNotes(report model.BattleReport) string {
	var parts []string
	if id := strings.TrimSpace(report.FloorID); id != "" {
		parts = append(parts, "floor="+id)
	}
	if len(report.FloorContext.NotableRules) > 0 {
		parts = append(parts, "rules="+strings.Join(report.FloorContext.NotableRules, ","))
	}
	if len(report.FloorContext.FloorModifiers) > 0 {
		parts = append(parts, "modifiers="+strings.Join(report.FloorContext.FloorModifiers, ","))
	}
	return strings.Join(parts, "; ")
}

func aggregateDamage(items []model.DamageMetric) model.DamageBySource {
	result := model.DamageBySource{
		Other: map[string]float64{},
	}

	for _, item := range items {
		sourceID := strings.TrimSpace(item.SourceID)
		category := strings.ToLower(strings.TrimSpace(item.Category))

		switch {
		case sourceID == "basic_attack":
			result.BasicAttack += item.Damage
		case category == "dot":
			result.DOT += item.Damage
		case category == "direct":
			result.Direct += item.Damage
		default:
			key := sourceID
			if key == "" {
				key = category
			}
			if key == "" {
				key = "unknown"
			}
			result.Other[key] += item.Damage
		}
	}

	if len(result.Other) == 0 {
		result.Other = nil
	}
	return result
}

func mapSkillUsage(items []model.SkillUsage) map[string]int {
	result := make(map[string]int)
	for _, item := range items {
		skillID := strings.TrimSpace(item.SkillID)
		if skillID == "" {
			continue
		}
		result[skillID] += item.Casts
	}
	return result
}

func mapDiagnosis(items []model.RawDiagnosis) []model.DiagnosisInput {
	if len(items) == 0 {
		return nil
	}

	result := make([]model.DiagnosisInput, 0, len(items))
	for _, item := range items {
		result = append(result, model.DiagnosisInput{
			Code:     item.Code,
			Severity: item.Severity,
			Message:  item.Message,
			Details:  encodeDiagnosisDetails(item.Details),
		})
	}
	return result
}

func encodeDiagnosisDetails(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		return compact.String()
	}
	return trimmed
}
