// Package cost standardizes each site's heterogeneous annual inputs into a
// per-kWh cell cost structure (electricity, gas, labor, tax, subsidy) using
// robust median representatives and the physical intensity coefficients.
package cost

import (
	"math"

	"eurobatt/internal/model"
	"eurobatt/internal/numutil"
)

// Breakdown computes a per-kWh cost structure for each site. The
// representative annual values are the medians across the cleaned year
// records, which suppresses residual noise after anomaly removal. The one-time
// subsidy is amortized across the production years of the analysis horizon.
func Breakdown(sites []model.Site, c model.Coefficients) []model.CostBreakdown {
	out := make([]model.CostBreakdown, 0, len(sites))
	for _, s := range sites {
		if len(s.Years) == 0 {
			continue
		}
		elec := medianField(s, func(y model.YearRecord) float64 { return y.ElectricityPriceEURkWh })
		gas := medianField(s, func(y model.YearRecord) float64 { return y.GasPriceEURkWh })
		labor := medianField(s, func(y model.YearRecord) float64 { return y.LaborHourlyEUR })
		tax := medianField(s, func(y model.YearRecord) float64 { return y.TaxAmountEUR })
		sub := medianField(s, func(y model.YearRecord) float64 { return y.SubsidyAmountEUR })
		util := medianField(s, func(y model.YearRecord) float64 { return y.CapacityUtil })

		annualProd := s.CapacityGWh * 1e6 * util
		if annualProd <= 0 || math.IsNaN(annualProd) {
			annualProd = math.NaN()
		}
		prodYears := c.HorizonYears - s.ConstructionYears
		if prodYears <= 0 {
			prodYears = 1
		}

		cb := model.CostBreakdown{
			Site:              s.Name,
			ElectricityEURkWh: elec * c.ElectricityIntensityKWhPerKWh,
			GasEURkWh:         gas * c.GasIntensityKWhPerKWh,
			LaborEURkWh:       labor * c.LaborHoursPerKWh,
			ProductionKWh:     annualProd,
		}
		if !math.IsNaN(annualProd) && annualProd > 0 {
			cb.TaxEURkWh = tax / annualProd
			cb.SubsidyEURkWh = sub / (annualProd * float64(prodYears))
		}
		cb.NetCostEURkWh = cb.ElectricityEURkWh + cb.GasEURkWh + cb.LaborEURkWh + cb.TaxEURkWh - cb.SubsidyEURkWh
		out = append(out, cb)
	}
	return out
}

// Compare returns the sites ordered by net cost ascending (cheapest first).
func Compare(breakdowns []model.CostBreakdown) []model.CostBreakdown {
	out := make([]model.CostBreakdown, len(breakdowns))
	copy(out, breakdowns)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].NetCostEURkWh > out[j].NetCostEURkWh; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func medianField(s model.Site, get func(model.YearRecord) float64) float64 {
	vals := make([]float64, 0, len(s.Years))
	for _, y := range s.Years {
		v := get(y)
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return math.NaN()
	}
	return numutil.Median(vals)
}
