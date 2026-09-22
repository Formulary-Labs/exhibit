package render

import (
	"encoding/json"
	"testing"
)

func TestFilterInternalOnly_excludesInternalFields(t *testing.T) {
	input := []byte(`{
		"program": "iso42001",
		"risks": [
			{"id": "RISK-001", "severity": "high", "internal_only": true},
			{"id": "RISK-002", "severity": "medium"}
		],
		"scope_note": "External-safe scope description",
		"internal_decision": {"text": "accept risk", "internal_only": true}
	}`)

	out, err := FilterInternalOnly(input, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// RISK-001 (internal_only: true) must be absent; RISK-002 must be present.
	risks, ok := result["risks"].([]interface{})
	if !ok {
		t.Fatalf("risks not an array in filtered output")
	}
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk after filtering, got %d", len(risks))
	}
	r, ok := risks[0].(map[string]interface{})
	if !ok {
		t.Fatal("risk entry is not a map")
	}
	if r["id"] != "RISK-002" {
		t.Errorf("expected RISK-002 to survive, got %v", r["id"])
	}

	// internal_decision object (internal_only: true) must be absent.
	if _, present := result["internal_decision"]; present {
		t.Error("internal_decision (internal_only: true) should have been stripped")
	}

	// Externally-safe fields must survive.
	if result["program"] != "iso42001" {
		t.Errorf("program field should survive filtering")
	}
	if result["scope_note"] == nil {
		t.Errorf("scope_note should survive filtering")
	}
}

func TestFilterInternalOnly_includesInternalFieldsWhenFlagSet(t *testing.T) {
	input := []byte(`{
		"program": "iso42001",
		"risks": [
			{"id": "RISK-001", "severity": "high", "internal_only": true},
			{"id": "RISK-002", "severity": "medium"}
		],
		"internal_decision": {"text": "accept risk", "internal_only": true}
	}`)

	out, err := FilterInternalOnly(input, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// Both risks must be present when includeInternal is true.
	risks, ok := result["risks"].([]interface{})
	if !ok {
		t.Fatalf("risks not an array")
	}
	if len(risks) != 2 {
		t.Fatalf("expected 2 risks when --internal-only set, got %d", len(risks))
	}

	// internal_decision must also be present.
	if _, present := result["internal_decision"]; !present {
		t.Error("internal_decision should be present when --internal-only is set")
	}
}
