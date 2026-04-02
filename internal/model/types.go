package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// AnalyzeRequest 是分析服务的规范化输入。
// 原始复杂战斗日志到该结构的转换由外部工具负责，本服务不处理原始日志解析。
type AnalyzeRequest struct {
	SchemaVersion string           `json:"schema_version,omitempty"`
	Metadata      AnalyzeMetadata  `json:"metadata"`
	Summary       BattleSummary    `json:"summary"`
	Metrics       BattleMetrics    `json:"metrics"`
	Diagnosis     []DiagnosisInput `json:"diagnosis,omitempty"`
}

type AnalyzeMetadata struct {
	BattleType     string   `json:"battle_type"`
	BattleID       string   `json:"battle_id,omitempty"`
	BuildTags      []string `json:"build_tags,omitempty"`
	FloorID        string   `json:"floor_id,omitempty"`
	NotableRules   []string `json:"notable_rules,omitempty"`
	FloorModifiers []string `json:"floor_modifiers,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

type BattleSummary struct {
	Win          bool   `json:"win"`
	Duration     int    `json:"duration"`
	LikelyReason string `json:"likely_reason"`
}

type BattleMetrics struct {
	DamageBySource DamageBySource `json:"damage_by_source"`
	SkillUsage     map[string]int `json:"skill_usage"`
}

type DamageBySource struct {
	DOT         float64            `json:"dot"`
	Direct      float64            `json:"direct"`
	BasicAttack float64            `json:"basic_attack"`
	Other       map[string]float64 `json:"other,omitempty"`
}

type DiagnosisInput struct {
	Code     string          `json:"code"`
	Severity string          `json:"severity"`
	Message  string          `json:"message"`
	Details  json.RawMessage `json:"details,omitempty"`
}

const AnalyzeRequestSchemaVersionV1 = "v1"

func (r *AnalyzeRequest) NormalizeAndValidate() error {
	if r == nil {
		return errors.New("analyze request is required")
	}

	r.SchemaVersion = strings.TrimSpace(r.SchemaVersion)
	if r.SchemaVersion == "" {
		r.SchemaVersion = AnalyzeRequestSchemaVersionV1
	}

	r.Metadata.BattleType = strings.TrimSpace(r.Metadata.BattleType)
	r.Metadata.BattleID = strings.TrimSpace(r.Metadata.BattleID)
	r.Metadata.FloorID = strings.TrimSpace(r.Metadata.FloorID)
	r.Metadata.Notes = strings.TrimSpace(r.Metadata.Notes)
	r.Metadata.BuildTags = normalizeStringSlice(r.Metadata.BuildTags)
	r.Metadata.NotableRules = normalizeStringSlice(r.Metadata.NotableRules)
	r.Metadata.FloorModifiers = normalizeStringSlice(r.Metadata.FloorModifiers)
	r.Summary.LikelyReason = strings.TrimSpace(r.Summary.LikelyReason)

	if r.Metadata.BattleType == "" {
		return errors.New("metadata.battle_type is required")
	}
	if r.Summary.Duration < 0 {
		return fmt.Errorf("summary.duration must be >= 0")
	}

	diagnosis := make([]DiagnosisInput, 0, len(r.Diagnosis))
	for i, item := range r.Diagnosis {
		item.Code = strings.TrimSpace(item.Code)
		item.Severity = strings.TrimSpace(item.Severity)
		item.Message = strings.TrimSpace(item.Message)

		if len(item.Details) > 0 {
			compacted, err := compactRawJSON(item.Details)
			if err != nil {
				return fmt.Errorf("diagnosis[%d].details is invalid: %w", i, err)
			}
			item.Details = compacted
		}

		if item.Code == "" && item.Severity == "" && item.Message == "" && len(item.Details) == 0 {
			continue
		}
		diagnosis = append(diagnosis, item)
	}
	if len(diagnosis) == 0 {
		r.Diagnosis = nil
	} else {
		r.Diagnosis = diagnosis
	}

	return nil
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func compactRawJSON(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), compact.Bytes()...), nil
}

// AnalyzeResult 是分析服务标准输出，供上层接口直接返回。
type AnalyzeResult struct {
	Summary     string   `json:"summary"`
	Problems    []string `json:"problems"`
	Suggestions []string `json:"suggestions"`
	RawText     string   `json:"raw_text,omitempty"`
}
