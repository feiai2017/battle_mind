package model

import "testing"

func TestAnalyzeRequestNormalizeAndValidate(t *testing.T) {
	req := AnalyzeRequest{
		Metadata: AnalyzeMetadata{
			BattleType:     "  burst  ",
			BuildTags:      []string{"dot", "dot", "  ", "burst"},
			NotableRules:   []string{"no_heal", "no_heal"},
			FloorModifiers: []string{"low_mana", "", "low_mana"},
		},
		Summary: BattleSummary{
			Duration: 123,
		},
		Diagnosis: []DiagnosisInput{
			{
				Code:    " LOW_DOT ",
				Details: []byte("{\n  \"uptime\": 0.72\n}"),
			},
			{},
		},
	}

	if err := req.NormalizeAndValidate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	if req.SchemaVersion != AnalyzeRequestSchemaVersionV1 {
		t.Fatalf("unexpected schema version: %s", req.SchemaVersion)
	}
	if req.Metadata.BattleType != "burst" {
		t.Fatalf("unexpected battle type: %q", req.Metadata.BattleType)
	}
	if len(req.Metadata.BuildTags) != 2 {
		t.Fatalf("unexpected build tags: %#v", req.Metadata.BuildTags)
	}
	if string(req.Diagnosis[0].Details) != `{"uptime":0.72}` {
		t.Fatalf("unexpected diagnosis details: %s", string(req.Diagnosis[0].Details))
	}
	if len(req.Diagnosis) != 1 {
		t.Fatalf("unexpected diagnosis count: %d", len(req.Diagnosis))
	}
}

func TestAnalyzeRequestNormalizeAndValidate_RequiresBattleType(t *testing.T) {
	req := AnalyzeRequest{
		Summary: BattleSummary{
			Duration: 1,
		},
	}

	if err := req.NormalizeAndValidate(); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestAnalyzeRequestNormalizeAndValidate_RejectsNegativeDuration(t *testing.T) {
	req := AnalyzeRequest{
		Metadata: AnalyzeMetadata{
			BattleType: "burst",
		},
		Summary: BattleSummary{
			Duration: -1,
		},
	}

	if err := req.NormalizeAndValidate(); err == nil {
		t.Fatalf("expected validation error")
	}
}
