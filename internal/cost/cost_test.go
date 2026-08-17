package cost

import (
	"math"
	"testing"

	"eurobatt/internal/model"
)

func site(name string, elec, gas, labor, tax, sub, util, capGWh float64, cons, dl int) model.Site {
	y := model.YearRecord{
		Year: 2025, ElectricityPriceEURkWh: elec, GasPriceEURkWh: gas,
		LaborHourlyEUR: labor, TaxAmountEUR: tax, SubsidyAmountEUR: sub,
		CapacityUtil: util, CapacityGWh: capGWh,
		ElectricityRange: model.Range{Lo: elec, Hi: elec},
		SubsidyRange:     model.Range{Lo: sub, Hi: sub},
	}
	return model.Site{Name: name, ConstructionYears: cons, SubsidyDeadlineYear: dl, CapacityGWh: capGWh, Years: []model.YearRecord{y}}
}

func TestBreakdownPerKWh(t *testing.T) {
	c := model.DefaultCoefficients()
	s := site("DE", 0.18, 0.09, 45, 5e6, 50*40*1e6, 0.75, 40, 3, 2030)
	bd := Breakdown([]model.Site{s}, c)[0]
	if math.Abs(bd.ElectricityEURkWh-0.18*50) > 1e-6 {
		t.Fatalf("electricity/kWh = %v, want %v", bd.ElectricityEURkWh, 0.18*50)
	}
	if math.Abs(bd.GasEURkWh-0.09*20) > 1e-6 {
		t.Fatalf("gas/kWh = %v, want %v", bd.GasEURkWh, 0.09*20)
	}
	if math.Abs(bd.LaborEURkWh-45*0.08) > 1e-6 {
		t.Fatalf("labor/kWh = %v, want %v", bd.LaborEURkWh, 45*0.08)
	}
	annualProd := 40e6 * 0.75
	if math.Abs(bd.TaxEURkWh-5e6/annualProd) > 1e-9 {
		t.Fatalf("tax/kWh = %v, want %v", bd.TaxEURkWh, 5e6/annualProd)
	}
	prodYears := c.HorizonYears - 3
	if math.Abs(bd.SubsidyEURkWh-(50*40*1e6)/(annualProd*float64(prodYears))) > 1e-9 {
		t.Fatalf("subsidy/kWh = %v", bd.SubsidyEURkWh)
	}
	want := bd.ElectricityEURkWh + bd.GasEURkWh + bd.LaborEURkWh + bd.TaxEURkWh - bd.SubsidyEURkWh
	if math.Abs(bd.NetCostEURkWh-want) > 1e-9 {
		t.Fatalf("net cost = %v, want %v", bd.NetCostEURkWh, want)
	}
}

func TestCompareOrdersByNetCost(t *testing.T) {
	c := model.DefaultCoefficients()
	cheap := site("Cheap", 0.10, 0.05, 25, 1e6, 70*40*1e6, 0.8, 40, 2, 2030)
	pricey := site("Pricey", 0.30, 0.12, 50, 8e6, 10*40*1e6, 0.7, 40, 3, 2030)
	ordered := Compare([]model.CostBreakdown{Breakdown([]model.Site{pricey, cheap}, c)[0], Breakdown([]model.Site{pricey, cheap}, c)[1]})
	if ordered[0].Site != "Cheap" {
		t.Fatalf("expected Cheap first, got %s (net %.2f vs %.2f)", ordered[0].Site, ordered[0].NetCostEURkWh, ordered[1].NetCostEURkWh)
	}
}
