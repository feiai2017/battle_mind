package model

import "encoding/json"

// BattleReport is the minimal structured input used by converters and rules.
// It intentionally keeps only the fields the service needs to consume.
type BattleReport struct {
	FloorID          string           `json:"floorId"`
	FloorContext     FloorContext     `json:"floorContext"`
	BuildContext     BuildContext     `json:"buildContext"`
	ResultSummary    ResultSummary    `json:"resultSummary"`
	AggregateMetrics AggregateMetrics `json:"aggregateMetrics"`
	Events           []ReportEvent    `json:"events"`
	Diagnosis        []RawDiagnosis   `json:"diagnosis"`
}

type FloorContext struct {
	PressureType   string   `json:"pressureType"`
	NotableRules   []string `json:"notableRules"`
	FloorModifiers []string `json:"floorModifiers"`
}

type BuildContext struct {
	Archetype      string          `json:"archetype"`
	SelectedSkills []SelectedSkill `json:"selectedSkills"`
}

type SelectedSkill struct {
	Tags []string `json:"tags"`
}

type ResultSummary struct {
	Win          bool    `json:"win"`
	Duration     float64 `json:"duration"`
	LikelyReason string  `json:"likelyReason"`
}

type AggregateMetrics struct {
	DamageBySource []DamageMetric `json:"damageBySource"`
	SkillUsage     []SkillUsage   `json:"skillUsage"`
}

type DamageMetric struct {
	Category string  `json:"category"`
	SourceID string  `json:"sourceId"`
	Damage   float64 `json:"damage"`
}

type SkillUsage struct {
	SkillID string `json:"skillId"`
	Casts   int    `json:"casts"`
}

type ReportEvent struct {
	Time       float64         `json:"time"`
	Category   string          `json:"category"`
	Type       string          `json:"type"`
	SourceID   *string         `json:"sourceId"`
	SourceName *string         `json:"sourceName"`
	TargetID   json.RawMessage `json:"targetId"`
	TargetName *string         `json:"targetName"`
	Amount     *float64        `json:"amount"`
	Tags       []string        `json:"tags"`
	Metadata   json.RawMessage `json:"metadata"`
}

type RawDiagnosis struct {
	Code     string          `json:"code"`
	Severity string          `json:"severity"`
	Message  string          `json:"message"`
	Details  json.RawMessage `json:"details"`
}
