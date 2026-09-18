package render_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Formulary-Labs/exhibit/render"
)

func TestRenderHTML_sectionsPresent(t *testing.T) {
	view := &render.AuditorView{
		Program:    "test-program",
		ReportDate: time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
		Coverage: &render.CoverageSummary{
			TotalControls: 100,
			EvidencedPct:  75,
			GapPct:        10,
		},
		Risks: &render.RiskSummary{
			Total: 5, High: 1, Open: 3,
		},
	}

	html := render.RenderHTML(view)

	for _, want := range []string{
		"Monitoring Activity",
		"Control Coverage",
		"Risk Register",
		"Evidence Calendar",
		"test-program",
		"2026-09-18",
		"75%",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing expected string: %q", want)
		}
	}
}

func TestRenderHTML_noJS(t *testing.T) {
	view := &render.AuditorView{Program: "test", ReportDate: time.Now()}
	html := render.RenderHTML(view)
	if strings.Contains(html, "<script") {
		t.Error("auditor view must not contain <script> tags")
	}
}

func TestRenderHTML_staleNotice(t *testing.T) {
	view := &render.AuditorView{
		Program:    "test",
		ReportDate: time.Now(),
		IsStale:    true,
		StaleNote:  "Data is stale — pipeline run overdue",
	}
	html := render.RenderHTML(view)
	if !strings.Contains(html, "stale-notice") {
		t.Error("expected stale notice class in HTML")
	}
	if !strings.Contains(html, "Data is stale") {
		t.Error("expected stale note text in HTML")
	}
}

func TestRenderHTML_unavailableSections(t *testing.T) {
	view := &render.AuditorView{Program: "test", ReportDate: time.Now()}
	html := render.RenderHTML(view)
	if !strings.Contains(html, "DATA UNAVAILABLE") {
		t.Error("expected DATA UNAVAILABLE for empty sections")
	}
}

func TestLoadProvenanceLog(t *testing.T) {
	entries := []render.ProvenanceEntry{
		{
			Timestamp:   "2026-09-01T10:00:00Z",
			Spec:        "functions/control-assessment-spec.md",
			OutputType:  "other",
			QualityGate: "pass",
			Purpose:     "Test entry",
			Program:     "testprog",
		},
		{
			Timestamp:   "2026-08-01T10:00:00Z", // outside 30-day window
			Spec:        "functions/control-coverage-spec.md",
			OutputType:  "other",
			QualityGate: "pass",
			Program:     "testprog",
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "provenance.jsonl")
	f, _ := os.Create(path)
	for _, e := range entries {
		line, _ := json.Marshal(e)
		f.Write(append(line, '\n')) //nolint:errcheck
	}
	f.Close()

	since := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC) // within 30 days of Sep 1
	loaded, err := render.LoadProvenanceLog(path, "testprog", since)
	if err != nil {
		t.Fatalf("LoadProvenanceLog error: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("expected 1 entry within window, got %d", len(loaded))
	}
	if loaded[0].Spec != "functions/control-assessment-spec.md" {
		t.Errorf("wrong entry returned: %q", loaded[0].Spec)
	}
}
