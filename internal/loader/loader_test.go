package loader

import (
	"errors"
	"math"
	"strings"
	"testing"
)

var errSourceInterrupted = errors.New("source interrupted")

type headerThenErrorReader struct {
	header *strings.Reader
}

func (r *headerThenErrorReader) Read(p []byte) (int, error) {
	if r.header.Len() > 0 {
		return r.header.Read(p)
	}
	return 0, errSourceInterrupted
}

func TestLoadCSVClean(t *testing.T) {
	in := strings.Join([]string{
		"site,year,electricity_price,electricity_unit,gas_price,gas_unit,labor_hourly,tax_amount,subsidy_amount,subsidy_unit,capacity_util,capacity_util_unit,capacity_gwh,construction_years,subsidy_deadline_year",
		"Germany,2025,180,EUR/MWh,90,EUR/MWh,45,5000000,50,EUR/kWh,0.75,ratio,40,3,2030",
		"France,2025,12,cEUR/kWh,80,EUR/MWh,42,4000000,60,EUR/kWh,75,percent,30,2,2029",
	}, "\n")
	recs, err := LoadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatalf("LoadCSV: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	if recs[0].Site != "Germany" || recs[0].Year != 2025 {
		t.Fatalf("unexpected record: %+v", recs[0])
	}
	if recs[0].ElectricityPrice != 180 {
		t.Fatalf("electricity = %v", recs[0].ElectricityPrice)
	}
	if recs[1].CapacityUtil != 75 {
		t.Fatalf("capacity util raw = %v", recs[1].CapacityUtil)
	}
}

func TestLoadCSVMissingFields(t *testing.T) {
	in := "site,year,electricity_price,electricity_unit\nSpain,2026,,EUR/MWh\n"
	recs, err := LoadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatalf("LoadCSV: %v", err)
	}
	if !math.IsNaN(recs[0].ElectricityPrice) {
		t.Fatalf("missing electricity should be NaN, got %v", recs[0].ElectricityPrice)
	}
}

func TestLoadCSVErrors(t *testing.T) {
	cases := map[string]string{
		"missing site col": "year,electricity_price\n2025,180\n",
		"unknown column":   "site,year,foo\nA,2025,1\n",
		"empty data":       "site,year\n",
		"bad year":         "site,year\nA,notayear\n",
		"empty site":       "site,year\n,2025\n",
		"dup column":       "site,site,year\nA,A,2025\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadCSV(strings.NewReader(in)); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
}

func TestLoadCSVPreservesReaderError(t *testing.T) {
	r := &headerThenErrorReader{header: strings.NewReader("site,year\n")}
	_, err := LoadCSV(r)
	if !errors.Is(err, errSourceInterrupted) {
		t.Fatalf("expected source error to remain discoverable, got %v", err)
	}
}
