// Package loader parses raw site-year records from CSV input into model
// records. It is deliberately tolerant of missing fields (encoded as NaN) so
// that cleaning can later decide how to estimate them.
package loader

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"eurobatt/internal/model"
)

// Required columns for a valid input file.
var requiredColumns = []string{"site", "year"}

// expected columns (order independent).
var columnSet = map[string]bool{
	"site": true, "year": true,
	"electricity_price": true, "electricity_unit": true,
	"gas_price": true, "gas_unit": true,
	"labor_hourly": true, "tax_amount": true,
	"subsidy_amount": true, "subsidy_unit": true,
	"capacity_util": true, "capacity_util_unit": true,
	"capacity_gwh":       true,
	"construction_years": true, "subsidy_deadline_year": true,
}

// LoadCSV reads raw records from r. Missing numeric cells become NaN and
// empty text cells stay empty strings; cleaning interprets them later.
func LoadCSV(r io.Reader) ([]model.RawRecord, error) {
	rd := csv.NewReader(r)
	rd.FieldsPerRecord = -1
	rd.TrimLeadingSpace = true
	header, err := rd.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	idx, err := indexHeader(header)
	if err != nil {
		return nil, err
	}
	var recs []model.RawRecord
	lineNo := 1
	for {
		row, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo+1, err)
		}
		lineNo++
		rec, perr := parseRow(row, idx)
		if perr != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, perr)
		}
		if rec.Site == "" {
			return nil, fmt.Errorf("line %d: empty site", lineNo)
		}
		recs = append(recs, rec)
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("no data rows")
	}
	return recs, nil
}

func indexHeader(header []string) (map[string]int, error) {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if !columnSet[key] {
			return nil, fmt.Errorf("unknown column %q", h)
		}
		if _, dup := idx[key]; dup {
			return nil, fmt.Errorf("duplicate column %q", h)
		}
		idx[key] = i
	}
	for _, c := range requiredColumns {
		if _, ok := idx[c]; !ok {
			return nil, fmt.Errorf("missing required column %q", c)
		}
	}
	return idx, nil
}

func parseRow(row []string, idx map[string]int) (model.RawRecord, error) {
	get := func(col string) string {
		i, ok := idx[col]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	year, err := parseInt(get("year"))
	if err != nil {
		return model.RawRecord{}, fmt.Errorf("year: %w", err)
	}
	rec := model.RawRecord{
		Site:                get("site"),
		Year:                year,
		ElectricityPrice:    parseFloat(get("electricity_price")),
		ElectricityUnit:     get("electricity_unit"),
		GasPrice:            parseFloat(get("gas_price")),
		GasUnit:             get("gas_unit"),
		LaborHourly:         parseFloat(get("labor_hourly")),
		TaxAmount:           parseFloat(get("tax_amount")),
		SubsidyAmount:       parseFloat(get("subsidy_amount")),
		SubsidyUnit:         get("subsidy_unit"),
		CapacityUtil:        parseFloat(get("capacity_util")),
		CapacityUtilUnit:    get("capacity_util_unit"),
		CapacityGWh:         parseFloat(get("capacity_gwh")),
		ConstructionYears:   parseIntDefault(get("construction_years"), 0),
		SubsidyDeadlineYear: parseIntDefault(get("subsidy_deadline_year"), 0),
	}
	return rec, nil
}

func parseFloat(s string) float64 {
	if s == "" {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

func parseInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
