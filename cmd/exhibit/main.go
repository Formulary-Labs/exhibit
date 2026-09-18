// exhibit generates a read-only, JavaScript-free, PDF-safe auditor compliance
// posture dashboard from a program run state JSON and provenance log.
//
// Usage:
//
//	exhibit --program <slug> [flags]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Formulary-Labs/exhibit/render"
	"github.com/Formulary-Labs/substrate/exit"
	"github.com/Formulary-Labs/substrate/provenance"
)

const version = "0.1.0"

func main() {
	var (
		programFlag      = flag.String("program", "", "Program slug (required)")
		runStateFlag     = flag.String("run-state", "", "Path to run state JSON (default: runs/[program]/latest.json)")
		provenanceFlag   = flag.String("provenance", "logs/provenance.jsonl", "Path to provenance log")
		lookbackFlag     = flag.Int("lookback-days", 90, "Provenance lookback window in days")
		outputFlag       = flag.String("output", "", "Output HTML path (default: stdout)")
		reportDateFlag   = flag.String("report-date", "", "Report date YYYY-MM-DD (default: today)")
		versionFlag      = flag.Bool("version", false, "Print version and exit")
	)
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Printf("exhibit version %s\n", version)
		os.Exit(exit.OK)
	}

	if *programFlag == "" {
		fmt.Fprintln(os.Stderr, `{"error": "--program is required", "code": 2}`)
		flag.Usage()
		os.Exit(exit.ToolError)
	}

	reportDate := time.Now().UTC()
	if *reportDateFlag != "" {
		var err error
		reportDate, err = time.Parse("2006-01-02", *reportDateFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, `{"error": "invalid report-date format: %v", "code": 2}`+"\n", err)
			os.Exit(exit.ToolError)
		}
	}

	runStatePath := *runStateFlag
	if runStatePath == "" {
		runStatePath = filepath.Join("runs", *programFlag, "latest.json")
	}

	view := &render.AuditorView{
		Program:    *programFlag,
		ReportDate: reportDate,
	}

	// Load run state.
	rs, err := loadRunState(runStatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load run state: %v\n", err)
	} else {
		populateFromRunState(view, rs, reportDate)
	}

	// Load provenance.
	since := reportDate.Add(-time.Duration(*lookbackFlag) * 24 * time.Hour)
	entries, err := render.LoadProvenanceLog(*provenanceFlag, *programFlag, since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load provenance: %v\n", err)
	}
	view.ProvenanceEntries = entries

	html := render.RenderHTML(view)

	if *outputFlag == "" {
		fmt.Print(html)
	} else {
		if err := os.WriteFile(*outputFlag, []byte(html), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
			os.Exit(exit.ToolError)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *outputFlag)
	}

	_ = provenance.Write("logs/provenance.jsonl", provenance.Entry{
		Spec:        "functions/auditor-view-spec.md",
		Output:      coalesceStr(*outputFlag, "stdout"),
		OutputType:  "other",
		Program:     *programFlag,
		Purpose:     fmt.Sprintf("exhibit: auditor view for %s, %d provenance entries, date %s", *programFlag, len(entries), reportDate.Format("2006-01-02")),
		Reusability: provenance.Instance,
		QualityGate: provenance.Pass,
		Tool:        "exhibit",
		ToolVersion: version,
	})
}

// rawRunState is a minimal struct for reading the run state JSON.
type rawRunState struct {
	Program    string     `json:"program"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	RunDate    *time.Time `json:"run_date,omitempty"`
	RecommendedNextRun *time.Time `json:"recommended_next_run,omitempty"`

	Coverage *struct {
		TotalControls    int     `json:"total_controls,omitempty"`
		EvidencedPct     float64 `json:"evidenced_pct,omitempty"`
		ImplementedPct   float64 `json:"implemented_pct,omitempty"`
		GapPct           float64 `json:"gap_pct,omitempty"`
		OwnerGapCount    int     `json:"owner_gap_count,omitempty"`
		EvidenceGapCount int     `json:"evidence_gap_count,omitempty"`
	} `json:"coverage,omitempty"`

	Risks []struct {
		Severity string `json:"severity"`
		Status   string `json:"status"`
	} `json:"risks,omitempty"`

	EvidenceWindows []struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		DueDate   time.Time `json:"due_date"`
		Status    string    `json:"status"`
		ControlID string    `json:"control_id,omitempty"`
	} `json:"evidence_windows,omitempty"`
}

func loadRunState(path string) (*rawRunState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rs rawRunState
	return &rs, json.Unmarshal(data, &rs)
}

func populateFromRunState(view *render.AuditorView, rs *rawRunState, now time.Time) {
	// Staleness check.
	if rs.RecommendedNextRun != nil && now.After(*rs.RecommendedNextRun) {
		view.IsStale = true
		refDate := rs.RunDate
		if refDate == nil {
			refDate = rs.UpdatedAt
		}
		dateStr := "unknown"
		if refDate != nil {
			dateStr = refDate.Format("2006-01-02")
		}
		view.StaleNote = fmt.Sprintf("Note: This view was generated from program data last updated %s. A pipeline run is overdue as of %s. Data may not reflect current program state.",
			dateStr, rs.RecommendedNextRun.Format("2006-01-02"))
	}

	// Coverage.
	if rs.Coverage != nil {
		view.Coverage = &render.CoverageSummary{
			TotalControls:    rs.Coverage.TotalControls,
			EvidencedPct:     rs.Coverage.EvidencedPct,
			ImplementedPct:   rs.Coverage.ImplementedPct,
			GapPct:           rs.Coverage.GapPct,
			OwnerGapCount:    rs.Coverage.OwnerGapCount,
			EvidenceGapCount: rs.Coverage.EvidenceGapCount,
		}
	}

	// Risks — external-safe: counts by severity only.
	if len(rs.Risks) > 0 {
		summary := &render.RiskSummary{}
		for _, r := range rs.Risks {
			summary.Total++
			switch r.Severity {
			case "critical":
				summary.Critical++
			case "high":
				summary.High++
			case "medium":
				summary.Medium++
			case "low":
				summary.Low++
			}
			switch r.Status {
			case "open", "accepted":
				summary.Open++
			case "mitigated", "closed":
				summary.Closed++
			}
		}
		view.Risks = summary
	}

	// Evidence windows.
	for _, w := range rs.EvidenceWindows {
		view.UpcomingEvidence = append(view.UpcomingEvidence, render.EvidenceItem{
			ID:        w.ID,
			Title:     w.Title,
			DueDate:   w.DueDate,
			Status:    w.Status,
			ControlID: w.ControlID,
		})
	}
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func usage() {
	fmt.Fprintln(os.Stderr, `exhibit — auditor-filtered compliance posture view

Usage:
  exhibit --program <slug> [flags]

Flags:
  --program string        Program slug (required)
  --run-state string      Path to run state JSON (default: runs/[program]/latest.json)
  --provenance string     Path to provenance log (default: logs/provenance.jsonl)
  --lookback-days int     Provenance lookback window in days (default: 90)
  --output string         Output HTML path (default: stdout)
  --report-date string    Report date YYYY-MM-DD (default: today)
  --version               Print version and exit

Examples:
  exhibit --program iso42001 --output exhibit.html
  exhibit --program iso42001 --lookback-days 180 > exhibit.html`)
}
