// Package model defines the domain types shared across the eurobatt
// site-selection simulation pipeline: raw input records, cleaned annual
// records, sites, physical/financial coefficients, scenario parameters and
// result containers.
package model

import "math"

// Scenario labels a market-assumption branch for the Monte Carlo run.
type Scenario string

const (
	ScenarioOptimistic  Scenario = "optimistic"
	ScenarioBaseline    Scenario = "baseline"
	ScenarioPessimistic Scenario = "pessimistic"
)

// AllScenarios returns scenarios in a fixed deterministic order.
func AllScenarios() []Scenario {
	return []Scenario{ScenarioOptimistic, ScenarioBaseline, ScenarioPessimistic}
}

// Range is a closed numeric interval used for interval estimation when a
// data point is missing.
type Range struct {
	Lo float64
	Hi float64
}

// Midpoint returns the midpoint of the range.
func (r Range) Midpoint() float64 {
	return (r.Lo + r.Hi) / 2
}

// Valid reports whether Lo <= Hi and both bounds are finite.
func (r Range) Valid() bool {
	return r.Lo <= r.Hi &&
		!math.IsNaN(r.Lo) && !math.IsNaN(r.Hi) &&
		!math.IsInf(r.Lo, 0) && !math.IsInf(r.Hi, 0)
}

// RawRecord is a row parsed directly from input before cleaning. Numeric
// fields use NaN to denote "absent in source".
type RawRecord struct {
	Site                string
	Year                int
	ElectricityPrice    float64
	ElectricityUnit     string
	GasPrice            float64
	GasUnit             string
	LaborHourly         float64
	TaxAmount           float64
	SubsidyAmount       float64
	SubsidyUnit         string
	CapacityUtil        float64
	CapacityUtilUnit    string
	CapacityGWh         float64
	ConstructionYears   int
	SubsidyDeadlineYear int
}

// YearRecord is a cleaned, unit-normalized annual record. Electricity and
// subsidy carry an interval estimate used when the source value is missing.
type YearRecord struct {
	Year                   int
	ElectricityPriceEURkWh float64 // EUR/kWh; NaN when missing
	ElectricityRange       Range   // interval estimate when missing
	ElectricityMissing     bool
	GasPriceEURkWh         float64 // EUR/kWh
	GasMissing             bool
	LaborHourlyEUR         float64 // EUR/hour
	LaborMissing           bool
	TaxAmountEUR           float64 // EUR/year
	TaxMissing             bool
	SubsidyAmountEUR       float64 // EUR total; NaN when missing
	SubsidyRange           Range   // interval estimate when missing
	SubsidyMissing         bool
	CapacityUtil           float64 // 0..1 ratio
	CapacityGWh            float64 // nameplate annual capacity
	Anomaly                bool
	AnomalyReason          string
}

// Site aggregates cleaned yearly records and site-level metadata.
type Site struct {
	Name                string
	ConstructionYears   int
	SubsidyDeadlineYear int
	CapacityGWh         float64
	Years               []YearRecord
	Flags               []string
}

// AddFlag appends a cleaning/processing flag.
func (s *Site) AddFlag(f string) {
	s.Flags = append(s.Flags, f)
}

// Coefficients holds physical and financial conversion constants shared by
// the cost standardization and simulation steps.
type Coefficients struct {
	ElectricityIntensityKWhPerKWh float64 // kWh electricity per kWh cell
	GasIntensityKWhPerKWh         float64 // kWh gas per kWh cell
	LaborHoursPerKWh              float64
	SellingPriceEURkWh            float64 // market price per kWh cell
	CapExEURkWh                   float64 // capex per kWh of nameplate capacity
	DiscountRate                  float64
	HorizonYears                  int
	Iterations                    int
	Seed                          uint64
}

// DefaultCoefficients returns realistic defaults for a European gigafactory
// (energy intensity ~50 kWh/kWh cell, ~20 kWh gas/kWh, 8% discount rate).
func DefaultCoefficients() Coefficients {
	return Coefficients{
		ElectricityIntensityKWhPerKWh: 50.0,
		GasIntensityKWhPerKWh:         20.0,
		LaborHoursPerKWh:              0.08,
		SellingPriceEURkWh:            95.0,
		CapExEURkWh:                   110.0,
		DiscountRate:                  0.08,
		HorizonYears:                  15,
		Iterations:                    4000,
		Seed:                          20260817,
	}
}

// ScenarioParams describes the sampling distribution for one scenario. ElecMean
// and SubMean are multipliers on the base electricity price and subsidy; the
// sigma terms are the standard deviation of the Gaussian noise.
type ScenarioParams struct {
	Name      Scenario
	ElecMean  float64
	ElecSigma float64
	SubMean   float64
	SubSigma  float64
}

// ScenarioTable returns the three scenario parameter sets.
func ScenarioTable() map[Scenario]ScenarioParams {
	return map[Scenario]ScenarioParams{
		ScenarioOptimistic:  {Name: ScenarioOptimistic, ElecMean: 0.85, ElecSigma: 0.04, SubMean: 1.15, SubSigma: 0.08},
		ScenarioBaseline:    {Name: ScenarioBaseline, ElecMean: 1.00, ElecSigma: 0.05, SubMean: 1.00, SubSigma: 0.10},
		ScenarioPessimistic: {Name: ScenarioPessimistic, ElecMean: 1.15, ElecSigma: 0.06, SubMean: 0.85, SubSigma: 0.12},
	}
}

// CostBreakdown is the per-kWh standardized cost structure for a site.
type CostBreakdown struct {
	Site              string
	ElectricityEURkWh float64
	GasEURkWh         float64
	LaborEURkWh       float64
	TaxEURkWh         float64
	SubsidyEURkWh     float64
	NetCostEURkWh     float64
	ProductionKWh     float64
}

// SimResult holds simulation outputs for one site and scenario.
type SimResult struct {
	Site          string
	Scenario      Scenario
	MeanNPV       float64
	P5            float64
	P50           float64
	P95           float64
	BreakEvenYear float64 // -1 if never within horizon
	BreakEvenProb float64
	YearlyMeanNPV []float64
	TotalSubsidy  float64
}

// SensitivityItem ranks one input's effect on NPV.
type SensitivityItem struct {
	Parameter  string
	BaseNPV    float64
	LowNPV     float64
	HighNPV    float64
	Swing      float64
	Normalized float64
	Top        bool
	MarketKey  bool
}

// MissingItem describes a required field that is absent for a site.
type MissingItem struct {
	Site   string
	Field  string
	Reason string
}

// ValidationError aggregates missing items; it fails the whole run so the
// operator can补齐 (backfill) and rerun deterministically.
type ValidationError struct {
	Missing []MissingItem
}

func (e *ValidationError) Error() string {
	msg := "site data incomplete; missing items:"
	for _, m := range e.Missing {
		msg += "\n  - " + m.Site + ": " + m.Field + " (" + m.Reason + ")"
	}
	return msg
}

// FinancialContext carries the analysis start year and site lookup that the
// simulation engine needs. It is computed once by the orchestrator.
type FinancialContext struct {
	StartYear int
}
