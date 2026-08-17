package report

import (
	"bytes"
	"strings"
	"testing"

	"eurobatt/internal/model"
)

func TestWriteCostTable(t *testing.T) {
	bds := []model.CostBreakdown{{Site: "DE", ElectricityEURkWh: 9, GasEURkWh: 1.8, LaborEURkWh: 3.6, TaxEURkWh: 0.2, SubsidyEURkWh: 5, NetCostEURkWh: 9.6, ProductionKWh: 3e7}}
	var buf bytes.Buffer
	if err := WriteCostTable(&buf, bds); err != nil {
		t.Fatalf("WriteCostTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "electricity_eur_kwh") {
		t.Fatalf("missing header: %q", out)
	}
	if !strings.Contains(out, "DE") || !strings.Contains(out, "9") {
		t.Fatalf("missing data: %q", out)
	}
}

func TestWriteNPVCurves(t *testing.T) {
	results := []model.SimResult{{Site: "DE", Scenario: model.ScenarioBaseline, YearlyMeanNPV: []float64{-1e9, 0, 1e9}}}
	var buf bytes.Buffer
	if err := WriteNPVCurves(&buf, results); err != nil {
		t.Fatalf("WriteNPVCurves: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "cumulative_npv_eur") {
		t.Fatalf("missing header: %q", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 { // header + 3 years
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
}

func TestWriteSensitivity(t *testing.T) {
	items := []model.SensitivityItem{{Parameter: "electricity_price", Swing: 1e8, Top: true}}
	var buf bytes.Buffer
	if err := WriteSensitivity(&buf, items); err != nil {
		t.Fatalf("WriteSensitivity: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "electricity_price") || !strings.Contains(out, "market_key") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestWriteSummary(t *testing.T) {
	bds := []model.CostBreakdown{{Site: "DE", NetCostEURkWh: 9.6, ElectricityEURkWh: 9}}
	results := []model.SimResult{{Site: "DE", Scenario: model.ScenarioBaseline, MeanNPV: 1e9, BreakEvenYear: 6, BreakEvenProb: 0.9, TotalSubsidy: 2e9, YearlyMeanNPV: []float64{-1e9, 1e9}}}
	sites := []model.Site{{Name: "DE", SubsidyDeadlineYear: 2030}}
	sens := map[string][]model.SensitivityItem{"DE": {{Parameter: "electricity_price", Swing: 1e8, Top: true, Normalized: 0.1}}}
	var buf bytes.Buffer
	if err := WriteSummary(&buf, bds, results, sites, sens); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Cost structure", "NPV", "Subsidy", "Sensitivity", "captured"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q: %q", want, out)
		}
	}
}
