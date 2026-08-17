// Package monte runs the Monte Carlo simulation that turns cleaned site data
// into NPV distributions, break-even timing and subsidy-capture outcomes under
// optimistic, baseline and pessimistic electricity-price and subsidy scenarios.
//
// Reproducibility: every run seeds a PCG generator from the global seed XOR a
// per-site FNV hash and a per-scenario offset, so identical inputs yield
// byte-identical outputs across repeated executions.
package monte

import (
	"fmt"
	"math"
	"math/rand/v2"

	"eurobatt/internal/model"
	"eurobatt/internal/numutil"
)

// Simulate runs the Monte Carlo over all sites and scenarios. A common analysis
// start year (the earliest data year across sites) is used so NPVs are
// comparable between sites.
func Simulate(sites []model.Site, coeffs model.Coefficients) ([]model.SimResult, error) {
	if len(sites) == 0 {
		return nil, fmt.Errorf("no sites to simulate")
	}
	if coeffs.HorizonYears <= 0 || coeffs.Iterations <= 0 {
		return nil, fmt.Errorf("invalid horizon/iterations")
	}
	startYear := math.MaxInt32
	for _, s := range sites {
		for _, y := range s.Years {
			if y.Year < startYear {
				startYear = y.Year
			}
		}
	}
	if startYear == math.MaxInt32 {
		startYear = 0
	}
	table := model.ScenarioTable()
	results := make([]model.SimResult, 0, len(sites)*len(model.AllScenarios()))
	for _, s := range sites {
		for _, sc := range model.AllScenarios() {
			results = append(results, runOne(s, coeffs, table[sc], startYear))
		}
	}
	return results, nil
}

func runOne(s model.Site, c model.Coefficients, sp model.ScenarioParams, startYear int) model.SimResult {
	src := rand.NewPCG(c.Seed^siteSeed(s.Name), scenarioSeed(sp.Name))
	r := rand.New(src)

	iters := c.Iterations
	horizon := c.HorizonYears
	npvs := make([]float64, iters)
	breakEvens := make([]float64, iters)
	totalSub := make([]float64, iters)
	yearly := make([][]float64, iters)
	for i := range yearly {
		yearly[i] = make([]float64, horizon)
	}

	totalCapex := c.CapExEURkWh * s.CapacityGWh * 1e6
	annualCapex := 0.0
	if s.ConstructionYears > 0 {
		annualCapex = -totalCapex / float64(s.ConstructionYears)
	}
	n := len(s.Years)

	for iter := 0; iter < iters; iter++ {
		elecMult := numutil.Clip(r.NormFloat64()*sp.ElecSigma+sp.ElecMean, 0.1, 3.0)
		subMult := numutil.Clip(r.NormFloat64()*sp.SubSigma+sp.SubMean, 0.0, 3.0)

		elecBase := make([]float64, n)
		subBase := make([]float64, n)
		for i, rec := range s.Years {
			if rec.ElectricityMissing {
				elecBase[i] = sampleRange(r, rec.ElectricityRange)
			} else {
				elecBase[i] = rec.ElectricityPriceEURkWh
			}
			if rec.SubsidyMissing {
				subBase[i] = sampleRange(r, rec.SubsidyRange)
			} else {
				subBase[i] = rec.SubsidyAmountEUR
			}
		}

		cum := 0.0
		prev := 0.0
		beYear := -1.0
		for t := 0; t < horizon; t++ {
			cf := 0.0
			if t < s.ConstructionYears {
				cf = annualCapex
			} else {
				idx := recordIndexForYear(s, startYear+t)
				rec := s.Years[idx]
				production := rec.CapacityGWh * 1e6 * rec.CapacityUtil
				if production <= 0 || math.IsNaN(production) {
					production = 0
				}
				elec := elecBase[idx] * elecMult
				elecCost := elec * c.ElectricityIntensityKWhPerKWh * production
				gasCost := rec.GasPriceEURkWh * c.GasIntensityKWhPerKWh * production
				laborCost := rec.LaborHourlyEUR * c.LaborHoursPerKWh * production
				taxCost := rec.TaxAmountEUR
				revenue := production * c.SellingPriceEURkWh
				cf = revenue - elecCost - gasCost - laborCost - taxCost
				if t == s.ConstructionYears && (startYear+t) < s.SubsidyDeadlineYear {
					subsidy := subBase[idx] * subMult
					cf += subsidy
					totalSub[iter] += subsidy
				}
			}
			disc := math.Pow(1+c.DiscountRate, float64(t))
			cum += cf / disc
			yearly[iter][t] = cum
			if beYear < 0 && cum >= 0 && t >= s.ConstructionYears {
				if prev < 0 {
					beYear = float64(t-1) + (-prev)/(cum-prev)
				} else {
					beYear = float64(t)
				}
			}
			prev = cum
		}
		npvs[iter] = cum
		breakEvens[iter] = beYear
	}

	sorted := numutil.SortedCopy(npvs)
	return model.SimResult{
		Site:          s.Name,
		Scenario:      sp.Name,
		MeanNPV:       numutil.Mean(npvs),
		P5:            numutil.Percentile(sorted, 5),
		P50:           numutil.Percentile(sorted, 50),
		P95:           numutil.Percentile(sorted, 95),
		BreakEvenYear: meanBreakEven(breakEvens),
		BreakEvenProb: breakEvenProb(breakEvens),
		YearlyMeanNPV: yearlyMean(yearly, horizon),
		TotalSubsidy:  numutil.Mean(totalSub),
	}
}

// recordIndexForYear returns the index of the year record closest to calYear,
// breaking ties toward the earlier year. Years are sorted ascending by clean.
func recordIndexForYear(s model.Site, calYear int) int {
	best := 0
	bestDiff := absInt(calYear - s.Years[0].Year)
	for i := 1; i < len(s.Years); i++ {
		d := absInt(calYear - s.Years[i].Year)
		if d < bestDiff || (d == bestDiff && s.Years[i].Year < s.Years[best].Year) {
			best = i
			bestDiff = d
		}
	}
	return best
}

func sampleRange(r *rand.Rand, rg model.Range) float64 {
	if !rg.Valid() {
		return rg.Midpoint()
	}
	return rg.Lo + (rg.Hi-rg.Lo)*r.Float64()
}

func yearlyMean(yearly [][]float64, horizon int) []float64 {
	out := make([]float64, horizon)
	for t := 0; t < horizon; t++ {
		col := make([]float64, len(yearly))
		for i := range yearly {
			col[i] = yearly[i][t]
		}
		out[t] = numutil.Mean(col)
	}
	return out
}

func breakEvenProb(bes []float64) float64 {
	if len(bes) == 0 {
		return 0
	}
	n := 0
	for _, b := range bes {
		if b >= 0 {
			n++
		}
	}
	return float64(n) / float64(len(bes))
}

func meanBreakEven(bes []float64) float64 {
	sum, n := 0.0, 0
	for _, b := range bes {
		if b >= 0 {
			sum += b
			n++
		}
	}
	if n == 0 {
		return -1
	}
	return sum / float64(n)
}

func siteSeed(name string) uint64 {
	h := uint64(1469598103934665603)
	for i := 0; i < len(name); i++ {
		h ^= uint64(name[i])
		h *= 1099511628211
	}
	return h
}

func scenarioSeed(s model.Scenario) uint64 {
	switch s {
	case model.ScenarioOptimistic:
		return 1
	case model.ScenarioBaseline:
		return 2
	case model.ScenarioPessimistic:
		return 3
	default:
		return 0
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
