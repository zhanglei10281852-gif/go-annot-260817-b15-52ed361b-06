# eurobatt

`eurobatt` is a local, reproducible scientific-computing tool for simulating
the site-selection cost of European power-battery (cell) capacity
investments. Given candidate plant sites (Germany, France, Spain, Italy,
...) with multi-year cost data — industrial electricity price, natural-gas
price, hourly labor cost, statutory taxes, local-government subsidies,
capacity utilization and construction period — it cleans heterogeneous
inputs, standardizes everything to **EUR per kWh of cell**, runs a
fixed-seed Monte Carlo across electricity/subsidy scenarios, and reports
cost-structure tables, NPV curves, break-even timing, subsidy-window
comparison and sensitivity rankings.

It is a pure offline tool: no network, no HTTP service, no external accounts.

## Pipeline / data flow

```
CSV input
  └─ loader.LoadCSV        parse rows, NaN for missing cells
  └─ clean.Clean           normalize units, dedup, anomaly removal, interval estimation
  └─ clean.Validate        fail the whole run with a missing-item list if any site is incomplete
  └─ cost.Breakdown        per-kWh standardized cost structure (median year)
  └─ monte.Simulate        fixed-seed Monte Carlo: optimistic/baseline/pessimistic
                            electricity & subsidy scenarios -> NPV, break-even, subsidy capture
  └─ sensitivity.Analyze   ±10% tornado ranking of NPV drivers
  └─ report.*              CSV artifacts + text summary
```

## Input format

A single CSV with one row per **site-year**. Columns (order-independent):

| column | meaning | example units |
| --- | --- | --- |
| `site` | site/country name | `DE` |
| `year` | data year | `2025` |
| `electricity_price` / `electricity_unit` | industrial power price | `EUR/MWh`, `EUR/kWh`, `cEUR/kWh`, `EUR/GJ` |
| `gas_price` / `gas_unit` | natural-gas price | `EUR/MWh`, `EUR/kWh`, `cEUR/kWh` |
| `labor_hourly` | labor cost per hour (EUR) | `45` |
| `tax_amount` | statutory taxes/fees per year (EUR total) | `5000000` |
| `subsidy_amount` / `subsidy_unit` | local subsidy | `EUR` (lump), `EUR/kWh` (per kWh of nameplate), `EUR/MWh` |
| `capacity_util` / `capacity_util_unit` | utilization | `ratio` (0..1) or `percent` (0..100) |
| `capacity_gwh` | nameplate annual capacity (GWh) | `40` |
| `construction_years` | build period | `3` |
| `subsidy_deadline_year` | subsidy application deadline | `2030` |

See `examples/sample_sites.csv` (intentionally messy: mixed units, a duplicate
row, an electricity price spike anomaly, and a missing electricity cell) and
`examples/incomplete_sites.csv` (used to demonstrate the failure/missing-list
behavior).

## Output

`eurobatt simulate` writes to the output directory (`-o`, default `out`):

- `cost_structure.csv` — per-site EUR/kWh breakdown (electricity, gas, labor, tax, subsidy, net).
- `npv_curves.csv` — yearly cumulative mean NPV per site/scenario (long format).
- `subsidy_comparison.csv` — subsidy obtainable within each site's deadline window.
- `sensitivity.csv` — NPV-driver ranking with `top` flag.
- `report.txt` — human-readable summary (also echoed to stdout).

## Commands

```bash
# full simulation
eurobatt simulate -i examples/sample_sites.csv -o out -iterations 4000

# quick reproducible run
eurobatt simulate -i examples/sample_sites.csv -o out -iterations 1000 -horizon 12

# check completeness only (prints the missing-item list on failure)
eurobatt validate -i examples/sample_sites.csv
```

Flags: `-i` (input, required), `-o` (output dir), `-iterations`,
`-seed` (default `20260817`), `-horizon` (default `15`).

## Boundary conditions & reproducibility

- **Missing electricity or subsidy** → interval estimation from the site's
  other years (or a documented default band when none exist); the Monte Carlo
  samples uniformly within the interval.
- **Single-year anomalies** (robust z-score > 3.5, requires ≥4 points) are
  removed and flagged; the value is replaced by an interval estimate.
- **Fixed seed** (`math/rand/v2` PCG, seeded per site+scenario) guarantees that
  repeated runs with identical inputs are byte-identical.
- **Any site incomplete** (no records, missing capacity / construction years /
  subsidy deadline, or no finite gas/labor at all) → the entire run fails with
  a missing-item list; backfill the listed items and rerun.

## Testing

```bash
go fmt ./...
go mod tidy
go mod verify
go build ./...
go test -timeout=120s -count=1 ./...
```

Tests cover normal paths, error paths, state transitions
(clean → validate → simulate), determinism/idempotency, and failure recovery
(incomplete data → exit 2 with missing list).

## Docker

The image is built from a pinned `golang:1.26-bookworm` builder and a `scratch`
runtime containing only the static binary; it builds for both amd64 and arm64.

```bash
# build (single-arch on the host)
docker build -t eurobatt:1.26 .

# build for amd64 + arm64 via buildx
docker buildx build --platform linux/amd64,linux/arm64 -t eurobatt:1.26 --load .

# run: mount your data and output directories
docker run --rm -v "$PWD/examples:/data" -v "$PWD/out:/out" eurobatt:1.26 \
    simulate -i /data/sample_sites.csv -o /out -iterations 4000

# validate only
docker run --rm -v "$PWD/examples:/data" eurobatt:1.26 validate -i /data/sample_sites.csv
```

## Module layout

| package | role |
| --- | --- |
| `internal/numutil` | deterministic numeric helpers (median, percentile, MAD) |
| `internal/model` | domain types, coefficients, scenarios, result containers |
| `internal/loader` | CSV parsing tolerant of missing cells |
| `internal/clean` | unit normalization, dedup, anomaly removal, interval estimation, validation |
| `internal/cost` | per-kWh standardization |
| `internal/monte` | Monte Carlo, NPV, break-even, subsidy-window capture |
| `internal/sensitivity` | tornado NPV-driver ranking |
| `internal/report` | CSV + text output |
| `cmd/eurobatt` | CLI entrypoint |
