package model

import (
	"math"
	"strings"
	"testing"
)

func TestRangeValidAndMidpoint(t *testing.T) {
	r := Range{Lo: 0.1, Hi: 0.3}
	if !r.Valid() {
		t.Fatalf("range should be valid")
	}
	if got := r.Midpoint(); got != 0.2 {
		t.Fatalf("midpoint = %v, want 0.2", got)
	}
	if (Range{Lo: 0.5, Hi: 0.2}).Valid() {
		t.Fatalf("inverted range should be invalid")
	}
	if (Range{Lo: math.NaN(), Hi: 1}).Valid() {
		t.Fatalf("NaN range should be invalid")
	}
}

func TestAllScenariosOrder(t *testing.T) {
	got := AllScenarios()
	want := []Scenario{ScenarioOptimistic, ScenarioBaseline, ScenarioPessimistic}
	if len(got) != len(want) {
		t.Fatalf("len = %d", len(got))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: %v", i, got[i])
		}
	}
}

func TestDefaultCoefficients(t *testing.T) {
	c := DefaultCoefficients()
	if c.HorizonYears <= 0 || c.Iterations <= 0 || c.Seed == 0 {
		t.Fatalf("invalid default coefficients: %+v", c)
	}
	if c.ElectricityIntensityKWhPerKWh <= 0 || c.SellingPriceEURkWh <= 0 {
		t.Fatalf("invalid physical coefficients: %+v", c)
	}
}

func TestValidationErrorFormatting(t *testing.T) {
	ve := &ValidationError{Missing: []MissingItem{
		{Site: "Germany", Field: "capacity_gwh", Reason: "absent"},
	}}
	msg := ve.Error()
	if !strings.Contains(msg, "Germany") || !strings.Contains(msg, "capacity_gwh") {
		t.Fatalf("error message missing details: %q", msg)
	}
	if !strings.Contains(msg, "incomplete") {
		t.Fatalf("error should mention incomplete: %q", msg)
	}
}

func TestScenarioTable(t *testing.T) {
	tbl := ScenarioTable()
	if len(tbl) != 3 {
		t.Fatalf("expected 3 scenarios, got %d", len(tbl))
	}
	o := tbl[ScenarioOptimistic]
	if o.ElecMean >= 1.0 || o.SubMean <= 1.0 {
		t.Fatalf("optimistic should lower elec / raise subsidy: %+v", o)
	}
	p := tbl[ScenarioPessimistic]
	if p.ElecMean <= 1.0 || p.SubMean >= 1.0 {
		t.Fatalf("pessimistic should raise elec / lower subsidy: %+v", p)
	}
}
