package rules

const (
	EventTypeCast          = "cast"
	EventTypeSkillHit      = "skill_hit"
	EventTypeBasicAttack   = "basic_attack"
	EventTypeDotApply      = "dot_apply"
	EventTypeDotTick       = "dot_tick"
	EventTypeDotBurst      = "dot_burst"
	EventTypeBuffGain      = "buff_gain"
	EventTypeResourceGain  = "resource_gain"
	EventTypeResourceSpend = "resource_spend"
	EventTypeEnemyHit      = "enemy_hit"
	EventTypeUnknown       = "unknown"
)

// RuleEvent is the minimal normalized event view that rule logic consumes.
// The goal is to avoid coupling rules directly to the raw battle report shape.
type RuleEvent struct {
	Timestamp float64 `json:"timestamp"`
	EventType string  `json:"event_type"`
	SkillName string  `json:"skill_name"`
	Target    string  `json:"target"`
	Value     float64 `json:"value"`
}
