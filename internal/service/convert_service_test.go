package service

import (
	"encoding/json"
	"testing"

	"github.com/feiai2017/battle_mind/internal/model"
)

func TestConvertBattleReportToAnalyzeRequest(t *testing.T) {
	input := []byte(`{
		"floorId": "f1",
		"floorContext": {
			"pressureType": "burst",
			"notableRules": ["no_heal"],
			"floorModifiers": ["low_mana"]
		},
		"buildContext": {
			"archetype": "dot_mage",
			"selectedSkills": [
				{"tags": ["dot", "aoe"]},
				{"tags": ["aoe", "burst"]}
			]
		},
		"resultSummary": {
			"win": true,
			"duration": 123.6,
			"likelyReason": "stable rotation"
		},
		"aggregateMetrics": {
			"damageBySource": [
				{"category": "dot", "sourceId": "burn", "damage": 120.5},
				{"category": "direct", "sourceId": "ice_lance", "damage": 80},
				{"category": "direct", "sourceId": "basic_attack", "damage": 20},
				{"category": "reflect", "sourceId": "thorns", "damage": 5}
			],
			"skillUsage": [
				{"skillId": "fireball", "casts": 3},
				{"skillId": "fireball", "casts": 2},
				{"skillId": "ice_lance", "casts": 4}
			]
		},
		"diagnosis": [
			{
				"code": "LOW_UPTIME",
				"severity": "medium",
				"message": "dot uptime could be higher",
				"details": {"uptime": 0.72}
			}
		]
	}`)

	var report model.BattleReport
	if err := json.Unmarshal(input, &report); err != nil {
		t.Fatalf("parse battle report failed: %v", err)
	}

	out := ConvertBattleReportToAnalyzeRequest(report)

	if out.Metadata.BattleType != "burst" {
		t.Fatalf("unexpected battle type: %s", out.Metadata.BattleType)
	}
	if len(out.Metadata.BuildTags) != 4 {
		t.Fatalf("unexpected build tags length: %d", len(out.Metadata.BuildTags))
	}
	if out.Summary.Duration != 124 {
		t.Fatalf("unexpected duration: %d", out.Summary.Duration)
	}
	if out.Metrics.DamageBySource.DOT != 120.5 {
		t.Fatalf("unexpected dot damage: %v", out.Metrics.DamageBySource.DOT)
	}
	if out.Metrics.DamageBySource.Direct != 80 {
		t.Fatalf("unexpected direct damage: %v", out.Metrics.DamageBySource.Direct)
	}
	if out.Metrics.DamageBySource.BasicAttack != 20 {
		t.Fatalf("unexpected basic attack damage: %v", out.Metrics.DamageBySource.BasicAttack)
	}
	if out.Metrics.DamageBySource.Other["thorns"] != 5 {
		t.Fatalf("unexpected other damage: %v", out.Metrics.DamageBySource.Other["thorns"])
	}
	if out.Metrics.SkillUsage["fireball"] != 5 {
		t.Fatalf("unexpected fireball casts: %d", out.Metrics.SkillUsage["fireball"])
	}
	if out.Metrics.SkillUsage["ice_lance"] != 4 {
		t.Fatalf("unexpected ice_lance casts: %d", out.Metrics.SkillUsage["ice_lance"])
	}
	if out.Diagnosis[0].Details != `{"uptime":0.72}` {
		t.Fatalf("unexpected diagnosis details: %s", out.Diagnosis[0].Details)
	}
}
