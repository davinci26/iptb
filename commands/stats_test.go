package commands

import (
	"testing"
)

func TestStats(t *testing.T) {

	for _, c := range []struct {
		in  []float64
		out Stats
	}{
		{[]float64{5, 3, 4, 2, 1}, Stats{Mean: 3.,
			Std:       1.4142135623730951,
			Max:       5.,
			Quantile3: 4.,
			Median:    3.,
			Quantile1: 2.0,
			Min:       1.0}},
		{[]float64{6, 3, 2, 4, 5, 1}, Stats{Mean: 3.5,
			Std:       1.707825127659933,
			Max:       6.,
			Quantile3: 4.5,
			Median:    3.5,
			Quantile1: 2.5,
			Min:       1.}},
		{[]float64{1}, Stats{Mean: 1.,
			Std:       0.,
			Max:       1.,
			Quantile3: 1.,
			Median:    1.,
			Quantile1: 1.,
			Min:       1.}},
	} {
		got, _ := BuildStats(c.in)
		if got != c.out {
			t.Errorf("Median(%.1f) => %.1f != %.1f", c.in, got, c.out)
		}
	}
	_, err := BuildStats([]float64{})
	if err == nil {
		t.Errorf("Results are empty")
	}
}
