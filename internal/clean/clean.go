// Package clean normalizes heterogeneous input units, removes duplicate
// records, flags and removes single-year anomalies, and fills missing
// electricity/subsidy values with interval estimates. It also validates
// site completeness, failing the whole run with a missing-item list when a
// site cannot support a deterministic simulation.
package clean

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"eurobatt/internal/model"
	"eurobatt/internal/numutil"
)

const (
	anomalyZThreshold = 3.5
	minAnomalyPoints  = 4
)

// Clean normalizes units, removes duplicates, flags/removes anomalies and
// fills missing values with interval estimates. Sites are returned sorted by
// name with year records sorted ascending. Cleaning is deterministic.
func Clean(raw []model.RawRecord) ([]model.Site, error) {
	bySite := groupBySite(raw)
	names := make([]string, 0, len(bySite))
	for k := range bySite {
		names = append(names, k)
	}
	sort.Strings(names)
	sites := make([]model.Site, 0, len(names))
	for _, name := range names {
		s, err := cleanSite(name, bySite[name])
		if err != nil {
			return nil, fmt.Errorf("site %s: %w", name, err)
		}
		sites = append(sites, s)
	}
	return sites, nil
}

// Validate checks that every site has the data required for a deterministic
// simulation. It returns a *ValidationError enumerating missing items so the
// operator can backfill and rerun.
func Validate(sites []model.Site) *model.ValidationError {
	var missing []model.MissingItem
	for _, s := range sites {
		if len(s.Years) == 0 {
			missing = append(missing, model.MissingItem{Site: s.Name, Field: "records", Reason: "no year records"})
			continue
		}
		if math.IsNaN(s.CapacityGWh) || s.CapacityGWh <= 0 {
			missing = append(missing, model.MissingItem{Site: s.Name, Field: "capacity_gwh", Reason: "absent or non-positive"})
		}
		if s.ConstructionYears <= 0 {
			missing = append(missing, model.MissingItem{Site: s.Name, Field: "construction_years", Reason: "absent or non-positive"})
		}
		if s.SubsidyDeadlineYear <= 0 {
			missing = append(missing, model.MissingItem{Site: s.Name, Field: "subsidy_deadline_year", Reason: "absent"})
		}
		hasGas, hasLabor := false, false
		for _, y := range s.Years {
			if !y.GasMissing && !math.IsNaN(y.GasPriceEURkWh) {
				hasGas = true
			}
			if !y.LaborMissing && !math.IsNaN(y.LaborHourlyEUR) {
				hasLabor = true
			}
		}
		if !hasGas {
			missing = append(missing, model.MissingItem{Site: s.Name, Field: "gas_price", Reason: "no finite value across years"})
		}
		if !hasLabor {
			missing = append(missing, model.MissingItem{Site: s.Name, Field: "labor_hourly", Reason: "no finite value across years"})
		}
	}
	if len(missing) > 0 {
		return &model.ValidationError{Missing: missing}
	}
	return nil
}

func groupBySite(raw []model.RawRecord) map[string][]model.RawRecord {
	m := make(map[string][]model.RawRecord)
	for _, r := range raw {
		m[r.Site] = append(m[r.Site], r)
	}
	return m
}

func cleanSite(name string, raw []model.RawRecord) (model.Site, error) {
	s := model.Site{Name: name}
	sort.Slice(raw, func(i, j int) bool { return raw[i].Year < raw[j].Year })

	capVals := finiteFields(raw, func(r model.RawRecord) float64 { return r.CapacityGWh })
	siteCap := math.NaN()
	if len(capVals) > 0 {
		siteCap = numutil.Median(capVals)
	}

	recs := make([]model.YearRecord, 0, len(raw))
	for _, r := range raw {
		yr, err := convertRecord(r, siteCap)
		if err != nil {
			return s, err
		}
		recs = append(recs, yr)
	}

	recs = dedupByYear(recs, &s)
	detectAnomalies(recs, &s)
	estimateIntervals(recs, siteCap, &s)
	setSiteMeta(&s, recs, raw)
	propagateCapacity(recs, s.CapacityGWh)
	s.Years = recs
	return s, nil
}

func convertRecord(r model.RawRecord, siteCap float64) (model.YearRecord, error) {
	yr := model.YearRecord{Year: r.Year}

	e, err := normalizeElectricity(r.ElectricityPrice, r.ElectricityUnit)
	if err != nil {
		return yr, err
	}
	if math.IsNaN(e) {
		yr.ElectricityMissing = true
		yr.ElectricityPriceEURkWh = math.NaN()
	} else {
		yr.ElectricityPriceEURkWh = e
	}

	g, err := normalizeGas(r.GasPrice, r.GasUnit)
	if err != nil {
		return yr, err
	}
	if math.IsNaN(g) {
		yr.GasMissing = true
		yr.GasPriceEURkWh = math.NaN()
	} else {
		yr.GasPriceEURkWh = g
	}

	if math.IsNaN(r.LaborHourly) {
		yr.LaborMissing = true
		yr.LaborHourlyEUR = math.NaN()
	} else {
		yr.LaborHourlyEUR = r.LaborHourly
	}

	if math.IsNaN(r.TaxAmount) {
		yr.TaxMissing = true
		yr.TaxAmountEUR = math.NaN()
	} else {
		yr.TaxAmountEUR = r.TaxAmount
	}

	u, err := normalizeUtil(r.CapacityUtil, r.CapacityUtilUnit)
	if err != nil {
		return yr, err
	}
	if math.IsNaN(u) {
		yr.CapacityUtil = math.NaN()
	} else {
		yr.CapacityUtil = u
	}

	if !math.IsNaN(r.CapacityGWh) && r.CapacityGWh > 0 {
		yr.CapacityGWh = r.CapacityGWh
	} else if !math.IsNaN(siteCap) && siteCap > 0 {
		yr.CapacityGWh = siteCap
	} else {
		yr.CapacityGWh = math.NaN()
	}

	sub, err := normalizeSubsidy(r.SubsidyAmount, r.SubsidyUnit, yr.CapacityGWh)
	if err != nil {
		return yr, err
	}
	if math.IsNaN(sub) {
		yr.SubsidyMissing = true
		yr.SubsidyAmountEUR = math.NaN()
	} else {
		yr.SubsidyAmountEUR = sub
	}
	return yr, nil
}

func dedupByYear(recs []model.YearRecord, s *model.Site) []model.YearRecord {
	out := make([]model.YearRecord, 0, len(recs))
	seen := make(map[int]int, len(recs))
	for _, r := range recs {
		if idx, ok := seen[r.Year]; ok {
			if recordsEqual(out[idx], r) {
				s.AddFlag(fmt.Sprintf("duplicate_year_%d_removed", r.Year))
				continue
			}
			s.AddFlag(fmt.Sprintf("duplicate_year_%d_conflict_kept_first", r.Year))
			continue
		}
		seen[r.Year] = len(out)
		out = append(out, r)
	}
	return out
}

func detectAnomalies(recs []model.YearRecord, s *model.Site) {
	check := func(name string, get func(model.YearRecord) float64, mark func(*model.YearRecord, string)) {
		vals := finiteRecs(recs, get)
		if len(vals) < minAnomalyPoints {
			return
		}
		med := numutil.Median(vals)
		mad := numutil.MAD(vals)
		if mad == 0 {
			return
		}
		scale := 1.4826 * mad
		for i := range recs {
			v := get(recs[i])
			if math.IsNaN(v) {
				continue
			}
			z := math.Abs(v-med) / scale
			if z > anomalyZThreshold {
				mark(&recs[i], fmt.Sprintf("%s_outlier_z=%.1f", name, z))
			}
		}
	}
	check("electricity",
		func(r model.YearRecord) float64 { return r.ElectricityPriceEURkWh },
		func(r *model.YearRecord, reason string) {
			r.ElectricityMissing = true
			r.ElectricityPriceEURkWh = math.NaN()
			r.Anomaly = true
			r.AnomalyReason = reason
		})
	check("gas",
		func(r model.YearRecord) float64 { return r.GasPriceEURkWh },
		func(r *model.YearRecord, reason string) {
			r.GasMissing = true
			r.GasPriceEURkWh = math.NaN()
			r.Anomaly = true
			r.AnomalyReason = reason
		})
	check("labor",
		func(r model.YearRecord) float64 { return r.LaborHourlyEUR },
		func(r *model.YearRecord, reason string) {
			r.LaborMissing = true
			r.LaborHourlyEUR = math.NaN()
			r.Anomaly = true
			r.AnomalyReason = reason
		})
}

func estimateIntervals(recs []model.YearRecord, siteCap float64, s *model.Site) {
	// Electricity interval estimate.
	elecVals := finiteRecs(recs, func(r model.YearRecord) float64 { return r.ElectricityPriceEURkWh })
	var elecRange model.Range
	if len(elecVals) > 0 {
		elecRange = model.Range{Lo: numutil.Min(elecVals), Hi: numutil.Max(elecVals)}
	} else {
		elecRange = model.Range{Lo: 0.08, Hi: 0.35}
		s.AddFlag("electricity_default_band")
	}
	for i := range recs {
		if recs[i].ElectricityMissing {
			recs[i].ElectricityRange = elecRange
			recs[i].ElectricityPriceEURkWh = elecRange.Midpoint()
		} else {
			recs[i].ElectricityRange = model.Range{Lo: recs[i].ElectricityPriceEURkWh, Hi: recs[i].ElectricityPriceEURkWh}
		}
	}

	// Subsidy interval estimate.
	subVals := finiteRecs(recs, func(r model.YearRecord) float64 { return r.SubsidyAmountEUR })
	var subRange model.Range
	if len(subVals) > 0 {
		subRange = model.Range{Lo: numutil.Min(subVals), Hi: numutil.Max(subVals)}
	} else {
		cap := siteCap
		if math.IsNaN(cap) || cap <= 0 {
			cap = 30
		}
		subRange = model.Range{Lo: 0, Hi: cap * 1e6 * 30}
		s.AddFlag("subsidy_default_band")
	}
	for i := range recs {
		if recs[i].SubsidyMissing {
			recs[i].SubsidyRange = subRange
			recs[i].SubsidyAmountEUR = subRange.Midpoint()
		} else {
			recs[i].SubsidyRange = model.Range{Lo: recs[i].SubsidyAmountEUR, Hi: recs[i].SubsidyAmountEUR}
		}
	}

	fillPoint(recs,
		func(r model.YearRecord) float64 { return r.GasPriceEURkWh },
		func(r *model.YearRecord, v float64) { r.GasPriceEURkWh = v; r.GasMissing = false },
		"gas", s)
	fillPoint(recs,
		func(r model.YearRecord) float64 { return r.LaborHourlyEUR },
		func(r *model.YearRecord, v float64) { r.LaborHourlyEUR = v; r.LaborMissing = false },
		"labor", s)
	fillPoint(recs,
		func(r model.YearRecord) float64 { return r.TaxAmountEUR },
		func(r *model.YearRecord, v float64) { r.TaxAmountEUR = v; r.TaxMissing = false },
		"tax", s)

	// Capacity utilization point fill.
	utils := finiteRecs(recs, func(r model.YearRecord) float64 { return r.CapacityUtil })
	utilMid := 0.75
	if len(utils) > 0 {
		utilMid = numutil.Median(utils)
	} else {
		s.AddFlag("capacity_util_default")
	}
	for i := range recs {
		if math.IsNaN(recs[i].CapacityUtil) {
			recs[i].CapacityUtil = utilMid
		}
	}
}

func fillPoint(recs []model.YearRecord, get func(model.YearRecord) float64, set func(*model.YearRecord, float64), name string, s *model.Site) {
	vals := finiteRecs(recs, get)
	if len(vals) == 0 {
		if name == "tax" {
			for i := range recs {
				if math.IsNaN(get(recs[i])) {
					recs[i].TaxAmountEUR = 0
					recs[i].TaxMissing = false
				}
			}
			s.AddFlag("tax_assumed_zero")
		}
		return
	}
	mid := numutil.Median(vals)
	for i := range recs {
		if math.IsNaN(get(recs[i])) {
			set(&recs[i], mid)
		}
	}
}

func setSiteMeta(s *model.Site, recs []model.YearRecord, raw []model.RawRecord) {
	caps := finiteRecs(recs, func(r model.YearRecord) float64 { return r.CapacityGWh })
	if len(caps) > 0 {
		s.CapacityGWh = numutil.Median(caps)
		if numutil.Min(caps) != numutil.Max(caps) {
			s.AddFlag("capacity_disagreement")
		}
	}
	cons := make([]int, 0)
	for _, r := range raw {
		if r.ConstructionYears > 0 {
			cons = append(cons, r.ConstructionYears)
		}
	}
	if len(cons) > 0 {
		s.ConstructionYears = intMedian(cons)
		if minInt(cons) != maxInt(cons) {
			s.AddFlag("construction_years_disagreement")
		}
	}
	dls := make([]int, 0)
	for _, r := range raw {
		if r.SubsidyDeadlineYear > 0 {
			dls = append(dls, r.SubsidyDeadlineYear)
		}
	}
	if len(dls) > 0 {
		s.SubsidyDeadlineYear = maxInt(dls)
		if minInt(dls) != maxInt(dls) {
			s.AddFlag("subsidy_deadline_disagreement")
		}
	}
}

func propagateCapacity(recs []model.YearRecord, siteCap float64) {
	for i := range recs {
		if math.IsNaN(recs[i].CapacityGWh) || recs[i].CapacityGWh <= 0 {
			recs[i].CapacityGWh = siteCap
		}
	}
}

// ---- unit normalization ----

func normalizeElectricity(price float64, unit string) (float64, error) {
	if math.IsNaN(price) {
		return math.NaN(), nil
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "eur/kwh":
		return price, nil
	case "eur/mwh", "eur/mwh_th":
		return price / 1000.0, nil
	case "ceur/kwh", "cent/kwh", "cents/kwh":
		return price / 100.0, nil
	case "eur/gj":
		return price / 277.7777778, nil
	default:
		return 0, fmt.Errorf("unsupported electricity unit %q", unit)
	}
}

func normalizeGas(price float64, unit string) (float64, error) {
	if math.IsNaN(price) {
		return math.NaN(), nil
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "eur/kwh":
		return price, nil
	case "eur/mwh", "eur/mwh_th":
		return price / 1000.0, nil
	case "ceur/kwh", "cent/kwh":
		return price / 100.0, nil
	default:
		return 0, fmt.Errorf("unsupported gas unit %q", unit)
	}
}

func normalizeUtil(v float64, unit string) (float64, error) {
	if math.IsNaN(v) {
		return math.NaN(), nil
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "ratio":
		return numutil.Clip(v, 0, 1), nil
	case "percent", "%":
		return numutil.Clip(v/100.0, 0, 1), nil
	default:
		return 0, fmt.Errorf("unsupported capacity_util unit %q", unit)
	}
}

func normalizeSubsidy(amount float64, unit string, capacityGWh float64) (float64, error) {
	if math.IsNaN(amount) {
		return math.NaN(), nil
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "eur", "eur_total":
		return amount, nil
	case "eur/kwh", "eur/kwh_cap":
		if math.IsNaN(capacityGWh) || capacityGWh <= 0 {
			return math.NaN(), nil
		}
		return amount * capacityGWh * 1e6, nil
	case "eur/mwh":
		if math.IsNaN(capacityGWh) || capacityGWh <= 0 {
			return math.NaN(), nil
		}
		return amount * capacityGWh * 1e3, nil
	default:
		return 0, fmt.Errorf("unsupported subsidy unit %q", unit)
	}
}

// ---- helpers ----

func finiteRecs(recs []model.YearRecord, get func(model.YearRecord) float64) []float64 {
	out := make([]float64, 0, len(recs))
	for _, r := range recs {
		v := get(r)
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
	}
	return out
}

func finiteFields(raw []model.RawRecord, get func(model.RawRecord) float64) []float64 {
	out := make([]float64, 0, len(raw))
	for _, r := range raw {
		v := get(r)
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
	}
	return out
}

func eqNaN(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	return numutil.ApproxEqual(a, b, 1e-9)
}

func recordsEqual(a, b model.YearRecord) bool {
	return a.Year == b.Year &&
		eqNaN(a.ElectricityPriceEURkWh, b.ElectricityPriceEURkWh) &&
		eqNaN(a.GasPriceEURkWh, b.GasPriceEURkWh) &&
		eqNaN(a.LaborHourlyEUR, b.LaborHourlyEUR) &&
		eqNaN(a.TaxAmountEUR, b.TaxAmountEUR) &&
		eqNaN(a.SubsidyAmountEUR, b.SubsidyAmountEUR) &&
		numutil.ApproxEqual(a.CapacityUtil, b.CapacityUtil, 1e-9) &&
		numutil.ApproxEqual(a.CapacityGWh, b.CapacityGWh, 1e-9)
}

func intMedian(xs []int) int {
	s := append([]int(nil), xs...)
	sort.Ints(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func minInt(xs []int) int {
	m := xs[0]
	for _, v := range xs[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxInt(xs []int) int {
	m := xs[0]
	for _, v := range xs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
