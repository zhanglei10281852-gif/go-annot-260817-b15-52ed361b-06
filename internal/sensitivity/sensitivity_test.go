package sensitivity

import (
	"math"
	"reflect"
	"testing"

	"eurobatt/internal/model"
)

func profSite(name string) model.Site {
	y := model.YearRecord{
		Year: 2025, ElectricityPriceEURkWh: 0.18, GasPriceEURkWh: 0.09,
		LaborHourlyEUR: 45, TaxAmountEUR: 5e6, SubsidyAmountEUR: 50 * 40 * 1e6,
		CapacityUtil: 0.75, CapacityGWh: 40,
		ElectricityRange: model.Range{Lo: 0.18, Hi: 0.18},
		SubsidyRange:     model.Range{Lo: 50 * 40 * 1e6, Hi: 50 * 40 * 1e6},
	}
	return model.Site{Name: name, ConstructionYears: 3, SubsidyDeadlineYear: 2030, CapacityGWh: 40, Years: []model.YearRecord{y}}
}

func fastCoeffs() model.Coefficients {
	c := model.DefaultCoefficients()
	c.Iterations = 300
	c.HorizonYears = 12
	return c
}

func TestAnalyzeRankingAndTopFlag(t *testing.T) {
	items := Analyze(profSite("DE"), fastCoeffs())
	if len(items) != 7 {
		t.Fatalf("expected 7 items, got %d", len(items))
	}
	topCount := 0
	for _, it := range items {
		if it.Top {
			topCount++
		}
	}
	if topCount != 3 {
		t.Fatalf("expected 3 top items, got %d", topCount)
	}
	for i := 1; i < len(items); i++ {
		if math.Abs(items[i].Swing) > math.Abs(items[i-1].Swing)+1e-6 {
			t.Fatalf("items not sorted by swing at %d: %v > %v", i, items[i].Swing, items[i-1].Swing)
		}
	}
}

func TestAnalyzeElectricityAndSubsidyDominant(t *testing.T) {
	items := Analyze(profSite("DE"), fastCoeffs())
	byName := map[string]model.SensitivityItem{}
	for _, it := range items {
		byName[it.Parameter] = it
	}
	elec := byName["electricity_price"]
	sub := byName["subsidy_amount"]
	if !elec.MarketKey || !sub.MarketKey {
		t.Fatalf("electricity and subsidy must be flagged as market keys")
	}
	// electricity and subsidy must be more sensitive than the other cost inputs
	// (gas, labor, tax), so they surface as the leading market-driven drivers.
	for _, lesser := range []string{"gas_price", "labor_hourly", "tax_amount"} {
		if math.Abs(elec.Swing) <= math.Abs(byName[lesser].Swing) {
			t.Fatalf("electricity swing %v not greater than %s swing %v", elec.Swing, lesser, byName[lesser].Swing)
		}
		if math.Abs(sub.Swing) <= math.Abs(byName[lesser].Swing) {
			t.Fatalf("subsidy swing %v not greater than %s swing %v", sub.Swing, lesser, byName[lesser].Swing)
		}
	}
}

func TestAnalyzeDoesNotMutateSite(t *testing.T) {
	s := profSite("DE")
	wantYears := append([]model.YearRecord(nil), s.Years...)
	Analyze(s, fastCoeffs())
	if !reflect.DeepEqual(s.Years, wantYears) {
		t.Fatalf("Analyze mutated the input site: got %+v want %+v", s.Years, wantYears)
	}
}
