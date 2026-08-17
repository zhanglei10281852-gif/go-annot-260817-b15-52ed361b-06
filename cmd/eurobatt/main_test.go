package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validCSV = `site,year,electricity_price,electricity_unit,gas_price,gas_unit,labor_hourly,tax_amount,subsidy_amount,subsidy_unit,capacity_util,capacity_util_unit,capacity_gwh,construction_years,subsidy_deadline_year
DE,2025,180,EUR/MWh,90,EUR/MWh,45,5000000,50,EUR/kWh,0.75,ratio,40,3,2030
DE,2026,185,EUR/MWh,92,EUR/MWh,46,5100000,50,EUR/kWh,0.76,ratio,40,3,2030
FR,2025,12,cEUR/kWh,80,EUR/MWh,42,4000000,60,EUR/kWh,75,percent,30,2,2029
FR,2026,13,cEUR/kWh,82,EUR/MWh,42,4100000,60,EUR/kWh,75,percent,30,2,2029
`

const incompleteCSV = `site,year,electricity_price,electricity_unit,gas_price,gas_unit,labor_hourly,tax_amount,subsidy_amount,subsidy_unit,capacity_util,capacity_util_unit,capacity_gwh,construction_years,subsidy_deadline_year
Bad,2025,180,EUR/MWh,90,EUR/MWh,45,5000000,50,EUR/kWh,0.75,ratio,,3,2030
`

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestRunSimulateSuccess(t *testing.T) {
	in := writeFile(t, "sites.csv", validCSV)
	out := filepath.Join(t.TempDir(), "out")
	code := run([]string{"simulate", "-i", in, "-o", out, "-iterations", "200", "-horizon", "10"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	for _, name := range []string{"cost_structure.csv", "npv_curves.csv", "sensitivity.csv", "subsidy_comparison.csv", "report.txt"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing output %s: %v", name, err)
		}
	}
}

func TestRunSimulateIncompleteFails(t *testing.T) {
	in := writeFile(t, "sites.csv", incompleteCSV)
	code := run([]string{"simulate", "-i", in, "-iterations", "100"})
	if code != 2 {
		t.Fatalf("expected exit 2 for incomplete data, got %d", code)
	}
}

func TestRunValidateOK(t *testing.T) {
	in := writeFile(t, "sites.csv", validCSV)
	code := run([]string{"validate", "-i", in})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunValidateMissingList(t *testing.T) {
	in := writeFile(t, "sites.csv", incompleteCSV)
	code := run([]string{"validate", "-i", in})
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	if code := run([]string{"frobnicate"}); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunSimulateReproducible(t *testing.T) {
	in := writeFile(t, "sites.csv", validCSV)
	out1 := filepath.Join(t.TempDir(), "out1")
	out2 := filepath.Join(t.TempDir(), "out2")
	run([]string{"simulate", "-i", in, "-o", out1, "-iterations", "200", "-horizon", "10"})
	run([]string{"simulate", "-i", in, "-o", out2, "-iterations", "200", "-horizon", "10"})
	b1, _ := os.ReadFile(filepath.Join(out1, "npv_curves.csv"))
	b2, _ := os.ReadFile(filepath.Join(out2, "npv_curves.csv"))
	if !strings.EqualFold(string(b1), string(b2)) {
		t.Fatalf("simulation not reproducible across runs")
	}
}
