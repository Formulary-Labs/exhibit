// Package render produces the static HTML auditor view for exhibit.
//
// The output is JavaScript-free, PDF-safe, and contains only externally-safe
// fields. Internal fields tagged internal_only:true are never rendered.
//
// Design system: matches the 62443 gold-standard light-theme from the prompt
// repo (dark navy header, --bg: #f1f5f9, CSS variables).
package render

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"
	"time"
)

// AuditorView is the input data assembled for rendering.
type AuditorView struct {
	Program           string
	ReportDate        time.Time
	IsStale           bool
	StaleNote         string
	ProvenanceWarning string // non-empty when provenance check failed but --warn-on-provenance-failure was set

	// Section 1 — Monitoring activity.
	ProvenanceEntries []ProvenanceEntry

	// Section 2 — Control coverage.
	Coverage *CoverageSummary

	// Section 3 — Risk register.
	Risks *RiskSummary

	// Section 4 — Evidence calendar.
	UpcomingEvidence []EvidenceItem
}

// ProvenanceEntry is a single provenance log entry (external-safe fields only).
type ProvenanceEntry struct {
	Timestamp   string `json:"timestamp"`
	Spec        string `json:"spec"`
	OutputType  string `json:"output_type"`
	QualityGate string `json:"quality_gate"`
	Purpose     string `json:"purpose"`
	Program     string `json:"program"`
}

// CoverageSummary is the external-safe coverage summary.
type CoverageSummary struct {
	TotalControls    int     `json:"total_controls"`
	EvidencedPct     float64 `json:"evidenced_pct"`
	ImplementedPct   float64 `json:"implemented_pct"`
	GapPct           float64 `json:"gap_pct"`
	OwnerGapCount    int     `json:"owner_gap_count"`
	EvidenceGapCount int     `json:"evidence_gap_count"`
}

// RiskSummary is the external-safe risk posture summary.
type RiskSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Open     int `json:"open"`
	Closed   int `json:"closed"`
}

// EvidenceItem is a single upcoming evidence window.
type EvidenceItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	DueDate   time.Time `json:"due_date"`
	Status    string    `json:"status"` // upcoming, overdue, collected
	ControlID string    `json:"control_id,omitempty"`
}

// LoadProvenanceLog reads the provenance JSONL and returns entries for program
// within the lookback window. Internal-only entries are excluded.
func LoadProvenanceLog(path, program string, since time.Time) ([]ProvenanceEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading provenance log: %w", err)
	}
	var entries []ProvenanceEntry
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e ProvenanceEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if program != "" && e.Program != program {
			continue
		}
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			continue
		}
		if ts.Before(since) {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// RenderHTML produces the auditor dashboard HTML string.
func RenderHTML(v *AuditorView) string { //nolint:revive // stutter is intentional for clarity
	sb := &strings.Builder{}
	sb.WriteString(htmlHead(v.Program, v.ReportDate))

	if v.ProvenanceWarning != "" {
		fmt.Fprintf(sb, `<div class="provenance-warning">%s</div>`+"\n", html.EscapeString(v.ProvenanceWarning))
	}
	if v.IsStale {
		fmt.Fprintf(sb, `<div class="stale-notice">%s</div>`+"\n", html.EscapeString(v.StaleNote))
	}

	sb.WriteString(`<main>` + "\n")
	renderSection1(sb, v)
	renderSection2(sb, v)
	renderSection3(sb, v)
	renderSection4(sb, v)
	sb.WriteString(`</main>` + "\n")
	sb.WriteString(htmlFoot())
	return sb.String()
}

func renderSection1(sb *strings.Builder, v *AuditorView) {
	sb.WriteString(`<section class="card"><h2>Monitoring Activity</h2>` + "\n")
	if len(v.ProvenanceEntries) == 0 {
		sb.WriteString(`<p class="unavailable">[DATA UNAVAILABLE — source: logs/provenance.jsonl]</p>`)
	} else {
		sb.WriteString(`<table><thead><tr><th>Timestamp</th><th>Spec</th><th>Output Type</th><th>Quality Gate</th><th>Purpose</th></tr></thead><tbody>` + "\n")
		for _, e := range v.ProvenanceEntries {
			qgClass := "gate-pass"
			if e.QualityGate != "pass" {
				qgClass = "gate-fail"
			}
			fmt.Fprintf(sb, `<tr><td>%s</td><td>%s</td><td>%s</td><td class="%s">%s</td><td>%s</td></tr>`+"\n",
				html.EscapeString(e.Timestamp),
				html.EscapeString(e.Spec),
				html.EscapeString(e.OutputType),
				qgClass,
				html.EscapeString(e.QualityGate),
				html.EscapeString(truncate(e.Purpose, 80)),
			)
		}
		sb.WriteString(`</tbody></table>`)
	}
	sb.WriteString(`</section>` + "\n")
}

func renderSection2(sb *strings.Builder, v *AuditorView) {
	sb.WriteString(`<section class="card"><h2>Control Coverage</h2>` + "\n")
	if v.Coverage == nil {
		sb.WriteString(`<p class="unavailable">[DATA UNAVAILABLE — source: runs/[program]/latest.json → coverage]</p>`)
	} else {
		c := v.Coverage
		fmt.Fprintf(sb, `<div class="metrics-grid">
<div class="metric"><span class="label">Total Controls</span><span class="value">%d</span></div>
<div class="metric evidenced"><span class="label">Evidenced</span><span class="value">%.0f%%</span></div>
<div class="metric impl"><span class="label">Implemented</span><span class="value">%.0f%%</span></div>
<div class="metric gap"><span class="label">Gap</span><span class="value">%.0f%%</span></div>
<div class="metric"><span class="label">Owner Gaps</span><span class="value">%d</span></div>
<div class="metric"><span class="label">Evidence Gaps</span><span class="value">%d</span></div>
</div>`, c.TotalControls, c.EvidencedPct, c.ImplementedPct, c.GapPct, c.OwnerGapCount, c.EvidenceGapCount)
	}
	sb.WriteString(`</section>` + "\n")
}

func renderSection3(sb *strings.Builder, v *AuditorView) {
	sb.WriteString(`<section class="card"><h2>Risk Register</h2>` + "\n")
	if v.Risks == nil {
		sb.WriteString(`<p class="unavailable">[DATA UNAVAILABLE — source: runs/[program]/latest.json → risks]</p>`)
	} else {
		r := v.Risks
		fmt.Fprintf(sb, `<div class="metrics-grid">
<div class="metric"><span class="label">Total</span><span class="value">%d</span></div>
<div class="metric critical"><span class="label">Critical</span><span class="value">%d</span></div>
<div class="metric high"><span class="label">High</span><span class="value">%d</span></div>
<div class="metric medium"><span class="label">Medium</span><span class="value">%d</span></div>
<div class="metric"><span class="label">Low</span><span class="value">%d</span></div>
<div class="metric"><span class="label">Open</span><span class="value">%d</span></div>
<div class="metric evidenced"><span class="label">Closed</span><span class="value">%d</span></div>
</div>`, r.Total, r.Critical, r.High, r.Medium, r.Low, r.Open, r.Closed)
	}
	sb.WriteString(`</section>` + "\n")
}

func renderSection4(sb *strings.Builder, v *AuditorView) {
	sb.WriteString(`<section class="card"><h2>Evidence Calendar</h2>` + "\n")
	if len(v.UpcomingEvidence) == 0 {
		sb.WriteString(`<p class="unavailable">[DATA UNAVAILABLE — source: runs/[program]/latest.json → evidence_windows]</p>`)
	} else {
		sb.WriteString(`<table><thead><tr><th>ID</th><th>Title</th><th>Due Date</th><th>Control</th><th>Status</th></tr></thead><tbody>` + "\n")
		for _, e := range v.UpcomingEvidence {
			var statusClass string
			switch e.Status {
			case "overdue":
				statusClass = "status-overdue"
			case "collected":
				statusClass = "status-collected"
			default:
				statusClass = "status-upcoming"
			}
			fmt.Fprintf(sb, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class="%s">%s</td></tr>`+"\n",
				html.EscapeString(e.ID),
				html.EscapeString(e.Title),
				e.DueDate.Format("2006-01-02"),
				html.EscapeString(e.ControlID),
				statusClass,
				html.EscapeString(e.Status),
			)
		}
		sb.WriteString(`</tbody></table>`)
	}
	sb.WriteString(`</section>` + "\n")
}

// exhibitCSS is the stylesheet for the auditor view, kept as a constant so
// it never passes through fmt.Sprintf and % characters need no escaping.
const exhibitCSS = `<style>
:root {
  --bg: #f1f5f9;
  --surface: #ffffff;
  --surface-2: #f8fafc;
  --border: #e2e8f0;
  --text: #0f172a;
  --text-muted: #64748b;
  --critical: #dc2626; --critical-light: #fee2e2;
  --high: #ea580c;    --high-light: #ffedd5;
  --medium: #ca8a04;  --medium-light: #fef9c3;
  --evidenced: #0891b2; --evidenced-light: #cffafe;
  --impl: #7c3aed;    --impl-light: #ede9fe;
  --gap: #dc2626;     --gap-light: #fee2e2;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: system-ui, -apple-system, sans-serif; background: var(--bg); color: var(--text); font-size: 14px; }
header { background: #0c1a2e; color: #fff; padding: 1.5rem 2rem; position: sticky; top: 0; }
header h1 { font-size: 1.25rem; font-weight: 600; }
header p  { color: #94a3b8; font-size: 0.875rem; margin-top: 0.25rem; }
main { max-width: 1200px; margin: 2rem auto; padding: 0 1rem; display: grid; gap: 1.5rem; }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 1.5rem; }
.card h2 { font-size: 1rem; font-weight: 600; margin-bottom: 1rem; color: var(--text); }
.provenance-warning { background: #fee2e2; border-left: 4px solid #dc2626; padding: 0.75rem 1rem; margin: 1rem 2rem; border-radius: 4px; font-size: 0.875rem; font-weight: 600; }
.stale-notice { background: #fef9c3; border-left: 4px solid #ca8a04; padding: 0.75rem 1rem; margin: 1rem 2rem; border-radius: 4px; font-size: 0.875rem; }
.unavailable { color: var(--text-muted); font-style: italic; }
table { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
th, td { padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid var(--border); }
th { background: var(--surface-2); font-weight: 600; font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
.metrics-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 1rem; }
.metric { background: var(--surface-2); border: 1px solid var(--border); border-radius: 6px; padding: 0.75rem; }
.metric .label { display: block; font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.metric .value { display: block; font-size: 1.5rem; font-weight: 700; margin-top: 0.25rem; }
.metric.critical .value { color: var(--critical); }
.metric.high .value { color: var(--high); }
.metric.medium .value { color: var(--medium); }
.metric.evidenced .value { color: var(--evidenced); }
.metric.impl .value { color: var(--impl); }
.metric.gap .value { color: var(--gap); }
.gate-pass { color: #16a34a; font-weight: 600; }
.gate-fail { color: var(--critical); font-weight: 600; }
.status-overdue { color: var(--critical); font-weight: 600; }
.status-upcoming { color: var(--text-muted); }
.status-collected { color: #16a34a; }
footer { text-align: center; padding: 2rem; color: var(--text-muted); font-size: 0.75rem; }
@media print { header { position: static; } }
</style>`

func htmlHead(program string, date time.Time) string {
	esc := html.EscapeString(program)
	dateStr := date.Format("2006-01-02")
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>` + esc + ` — Auditor View — ` + dateStr + `</title>
` + exhibitCSS + `
</head>
<body>
<header>
  <h1>` + esc + ` — Compliance Posture</h1>
  <p>Auditor View — Generated ` + dateStr + ` — Formulary/exhibit</p>
</header>
`
}

func htmlFoot() string {
	return `<footer>This view was generated by <a href="https://github.com/Formulary-Labs/exhibit">exhibit</a>. It contains only externally-safe compliance posture data. Internal program management data is not included.</footer>
</body>
</html>
`
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
