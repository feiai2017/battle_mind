package model

// AnalyzeRequest 是分析服务的规范化输入。
// 原始复杂战斗日志到该结构的转换由外部工具负责，本服务不处理原始日志解析。
type AnalyzeRequest struct {
	Metadata  AnalyzeMetadata  `json:"metadata"`
	Summary   BattleSummary    `json:"summary"`
	Metrics   BattleMetrics    `json:"metrics"`
	Diagnosis []DiagnosisInput `json:"diagnosis"`
}

type AnalyzeMetadata struct {
	BattleType string   `json:"battle_type"`
	BuildTags  []string `json:"build_tags"`
	Notes      string   `json:"notes"`
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
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Details  string `json:"details"`
}

// AnalyzeResult 是分析服务标准输出，供上层接口直接返回。
type AnalyzeResult struct {
	Summary     string   `json:"summary"`
	Problems    []string `json:"problems"`
	Suggestions []string `json:"suggestions"`
	RawText     string   `json:"raw_text,omitempty"`
}
