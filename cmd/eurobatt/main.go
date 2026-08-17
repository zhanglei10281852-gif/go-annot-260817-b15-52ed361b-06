// Command eurobatt is a local, reproducible scientific-computing tool that
// simulates site-selection costs for European power-battery capacity
// investments. It parses multi-year site cost data, cleans heterogeneous units
// and missing/duplicate/anomalous records, standardizes costs per kWh, runs a
// fixed-seed Monte Carlo across electricity/subsidy scenarios, and emits
// cost-structure tables, NPV curves, subsidy-window comparisons and
// sensitivity rankings.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"eurobatt/internal/clean"
	"eurobatt/internal/cost"
	"eurobatt/internal/loader"
	"eurobatt/internal/model"
	"eurobatt/internal/monte"
	"eurobatt/internal/report"
	"eurobatt/internal/sensitivity"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "simulate":
		return runSimulate(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "-h", "--help", "help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func runSimulate(args []string) int {
	fs := flag.NewFlagSet("simulate", flag.ContinueOnError)
	in := fs.String("i", "", "input CSV path (required)")
	out := fs.String("o", "out", "output directory")
	iters := fs.Int("iterations", 4000, "Monte Carlo iterations")
	seed := fs.Uint64("seed", 20260817, "random seed (fixed for reproducibility)")
	horizon := fs.Int("horizon", 15, "analysis horizon in years")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" {
		fmt.Fprintln(os.Stderr, "missing required -i input path")
		return 2
	}
	sites, err := loadClean(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load/clean error:", err)
		return 1
	}
	if ve := clean.Validate(sites); ve != nil {
		fmt.Fprintln(os.Stderr, ve.Error())
		fmt.Fprintln(os.Stderr, "backfill the listed items and rerun; the run is aborted so results stay consistent.")
		return 2
	}
	c := model.DefaultCoefficients()
	c.Iterations = *iters
	c.Seed = *seed
	c.HorizonYears = *horizon
	bds := cost.Breakdown(sites, c)
	results, err := monte.Simulate(sites, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "simulate error:", err)
		return 1
	}
	sens := make(map[string][]model.SensitivityItem, len(sites))
	for _, s := range sites {
		sens[s.Name] = sensitivity.Analyze(s, c)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "output dir error:", err)
		return 1
	}
	if err := writeAll(*out, bds, results, sites, sens); err != nil {
		fmt.Fprintln(os.Stderr, "write error:", err)
		return 1
	}
	if err := report.WriteSummary(os.Stdout, bds, results, sites, sens); err != nil {
		fmt.Fprintln(os.Stderr, "summary error:", err)
		return 1
	}
	fmt.Println("\noutputs written to", *out)
	return 0
}

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	in := fs.String("i", "", "input CSV path (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" {
		fmt.Fprintln(os.Stderr, "missing required -i input path")
		return 2
	}
	sites, err := loadClean(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load/clean error:", err)
		return 1
	}
	if ve := clean.Validate(sites); ve != nil {
		fmt.Fprintln(os.Stderr, ve.Error())
		return 2
	}
	fmt.Printf("OK: %d sites pass completeness checks\n", len(sites))
	for _, s := range sites {
		fmt.Printf("  %s: %d years, cap=%.1f GWh, construction=%dy, deadline=%d, flags=%v\n",
			s.Name, len(s.Years), s.CapacityGWh, s.ConstructionYears, s.SubsidyDeadlineYear, s.Flags)
	}
	return 0
}

func loadClean(path string) ([]model.Site, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := loader.LoadCSV(f)
	if err != nil {
		return nil, err
	}
	return clean.Clean(raw)
}

func writeAll(dir string, bds []model.CostBreakdown, results []model.SimResult, sites []model.Site, sens map[string][]model.SensitivityItem) error {
	writers := []struct {
		name string
		fn   func(*os.File) error
	}{
		{"cost_structure.csv", func(f *os.File) error { return report.WriteCostTable(f, bds) }},
		{"npv_curves.csv", func(f *os.File) error { return report.WriteNPVCurves(f, results) }},
		{"subsidy_comparison.csv", func(f *os.File) error { return report.WriteSubsidyComparison(f, results, sites) }},
		{"sensitivity.csv", func(f *os.File) error {
			var all []model.SensitivityItem
			for _, s := range sites {
				all = append(all, sens[s.Name]...)
			}
			return report.WriteSensitivity(f, all)
		}},
	}
	for _, w := range writers {
		f, err := os.Create(filepath.Join(dir, w.name))
		if err != nil {
			return err
		}
		if err := w.fn(f); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	f, err := os.Create(filepath.Join(dir, "report.txt"))
	if err != nil {
		return err
	}
	defer f.Close()
	return report.WriteSummary(f, bds, results, sites, sens)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "eurobatt - European battery plant site-selection cost simulator")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "usage: eurobatt <command> [flags]")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  simulate   clean, validate, run Monte Carlo and write reports")
	fmt.Fprintln(w, "  validate   check completeness and print any missing-item list")
	fmt.Fprintln(w, "flags:")
	fmt.Fprintln(w, "  -i PATH        input CSV path (required)")
	fmt.Fprintln(w, "  -o DIR         output directory (simulate, default out)")
	fmt.Fprintln(w, "  -iterations N  Monte Carlo iterations (default 4000)")
	fmt.Fprintln(w, "  -seed N        random seed (default 20260817)")
	fmt.Fprintln(w, "  -horizon N     analysis horizon years (default 15)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "example:")
	fmt.Fprintln(w, "  eurobatt simulate -i examples/sample_sites.csv -o out -iterations 4000")
}
