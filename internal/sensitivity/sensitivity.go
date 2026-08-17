// Package sensitivity ranks the effect of each site cost input on the
// baseline-scenario NPV using a one-at-a-time perturbation (tornado
// analysis). Continuous inputs are perturbed ±10%; the integer construction
// period is perturbed ±1 year. Results are reproducible because the
// underlying Monte Carlo uses the fixed coefficient seed.
//
// Only the business cost inputs defined by the input model are ranked
// (electricity price, subsidy, capacity utilization, gas price, labor, tax,
// construction period), so electricity and subsidy surface as the leading
// market-driven sensitivities rather than being masked by financial
// assumptions such as selling price or capex.
package sensitivity

import (
	"math"
	"sort"

	"eurobatt/internal/model"
	"eurobatt/internal/monte"
	"eurobatt/internal/numutil"
)

const delta = 0.10

type perturber struct {
	name  string
	apply func(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients)
}

var perturbers = []perturber{
	{"electricity_price", perturbElec},
	{"subsidy_amount", perturbSubsidy},
	{"capacity_utilization", perturbUtil},
	{"gas_price", perturbGas},
	{"labor_hourly", perturbLabor},
	{"tax_amount", perturbTax},
	{"construction_years", perturbConstruction},
}

// Analyze returns sensitivity items sorted by absolute NPV swing descending.
// The top three items are flagged Top=true.
func Analyze(s model.Site, c model.Coefficients) []model.SensitivityItem {
	base := baselineNPV(s, c)
	items := make([]model.SensitivityItem, 0, len(perturbers))
	for _, p := range perturbers {
		sLo, cLo := p.apply(cloneSite(s), c, 1-delta)
		sHi, cHi := p.apply(cloneSite(s), c, 1+delta)
		lo := baselineNPV(sLo, cLo)
		hi := baselineNPV(sHi, cHi)
		swing := hi - lo
		norm := 0.0
		if math.Abs(base) > 1e-9 {
			norm = swing / math.Abs(base)
		}
		items = append(items, model.SensitivityItem{
			Parameter:  p.name,
			BaseNPV:    base,
			LowNPV:     lo,
			HighNPV:    hi,
			Swing:      swing,
			Normalized: norm,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return math.Abs(items[i].Swing) > math.Abs(items[j].Swing)
	})
	for i := 0; i < 3 && i < len(items); i++ {
		items[i].Top = true
	}
	for i := range items {
		if items[i].Parameter == "electricity_price" || items[i].Parameter == "subsidy_amount" {
			items[i].MarketKey = true
		}
	}
	return items
}

func baselineNPV(s model.Site, c model.Coefficients) float64 {
	res, err := monte.Simulate([]model.Site{s}, c)
	if err != nil {
		return math.NaN()
	}
	for _, r := range res {
		if r.Scenario == model.ScenarioBaseline {
			return r.MeanNPV
		}
	}
	return math.NaN()
}

func cloneSite(s model.Site) model.Site {
	s.Years = append([]model.YearRecord(nil), s.Years...)
	return s
}

func perturbElec(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients) {
	for i := range s.Years {
		s.Years[i].ElectricityPriceEURkWh *= f
		s.Years[i].ElectricityRange.Lo *= f
		s.Years[i].ElectricityRange.Hi *= f
	}
	return s, c
}

func perturbGas(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients) {
	for i := range s.Years {
		if !math.IsNaN(s.Years[i].GasPriceEURkWh) {
			s.Years[i].GasPriceEURkWh *= f
		}
	}
	return s, c
}

func perturbLabor(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients) {
	for i := range s.Years {
		if !math.IsNaN(s.Years[i].LaborHourlyEUR) {
			s.Years[i].LaborHourlyEUR *= f
		}
	}
	return s, c
}

func perturbTax(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients) {
	for i := range s.Years {
		if !math.IsNaN(s.Years[i].TaxAmountEUR) {
			s.Years[i].TaxAmountEUR *= f
		}
	}
	return s, c
}

func perturbSubsidy(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients) {
	for i := range s.Years {
		s.Years[i].SubsidyAmountEUR *= f
		s.Years[i].SubsidyRange.Lo *= f
		s.Years[i].SubsidyRange.Hi *= f
	}
	return s, c
}

func perturbUtil(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients) {
	for i := range s.Years {
		v := s.Years[i].CapacityUtil * f
		s.Years[i].CapacityUtil = numutil.Clip(v, 0.01, 1.0)
	}
	return s, c
}

// perturbConstruction shifts the integer build period by one year; f<1
// shortens, f>1 lengthens, clamped to >=1.
func perturbConstruction(s model.Site, c model.Coefficients, f float64) (model.Site, model.Coefficients) {
	step := 1
	if f < 1 {
		step = -1
	}
	s.ConstructionYears += step
	if s.ConstructionYears < 1 {
		s.ConstructionYears = 1
	}
	if s.ConstructionYears >= c.HorizonYears {
		s.ConstructionYears = c.HorizonYears - 1
	}
	return s, c
}
