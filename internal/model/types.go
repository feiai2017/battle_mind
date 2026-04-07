package model

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// AnalyzeRequest 是分析服务的规范化输入。
// 原始复杂战斗日志到该结构的转换由外部工具负责，本服务不处理原始日志解析。
type AnalyzeRequest struct {
	LogText       string           `json:"log_text"`
	BattleType    string           `json:"battle_type,omitempty"`
	BuildTags     []string         `json:"build_tags,omitempty"`
	Notes         string           `json:"notes,omitempty"`
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

const (
	MaxLogTextLength  = 20000
	MaxBuildTagsCount = 10
	MaxBuildTagLength = 64
	MaxNotesLength    = 1000
)

var allowedBattleTypes = map[string]struct{}{
	"boss_pve":   {},
	"stage_pve":  {},
	"arena_pvp":  {},
	"simulation": {},
	"unknown":    {},
	"baseline":   {},
	"burst":      {},
	"swarm":      {},
}

func ValidateAnalyzeRequest(req AnalyzeRequest) *AppError {
	return req.NormalizeAndValidate()
}

func (r *AnalyzeRequest) NormalizeAndValidate() *AppError {
	if r == nil {
		return &AppError{
			Code:    ErrCodeInvalidArgument,
			Message: "analyze request is required",
		}
	}

	r.LogText = strings.TrimSpace(r.LogText)
	r.BattleType = strings.TrimSpace(r.BattleType)
	r.Notes = strings.TrimSpace(r.Notes)
	r.SchemaVersion = strings.TrimSpace(r.SchemaVersion)
	if r.SchemaVersion == "" {
		r.SchemaVersion = AnalyzeRequestSchemaVersionV1
	}

	// Reuse structured metadata as top-level compatibility fields when callers
	// send the normalized AnalyzeRequest produced by the converter.
	r.Metadata.BattleType = strings.TrimSpace(r.Metadata.BattleType)
	r.Metadata.BattleID = strings.TrimSpace(r.Metadata.BattleID)
	r.Metadata.FloorID = strings.TrimSpace(r.Metadata.FloorID)
	r.Metadata.Notes = strings.TrimSpace(r.Metadata.Notes)
	r.Metadata.BuildTags = normalizeStringSlice(r.Metadata.BuildTags)
	r.Metadata.NotableRules = normalizeStringSlice(r.Metadata.NotableRules)
	r.Metadata.FloorModifiers = normalizeStringSlice(r.Metadata.FloorModifiers)

	if r.BattleType == "" {
		r.BattleType = r.Metadata.BattleType
	}
	if len(r.BuildTags) == 0 {
		r.BuildTags = append([]string(nil), r.Metadata.BuildTags...)
	}
	if r.Notes == "" {
		r.Notes = r.Metadata.Notes
	}

	if r.LogText == "" && !hasStructuredAnalyzeInput(*r) {
		return &AppError{
			Code:    ErrCodeEmptyLogText,
			Message: "log_text or structured analyze input is required",
		}
	}
	if r.LogText != "" && utf8.RuneCountInString(r.LogText) > MaxLogTextLength {
		return &AppError{
			Code:    ErrCodeLogTooLong,
			Message: "log_text exceeds max length",
		}
	}

	if r.BattleType != "" {
		if _, ok := allowedBattleTypes[r.BattleType]; !ok {
			return &AppError{
				Code:    ErrCodeInvalidBattleType,
				Message: "battle_type is invalid",
			}
		}
	}

	if len(r.BuildTags) > 0 {
		if len(r.BuildTags) > MaxBuildTagsCount {
			return &AppError{
				Code:    ErrCodeInvalidBuildTags,
				Message: "build_tags exceeds max count",
			}
		}
		normalizedTags := make([]string, 0, len(r.BuildTags))
		for _, tag := range r.BuildTags {
			trimmed := strings.TrimSpace(tag)
			if trimmed == "" {
				return &AppError{
					Code:    ErrCodeInvalidBuildTags,
					Message: "build_tags contains empty tag",
				}
			}
			if utf8.RuneCountInString(trimmed) > MaxBuildTagLength {
				return &AppError{
					Code:    ErrCodeInvalidBuildTags,
					Message: "build_tags contains overlong tag",
				}
			}
			normalizedTags = append(normalizedTags, trimmed)
		}
		r.BuildTags = normalizedTags
	}

	if utf8.RuneCountInString(r.Notes) > MaxNotesLength {
		return &AppError{
			Code:    ErrCodeNotesTooLong,
			Message: "notes exceeds max length",
		}
	}

	r.Summary.LikelyReason = strings.TrimSpace(r.Summary.LikelyReason)

	diagnosis := make([]DiagnosisInput, 0, len(r.Diagnosis))
	for _, item := range r.Diagnosis {
		item.Code = strings.TrimSpace(item.Code)
		item.Severity = strings.TrimSpace(item.Severity)
		item.Message = strings.TrimSpace(item.Message)
		if len(item.Details) > 0 {
			compacted, err := compactRawJSON(item.Details)
			if err != nil {
				return &AppError{
					Code:    ErrCodeInvalidArgument,
					Message: "diagnosis.details contains invalid json",
				}
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

func hasStructuredAnalyzeInput(r AnalyzeRequest) bool {
	if r.Summary.Win || r.Summary.Duration > 0 || r.Summary.LikelyReason != "" {
		return true
	}
	if r.Metrics.DamageBySource.DOT != 0 ||
		r.Metrics.DamageBySource.Direct != 0 ||
		r.Metrics.DamageBySource.BasicAttack != 0 ||
		len(r.Metrics.DamageBySource.Other) > 0 ||
		len(r.Metrics.SkillUsage) > 0 {
		return true
	}
	if len(r.Diagnosis) > 0 {
		return true
	}
	return false
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
	Summary     string         `json:"summary"`
	Issues      []AnalyzeIssue `json:"issues"`
	Suggestions []string       `json:"suggestions"`
	RawText     string         `json:"raw_text,omitempty"`
}

type AnalyzeIssue struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Evidence    []string `json:"evidence"`
}
