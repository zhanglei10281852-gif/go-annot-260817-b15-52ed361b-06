// Package numutil provides deterministic numeric helpers used across the
// simulation pipeline. All functions are pure and allocate only where needed.
package numutil

import (
	"math"
	"sort"
)

// Mean returns the arithmetic mean of xs, or NaN for an empty slice.
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// Median returns the 50th percentile of xs using linear interpolation.
// It does not mutate the input. Returns NaN for an empty slice.
func Median(xs []float64) float64 {
	return Percentile(xs, 50)
}

// Percentile returns the p-th percentile (0 <= p <= 100) of xs using linear
// interpolation between closest ranks. The input is not mutated.
func Percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	if p <= 0 {
		return minVal(xs)
	}
	if p >= 100 {
		return maxVal(xs)
	}
	s := SortedCopy(xs)
	rank := (p / 100) * float64(len(s)-1)
	lo := math.Floor(rank)
	hi := math.Ceil(rank)
	if lo == hi {
		return s[int(rank)]
	}
	frac := rank - lo
	return s[int(lo)] + frac*(s[int(hi)]-s[int(lo)])
}

// SortedCopy returns a sorted copy of xs.
func SortedCopy(xs []float64) []float64 {
	s := make([]float64, len(xs))
	copy(s, xs)
	sort.Float64s(s)
	return s
}

// Min returns the minimum value of xs, or NaN if empty.
func Min(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	return minVal(xs)
}

// Max returns the maximum value of xs, or NaN if empty.
func Max(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	return maxVal(xs)
}

// Clip constrains x to the closed interval [lo, hi].
func Clip(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// ApproxEqual reports whether a and b are within tol of each other.
func ApproxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// MAD returns the median absolute deviation of xs about its median.
// Returns 0 for an empty slice.
func MAD(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	med := Median(xs)
	devs := make([]float64, len(xs))
	for i, x := range xs {
		devs[i] = math.Abs(x - med)
	}
	return Median(devs)
}

// FiniteValues returns the subset of xs that are finite (not NaN/Inf).
func FiniteValues(xs []float64) []float64 {
	out := make([]float64, 0, len(xs))
	for _, x := range xs {
		if !math.IsNaN(x) && !math.IsInf(x, 0) {
			out = append(out, x)
		}
	}
	return out
}

func minVal(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func maxVal(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}
