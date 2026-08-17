package numutil

import (
	"math"
	"testing"
)

func TestMeanAndMedian(t *testing.T) {
	if got := Mean([]float64{1, 2, 3, 4}); got != 2.5 {
		t.Fatalf("Mean = %v, want 2.5", got)
	}
	if got := Median([]float64{3, 1, 2, 4, 5}); got != 3 {
		t.Fatalf("Median = %v, want 3", got)
	}
	if !math.IsNaN(Mean(nil)) {
		t.Fatalf("Mean(nil) should be NaN")
	}
}

func TestPercentileInterpolation(t *testing.T) {
	xs := []float64{10, 20, 30, 40}
	// p=25 -> rank 0.75 -> 10 + 0.75*(20-10) = 17.5
	if got := Percentile(xs, 25); got != 17.5 {
		t.Fatalf("P25 = %v, want 17.5", got)
	}
	if got := Percentile(xs, 0); got != 10 {
		t.Fatalf("P0 = %v, want 10", got)
	}
	if got := Percentile(xs, 100); got != 40 {
		t.Fatalf("P100 = %v, want 40", got)
	}
	// input not mutated
	if xs[0] != 10 {
		t.Fatalf("input mutated")
	}
}

func TestClipAndFinite(t *testing.T) {
	if got := Clip(5, 0, 10); got != 5 {
		t.Fatalf("Clip = %v", got)
	}
	if got := Clip(-3, 0, 10); got != 0 {
		t.Fatalf("Clip low = %v", got)
	}
	if got := Clip(15, 0, 10); got != 10 {
		t.Fatalf("Clip high = %v", got)
	}
	fv := FiniteValues([]float64{1, math.NaN(), 3, math.Inf(1), 5})
	if len(fv) != 3 || fv[2] != 5 {
		t.Fatalf("FiniteValues = %v", fv)
	}
}

func TestMAD(t *testing.T) {
	// [1,1,2,2,4] median=2, devs=[1,1,0,0,2] median=1
	got := MAD([]float64{1, 1, 2, 2, 4})
	if !ApproxEqual(got, 1, 1e-9) {
		t.Fatalf("MAD = %v, want 1", got)
	}
}
