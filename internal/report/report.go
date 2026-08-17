// Package report serializes simulation outputs into CSV artifacts and a
// human-readable text summary: the per-site cost-structure table, NPV curves,
// subsidy comparison within the application-deadline window, and the
// sensitivity ranking with the top drivers flagged.
package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"eurobatt/internal/model"
)

func f(v float64) string {
	return strconv.FormatFloat(v, 'g', 6, 64)
}

// WriteCostTable writes the per-kWh cost structure as CSV.
func WriteCostTable(w io.Writer, bds []model.CostBreakdown) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{
		"site", "electricity_eur_kwh", "gas_eur_kwh", "labor_eur_kwh",
		"tax_eur_kwh", "subsidy_eur_kwh", "net_cost_eur_kwh", "annual_production_kwh",
	}); err != nil {
		return err
	}
	for _, b := range bds {
		if err := cw.Write([]string{
			b.Site, f(b.ElectricityEURkWh), f(b.GasEURkWh), f(b.LaborEURkWh),
			f(b.TaxEURkWh), f(b.SubsidyEURkWh), f(b.NetCostEURkWh), f(b.ProductionKWh),
		}); err != nil {
			return err
		}
	}
	return nil
}

// WriteNPVCurves writes yearly cumulative mean NPV per site/scenario in long
// format: site,scenario,year_offset,cumulative_npv_eur.
func WriteNPVCurves(w io.Writer, results []model.SimResult) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"site", "scenario", "year_offset", "cumulative_npv_eur"}); err != nil {
		return err
	}
	for _, r := range results {
		for t, v := range r.YearlyMeanNPV {
			if err := cw.Write([]string{r.Site, string(r.Scenario), strconv.Itoa(t), f(v)}); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteSensitivity writes the sensitivity ranking as CSV.
func WriteSensitivity(w io.Writer, items []model.SensitivityItem) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"parameter", "base_npv_eur", "low_npv_eur", "high_npv_eur", "swing_eur", "normalized", "top", "market_key"}); err != nil {
		return err
	}
	for _, it := range items {
		top := "false"
		if it.Top {
			top = "true"
		}
		mk := "false"
		if it.MarketKey {
			mk = "true"
		}
		if err := cw.Write([]string{it.Parameter, f(it.BaseNPV), f(it.LowNPV), f(it.HighNPV), f(it.Swing), f(it.Normalized), top, mk}); err != nil {
			return err
		}
	}
	return nil
}

// WriteSubsidyComparison writes the subsidy obtainable within each site's
// application-deadline window (baseline scenario) as CSV.
func WriteSubsidyComparison(w io.Writer, results []model.SimResult, sites []model.Site) error {
	deadline := make(map[string]int, len(sites))
	for _, s := range sites {
		deadline[s.Name] = s.SubsidyDeadlineYear
	}
	baseline := make(map[string]model.SimResult, len(sites))
	for _, r := range results {
		if r.Scenario == model.ScenarioBaseline {
			baseline[r.Site] = r
		}
	}
	type row struct {
		site     string
		deadline int
		sub      float64
		captured bool
	}
	rows := make([]row, 0, len(sites))
	for _, s := range sites {
		r := baseline[s.Name]
		rows = append(rows, row{s.Name, deadline[s.Name], r.TotalSubsidy, r.TotalSubsidy > 0})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].sub > rows[j].sub })
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"site", "subsidy_deadline_year", "baseline_total_subsidy_eur", "captured_in_window"}); err != nil {
		return err
	}
	for _, r := range rows {
		cap := "false"
		if r.captured {
			cap = "true"
		}
		if err := cw.Write([]string{r.site, strconv.Itoa(r.deadline), f(r.sub), cap}); err != nil {
			return err
		}
	}
	return nil
}

// WriteSummary writes a human-readable text report combining all sections.
func WriteSummary(w io.Writer, bds []model.CostBreakdown, results []model.SimResult, sites []model.Site, sensitivity map[string][]model.SensitivityItem) error {
	var b strings.Builder
	b.WriteString("Eurobatt site-selection cost simulation\n")
	b.WriteString("=========================================\n\n")

	b.WriteString("Cost structure (EUR per kWh cell, median year):\n")
	b.WriteString("  site            elec    gas   labor    tax   subsid    net\n")
	for _, c := range bds {
		fmt.Fprintf(&b, "  %-14s %6.2f %6.2f %6.2f %6.2f %8.2f %6.2f\n",
			c.Site, c.ElectricityEURkWh, c.GasEURkWh, c.LaborEURkWh, c.TaxEURkWh, c.SubsidyEURkWh, c.NetCostEURkWh)
	}
	b.WriteString("\n")

	b.WriteString("NPV / break-even (mean over Monte Carlo):\n")
	b.WriteString("  site            scenario      meanNPV        P5       P50       P95  breakEven  prob\n")
	for _, r := range results {
		fmt.Fprintf(&b, "  %-14s %-11s %10s %9s %9s %9s %8.1f %5.0f%%\n",
			r.Site, r.Scenario, formatEUR(r.MeanNPV), formatEUR(r.P5), formatEUR(r.P50), formatEUR(r.P95),
			r.BreakEvenYear, r.BreakEvenProb*100)
	}
	b.WriteString("\n")

	b.WriteString("Subsidy obtainable within application-deadline window (baseline):\n")
	deadline := make(map[string]int, len(sites))
	for _, s := range sites {
		deadline[s.Name] = s.SubsidyDeadlineYear
	}
	for _, s := range sites {
		var r model.SimResult
		for _, rr := range results {
			if rr.Site == s.Name && rr.Scenario == model.ScenarioBaseline {
				r = rr
				break
			}
		}
		status := "captured"
		if r.TotalSubsidy <= 0 {
			status = "forfeited"
		}
		fmt.Fprintf(&b, "  %-14s deadline=%d  %s  (%s)\n", s.Name, deadline[s.Name], formatEUR(r.TotalSubsidy), status)
	}
	b.WriteString("\n")

	b.WriteString("Sensitivity ranking (* = top swing driver, † = electricity/subsidy market key):\n")
	for siteName, items := range sensitivity {
		fmt.Fprintf(&b, "  [%s]\n", siteName)
		for _, it := range items {
			mark := "  "
			if it.Top {
				mark = "* "
			}
			mk := ""
			if it.MarketKey {
				mk = " †"
			}
			fmt.Fprintf(&b, "    %s%-22s swing=%s (%.1f%%)%s\n", mark, it.Parameter, formatEUR(it.Swing), it.Normalized*100, mk)
		}
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func formatEUR(v float64) string {
	switch {
	case math.IsNaN(v):
		return "n/a"
	case math.Abs(v) >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case math.Abs(v) >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	case math.Abs(v) >= 1e3:
		return fmt.Sprintf("%.1fk", v/1e3)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}
