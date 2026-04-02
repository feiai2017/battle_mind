package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feiai2017/battle_mind/internal/model"
)

const testBattleReportJSON = `{
  "floorId": "floor-01",
  "floorContext": {
    "pressureType": "burst",
    "notableRules": ["no_heal"],
    "floorModifiers": ["low_mana"]
  },
  "buildContext": {
    "archetype": "dot_mage",
    "selectedSkills": [
      { "tags": ["dot", "aoe"] },
      { "tags": ["burst"] }
    ]
  },
  "resultSummary": {
    "win": true,
    "duration": 96.4,
    "likelyReason": "good timing"
  },
  "aggregateMetrics": {
    "damageBySource": [
      { "category": "dot", "sourceId": "burn", "damage": 120.0 },
      { "category": "direct", "sourceId": "ice_lance", "damage": 80.0 },
      { "category": "direct", "sourceId": "basic_attack", "damage": 30.0 }
    ],
    "skillUsage": [
      { "skillId": "fireball", "casts": 4 }
    ]
  },
  "diagnosis": [
    {
      "code": "LOW_DOT_UPTIME",
      "severity": "medium",
      "message": "dot uptime can be improved",
      "details": { "uptime": 0.68 }
    }
  ]
}`

func TestConvertAnalyzeRequest_DefaultResponse(t *testing.T) {
	h := New()
	req := httptest.NewRequest(http.MethodPost, "/tools/convert/analyze-request", strings.NewReader(testBattleReportJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ConvertAnalyzeRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var payload struct {
		OK   bool                 `json:"ok"`
		Data model.AnalyzeRequest `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if !payload.OK {
		t.Fatalf("expected ok=true")
	}
	if payload.Data.Metadata.BattleType != "burst" {
		t.Fatalf("unexpected battle type: %s", payload.Data.Metadata.BattleType)
	}
	if payload.Data.Metrics.DamageBySource.BasicAttack != 30 {
		t.Fatalf("unexpected basic attack damage: %v", payload.Data.Metrics.DamageBySource.BasicAttack)
	}
	if payload.Data.Metrics.SkillUsage["fireball"] != 4 {
		t.Fatalf("unexpected skill usage: %d", payload.Data.Metrics.SkillUsage["fireball"])
	}
}

func TestConvertAnalyzeRequest_DownloadResponse(t *testing.T) {
	h := New()
	req := httptest.NewRequest(http.MethodPost, "/tools/convert/analyze-request?download=1", strings.NewReader(testBattleReportJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ConvertAnalyzeRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename=\"floor-01.analyze_request.json\"" {
		t.Fatalf("unexpected content-disposition: %s", got)
	}

	var payload model.AnalyzeRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if payload.Summary.Duration != 96 {
		t.Fatalf("unexpected duration: %d", payload.Summary.Duration)
	}
}
