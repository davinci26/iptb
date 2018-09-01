package commands

import (
	"math"
	"sort"

	"github.com/pkg/errors"
)

// Stats is a struct that specifies the generated stats
// Quantiles are calculated with Midpoint interpolation
type Stats struct {
	Mean      float64
	Std       float64
	Max       float64
	Quantile3 float64
	Median    float64
	Quantile1 float64
	Min       float64
}

//BuildStats generates a stats struct from an input float64 slice
// It returns an error if the slice is empty
func BuildStats(results []float64) (Stats, error) {
	if len(results) == 0 {
		return Stats{}, errors.New("Results are empty")
	}

	// Calculate the min, max mean, std with a single pass of the array
	sum := 0.
	sumSquarred := 0.
	min := results[0]
	max := results[0]

	for _, rs := range results {
		sum += rs
		sumSquarred += rs * rs
		if rs < min {
			min = rs
		}
		if rs > max {
			max = rs
		}
	}

	mean := sum / float64(len(results))
	variance := sumSquarred/float64(len(results)) - mean*mean

	q1, median, q3 := quantiles(results)

	return Stats{
		Mean:      mean,
		Std:       math.Sqrt(variance),
		Max:       max,
		Quantile3: q3,
		Median:    median,
		Quantile1: q1,
		Min:       min,
	}, nil

}

// Quantile is calculated based on Midpoint interpolation
func quantiles(inputArray []float64) (float64, float64, float64) {

	if len(inputArray) == 0 {
		return 0., 0., 0.
	}
	// Create a copy of the input
	tmp := make([]float64, len(inputArray))
	copy(tmp, inputArray)
	// Sort the copied array to calculate the quartiles
	sort.Float64s(tmp)
	// Make calculations
	length := len(tmp)
	// Split the array into quartiles and
	// calculate the median for each quartile
	c1 := 0
	c2 := 0
	if length%2 == 0 {
		c1 = length/2 + 1
		c2 = length/2 - 1
	} else {
		c1 = (length-1)/2 + 1
		c2 = c1 - 1
	}
	return median(tmp[:c1]), median(tmp), median(tmp[c2:])
}

// Calculate the median of an array
// Critical this function assumes the input is sorted
func median(sortedInputArray []float64) float64 {
	med := 0.
	length := len(sortedInputArray)
	if length%2 == 0 {
		med = (sortedInputArray[length/2-1] + sortedInputArray[length/2]) / 2
	} else {
		med = sortedInputArray[length/2]
	}
	return med
}
