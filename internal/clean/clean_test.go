package clean

import (
	"math"
	"testing"

	"eurobatt/internal/model"
)

func row(site string, year int, elec float64, elecU string, gas float64, gasU string, labor, tax, sub float64, subU string, util float64, utilU string, cap float64, cons, dl int) model.RawRecord {
	return model.RawRecord{
		Site: site, Year: year,
		ElectricityPrice: elec, ElectricityUnit: elecU,
		GasPrice: gas, GasUnit: gasU,
		LaborHourly: labor, TaxAmount: tax,
		SubsidyAmount: sub, SubsidyUnit: subU,
		CapacityUtil: util, CapacityUtilUnit: utilU,
		CapacityGWh: cap, ConstructionYears: cons, SubsidyDeadlineYear: dl,
	}
}

func TestCleanUnitNormalization(t *testing.T) {
	raw := []model.RawRecord{
		row("DE", 2025, 180, "EUR/MWh", 90, "EUR/MWh", 45, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
		row("FR", 2025, 12, "cEUR/kWh", 80, "EUR/MWh", 42, 4e6, 60, "EUR/kWh", 75, "percent", 30, 2, 2029),
	}
	sites, err := Clean(raw)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	de := sites[0]
	if got := de.Years[0].ElectricityPriceEURkWh; got != 0.18 {
		t.Fatalf("DE electricity EUR/kWh = %v, want 0.18", got)
	}
	if got := de.Years[0].GasPriceEURkWh; got != 0.09 {
		t.Fatalf("DE gas EUR/kWh = %v, want 0.09", got)
	}
	if got := de.Years[0].SubsidyAmountEUR; got != 50*40*1e6 {
		t.Fatalf("DE subsidy total = %v, want %v", got, 50*40*1e6)
	}
	fr := sites[1]
	if got := fr.Years[0].ElectricityPriceEURkWh; got != 0.12 {
		t.Fatalf("FR electricity EUR/kWh = %v, want 0.12", got)
	}
	if got := fr.Years[0].CapacityUtil; got != 0.75 {
		t.Fatalf("FR capacity util = %v, want 0.75", got)
	}
}

func TestCleanSubsidyEURMWhConversion(t *testing.T) {
	raw := []model.RawRecord{
		row("NL", 2025, 120, "EUR/MWh", 70, "EUR/MWh", 40, 2e6, 25, "EUR/MWh", 0.8, "ratio", 2, 2, 2030),
	}
	sites, err := Clean(raw)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	const want = 25 * 2 * 1e3
	if got := sites[0].Years[0].SubsidyAmountEUR; got != want {
		t.Fatalf("subsidy total = %v, want %v", got, want)
	}
}

func TestCleanDedup(t *testing.T) {
	raw := []model.RawRecord{
		row("DE", 2025, 180, "EUR/MWh", 90, "EUR/MWh", 45, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
		row("DE", 2025, 180, "EUR/MWh", 90, "EUR/MWh", 45, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
		row("DE", 2026, 190, "EUR/MWh", 92, "EUR/MWh", 46, 5e6, 50, "EUR/kWh", 0.76, "ratio", 40, 3, 2030),
	}
	sites, err := Clean(raw)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(sites[0].Years) != 2 {
		t.Fatalf("expected 2 unique years, got %d", len(sites[0].Years))
	}
	found := false
	for _, f := range sites[0].Flags {
		if f == "duplicate_year_2025_removed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate removal flag, got %v", sites[0].Flags)
	}
}

func TestCleanAnomalyRemoval(t *testing.T) {
	raw := []model.RawRecord{
		row("DE", 2021, 180, "EUR/MWh", 90, "EUR/MWh", 45, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
		row("DE", 2022, 190, "EUR/MWh", 92, "EUR/MWh", 46, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
		row("DE", 2023, 170, "EUR/MWh", 88, "EUR/MWh", 44, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
		row("DE", 2024, 200, "EUR/MWh", 94, "EUR/MWh", 47, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
		row("DE", 2025, 5000, "EUR/MWh", 95, "EUR/MWh", 48, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
	}
	sites, err := Clean(raw)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	var anomalous *model.YearRecord
	for i := range sites[0].Years {
		if sites[0].Years[i].Year == 2025 {
			anomalous = &sites[0].Years[i]
		}
	}
	if anomalous == nil {
		t.Fatalf("2025 record missing")
	}
	if !anomalous.Anomaly {
		t.Fatalf("2025 electricity should be flagged anomaly")
	}
	if !anomalous.ElectricityMissing {
		t.Fatalf("anomalous electricity should be reset to missing")
	}
	// The spike (5.0 EUR/kWh) must be replaced by the interval midpoint of the
	// remaining valid years [0.17, 0.20], i.e. 0.185.
	if math.Abs(anomalous.ElectricityPriceEURkWh-0.185) > 1e-9 {
		t.Fatalf("anomalous electricity should be interval midpoint 0.185, got %v", anomalous.ElectricityPriceEURkWh)
	}
	if math.Abs(anomalous.ElectricityRange.Lo-0.17) > 1e-9 || math.Abs(anomalous.ElectricityRange.Hi-0.20) > 1e-9 {
		t.Fatalf("electricity range = %+v, want [0.17,0.20]", anomalous.ElectricityRange)
	}
}

func TestCleanIntervalEstimationMissingElectricity(t *testing.T) {
	raw := []model.RawRecord{
		row("FR", 2025, 120, "EUR/MWh", 80, "EUR/MWh", 42, 4e6, 60, "EUR/kWh", 75, "percent", 30, 2, 2029),
		row("FR", 2026, 130, "EUR/MWh", 82, "EUR/MWh", 42, 4e6, 60, "EUR/kWh", 75, "percent", 30, 2, 2029),
		row("FR", 2027, math.NaN(), "EUR/MWh", 84, "EUR/MWh", 43, 4e6, 60, "EUR/kWh", 75, "percent", 30, 2, 2029),
	}
	sites, err := Clean(raw)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	var y2027 *model.YearRecord
	for i := range sites[0].Years {
		if sites[0].Years[i].Year == 2027 {
			y2027 = &sites[0].Years[i]
		}
	}
	if y2027 == nil {
		t.Fatalf("2027 missing")
	}
	if !y2027.ElectricityMissing {
		t.Fatalf("2027 electricity should be missing")
	}
	if !y2027.ElectricityRange.Valid() {
		t.Fatalf("electricity range should be valid: %+v", y2027.ElectricityRange)
	}
	if y2027.ElectricityRange.Lo != 0.12 || y2027.ElectricityRange.Hi != 0.13 {
		t.Fatalf("electricity range = %+v, want [0.12,0.13]", y2027.ElectricityRange)
	}
}

func TestValidateIncomplete(t *testing.T) {
	raw := []model.RawRecord{
		row("Bad", 2025, 180, "EUR/MWh", 90, "EUR/MWh", 45, 5e6, 50, "EUR/kWh", 0.75, "ratio", 0, 0, 0),
	}
	sites, err := Clean(raw)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	ve := Validate(sites)
	if ve == nil {
		t.Fatalf("expected validation error for incomplete site")
	}
	fields := map[string]bool{}
	for _, m := range ve.Missing {
		fields[m.Field] = true
	}
	for _, want := range []string{"capacity_gwh", "construction_years", "subsidy_deadline_year"} {
		if !fields[want] {
			t.Fatalf("missing field %q not reported; got %v", want, ve.Missing)
		}
	}
}

func TestValidateComplete(t *testing.T) {
	raw := []model.RawRecord{
		row("DE", 2025, 180, "EUR/MWh", 90, "EUR/MWh", 45, 5e6, 50, "EUR/kWh", 0.75, "ratio", 40, 3, 2030),
	}
	sites, err := Clean(raw)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if ve := Validate(sites); ve != nil {
		t.Fatalf("complete site should validate, got %v", ve)
	}
}
