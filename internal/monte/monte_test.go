package monte

import (
	"math"
	"testing"

	"eurobatt/internal/model"
)

func profitableSite(name string, deadline int) model.Site {
	y := model.YearRecord{
		Year: 2025, ElectricityPriceEURkWh: 0.18, GasPriceEURkWh: 0.09,
		LaborHourlyEUR: 45, TaxAmountEUR: 5e6, SubsidyAmountEUR: 50 * 40 * 1e6,
		CapacityUtil: 0.75, CapacityGWh: 40,
		ElectricityRange: model.Range{Lo: 0.18, Hi: 0.18},
		SubsidyRange:     model.Range{Lo: 50 * 40 * 1e6, Hi: 50 * 40 * 1e6},
	}
	return model.Site{Name: name, ConstructionYears: 3, SubsidyDeadlineYear: deadline, CapacityGWh: 40, Years: []model.YearRecord{y}}
}

func fastCoeffs() model.Coefficients {
	c := model.DefaultCoefficients()
	c.Iterations = 600
	c.HorizonYears = 12
	return c
}

func TestSimulateDeterminism(t *testing.T) {
	c := fastCoeffs()
	s := profitableSite("DE", 2030)
	r1, err := Simulate([]model.Site{s}, c)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	r2, _ := Simulate([]model.Site{s}, c)
	for i := range r1 {
		if r1[i].MeanNPV != r2[i].MeanNPV {
			t.Fatalf("non-deterministic NPV: %v vs %v", r1[i].MeanNPV, r2[i].MeanNPV)
		}
		if len(r1[i].YearlyMeanNPV) != len(r2[i].YearlyMeanNPV) {
			t.Fatalf("yearly length mismatch")
		}
		for k := range r1[i].YearlyMeanNPV {
			if r1[i].YearlyMeanNPV[k] != r2[i].YearlyMeanNPV[k] {
				t.Fatalf("non-deterministic yearly NPV at %d", k)
			}
		}
	}
}

func TestSimulateScenarioOrdering(t *testing.T) {
	c := fastCoeffs()
	s := profitableSite("DE", 2030)
	res, _ := Simulate([]model.Site{s}, c)
	byName := map[model.Scenario]model.SimResult{}
	for _, r := range res {
		byName[r.Scenario] = r
	}
	if !(byName[model.ScenarioOptimistic].MeanNPV > byName[model.ScenarioBaseline].MeanNPV &&
		byName[model.ScenarioBaseline].MeanNPV > byName[model.ScenarioPessimistic].MeanNPV) {
		t.Fatalf("scenario ordering wrong: opt=%.3g base=%.3g pess=%.3g",
			byName[model.ScenarioOptimistic].MeanNPV, byName[model.ScenarioBaseline].MeanNPV, byName[model.ScenarioPessimistic].MeanNPV)
	}
}

func TestBreakEvenAchieved(t *testing.T) {
	c := fastCoeffs()
	s := profitableSite("DE", 2030)
	res, _ := Simulate([]model.Site{s}, c)
	for _, r := range res {
		if r.Scenario != model.ScenarioBaseline {
			continue
		}
		if r.BreakEvenYear < 0 {
			t.Fatalf("expected break-even within horizon, got none")
		}
		if r.BreakEvenProb <= 0 {
			t.Fatalf("expected positive break-even probability")
		}
	}
}

func TestSubsidyForfeitedAfterDeadline(t *testing.T) {
	c := fastCoeffs()
	s := profitableSite("DE", 2026)
	res, _ := Simulate([]model.Site{s}, c)
	for _, r := range res {
		if r.Scenario != model.ScenarioBaseline {
			continue
		}
		if math.Abs(r.TotalSubsidy) > 1e-6 {
			t.Fatalf("subsidy should be forfeited, got %v", r.TotalSubsidy)
		}
	}
}

func TestSubsidyCapturedWhenOperationsStartOnDeadline(t *testing.T) {
	c := fastCoeffs()
	s := profitableSite("DE", 2028)
	res, err := Simulate([]model.Site{s}, c)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	for _, r := range res {
		if r.Scenario == model.ScenarioBaseline && r.TotalSubsidy <= 0 {
			t.Fatalf("subsidy should be captured in the deadline year, got %v", r.TotalSubsidy)
		}
	}
}

func TestSitesUseIndependentRandomStreams(t *testing.T) {
	c := fastCoeffs()
	res, err := Simulate([]model.Site{
		profitableSite("Alpha", 2030),
		profitableSite("Beta", 2030),
	}, c)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	var alpha, beta *model.SimResult
	for i := range res {
		if res[i].Scenario != model.ScenarioBaseline {
			continue
		}
		switch res[i].Site {
		case "Alpha":
			alpha = &res[i]
		case "Beta":
			beta = &res[i]
		}
	}
	if alpha == nil || beta == nil {
		t.Fatalf("missing baseline results: alpha=%v beta=%v", alpha, beta)
	}
	if alpha.MeanNPV == beta.MeanNPV {
		t.Fatalf("distinct sites unexpectedly produced identical baseline samples: %v", alpha.MeanNPV)
	}
}

func TestRecordIndexForYearNearest(t *testing.T) {
	s := model.Site{Years: []model.YearRecord{{Year: 2025}, {Year: 2027}, {Year: 2030}}}
	if got := recordIndexForYear(s, 2026); got != 0 {
		t.Fatalf("2026 -> index %d, want 0 (tie to earlier)", got)
	}
	if got := recordIndexForYear(s, 2028); got != 1 {
		t.Fatalf("2028 -> index %d, want 1", got)
	}
	if got := recordIndexForYear(s, 2099); got != 2 {
		t.Fatalf("2099 -> index %d, want 2", got)
	}
}
