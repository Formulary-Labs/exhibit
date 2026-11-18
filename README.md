# exhibit

One self-contained HTML file. Opens in a browser, prints to PDF, contains no JavaScript, no external dependencies, no internal-only fields. Hand it to an auditor.

```bash
go get github.com/Formulary-Labs/exhibit
```

## In the ecosystem

Formulary is a **pick-and-choose** suite of small, single-purpose CLIs. Install only the tools your workflow needs; each runs standalone. Shared contracts live in [`substrate`](https://github.com/Formulary-Labs/substrate). Optional Make pipelines (or an agent such as `regimen`) compose tools into a run — plug-and-play, not all-or-nothing.

**This tool:** single self-contained HTML auditor view — renders, does not compute.  
**Works with:** `vital`, `titer`, `specimen`, `dose` feed the sections.  
**Pipeline stage:** present.


## What it does

`exhibit` produces a single self-contained HTML file from program compliance posture data. The four sections — monitoring activity, control coverage, risk register, and evidence calendar — are assembled from the outputs of other Formulary tools. `exhibit` renders; it does not compute.

## Input

```go
import "github.com/Formulary-Labs/exhibit/render"

html, err := render.Dashboard(render.AuditorView{
    Program:    "my-program",
    ReportDate: time.Now(),

    // From substrate/provenance JSONL, filtered for this program
    ProvenanceEntries: provenanceEntries,

    // From titer CoverageMatrix
    Coverage: &render.CoverageSummary{
        TotalControls:    114,
        EvidencedPct:     82.0,
        ImplementedPct:   88.0,
        GapPct:           12.0,
        OwnerGapCount:    3,
        EvidenceGapCount: 8,
    },

    // From specimen Register
    Risks: &render.RiskSummary{
        Total: 14, Critical: 0, High: 3,
        Medium: 7, Low: 4, Open: 9, Closed: 5,
    },

    // From dose Event list
    UpcomingEvidence: evidenceItems,
})
```

`LoadProvenanceLog(path, program, since)` is available to read provenance entries from the JSONL log by program and time window.

### CLI

```bash
exhibit --program my-isms --output out/dashboard.html
```

| Flag | Default | Description |
|---|---|---|
| `--program` | — | Program slug (required) |
| `--run-state` | `runs/[program]/latest.json` | Program run state JSON |
| `--provenance` | `logs/provenance.jsonl` | Provenance log path |
| `--lookback-days` | `90` | Provenance lookback window |
| `--report-date` | today | Report date (YYYY-MM-DD) |
| `--internal-only` | `false` | Include `internal_only` sections (auditor mode excludes them by default) |
| `--verify-provenance` | `true` | Verify the provenance log's hash-chain integrity before rendering; exits `2` on tamper detection |
| `--warn-on-provenance-failure` | `false` | Render with a warning banner instead of exiting when verification fails |

Provenance integrity: `exhibit` verifies the hash chain of the provenance ledger before assembling the report, so a tampered log fails loudly rather than rendering a trustworthy-looking dashboard from untrusted data.

## Dashboard sections

### Monitoring activity

A table of provenance log entries for this program within a configurable lookback window. Shows: timestamp, spec, output type, quality gate result, and purpose. Internal-only entries are excluded from the rendered output.

### Control coverage

Metrics grid: total controls · evidenced % · implemented % · gap % · owner gaps · evidence gaps. Sourced from `titer`.

### Risk register

Metrics grid: total · critical · high · medium · low · open · closed. Sourced from `specimen`.

### Evidence calendar

Upcoming evidence windows with status coloring:
- Red — overdue
- Amber — due within 30 days
- Green — collected

Sourced from `dose` event data or the program's evidence windows.

## Missing data handling

If any section's source data is unavailable, `exhibit` renders a `[DATA UNAVAILABLE — source: …]` note in that section. It does not fail. A partially populated dashboard is better than no dashboard.

## Output

```go
err = os.WriteFile("dashboard.html", []byte(html), 0644)
```

The HTML uses CSS variables for theming (`--bg: #f1f5f9`, dark navy header). No JavaScript. No CDN. No remote resources. All styling is inline. Print to PDF from any browser.

## Assembly pattern

The typical assembly sequence before an audit submission or quarterly review:

```bash
# 1. Coverage matrix
titer --catalog catalog.yaml --soa output/soa.csv > coverage.json

# 2. Risk register (already current from specimen)

# 3. Evidence calendar (from dose run)

# 4. Render the dashboard
exhibit --program iso42001 --coverage coverage.json --risks risks.json \
        --evidence evidence.json --provenance logs/provenance.jsonl \
        > dashboard.html
```

## License

Apache License 2.0
