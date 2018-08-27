package commands

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

var (
	wd, _ = os.Getwd()
)

func expect(t *testing.T, a interface{}, b interface{}) {
	_, fn, line, _ := runtime.Caller(1)
	fn = strings.Replace(fn, wd+"/", "", -1)

	if !reflect.DeepEqual(a, b) {
		t.Errorf("(%s:%d) Expected %v (type %v) - Got %v (type %v)", fn, line, b, reflect.TypeOf(b), a, reflect.TypeOf(a))
	}
}

func TestParseRange(t *testing.T) {
	cases := []struct {
		input        string
		expectedList []int
		expectedErr  error
	}{
		{"0", []int{0}, nil},
		{"[0-1]", []int{0, 1}, nil},
		{"[0-5]", []int{0, 1, 2, 3, 4, 5}, nil},
		{"[4-7]", []int{4, 5, 6, 7}, nil},
		{"[0,1]", []int{0, 1}, nil},
		{"[1,4]", []int{1, 4}, nil},
		{"[1,3,5-8]", []int{1, 3, 5, 6, 7, 8}, nil},
	}

	for _, c := range cases {
		list, err := parseRange(c.input)

		expect(t, err, c.expectedErr)
		expect(t, list, c.expectedList)
	}
}

func TestValidRange(t *testing.T) {
	buildError := func(max, total int) error {
		return fmt.Errorf("Node range contains value (%d) outside of valid range [0-%d]", max, total-1)
	}

	cases := []struct {
		inputList   []int
		inputTotal  int
		expectedErr error
	}{
		{[]int{0, 1}, 2, nil},
		{[]int{0, 3}, 2, buildError(3, 2)},
	}

	for _, c := range cases {
		err := validRange(c.inputList, c.inputTotal)

		expect(t, err, c.expectedErr)
	}
}

func TestParseCommand(t *testing.T) {
	cases := []struct {
		inputArgs     []string
		inputTerm     bool
		expectedRange string
		expectedArgs  []string
	}{
		{[]string{"0", "--", "--foo", "--bar"}, false, "0", []string{"--foo", "--bar"}},
		{[]string{"--foo", "--bar"}, true, "", []string{"--foo", "--bar"}},
	}

	for _, c := range cases {
		nodeRange, args := parseCommand(c.inputArgs, c.inputTerm)

		expect(t, nodeRange, c.expectedRange)
		expect(t, args, c.expectedArgs)
	}
}

func TestParseAttrSlice(t *testing.T) {
	cases := []struct {
		inputArgs     []string
		expectedAttrs map[string]interface{}
	}{
		{[]string{}, map[string]interface{}{}},
		{[]string{"foo"}, map[string]interface{}{"foo": "true"}},
		{[]string{"foo,bar"}, map[string]interface{}{"foo": "bar"}},
		{[]string{"foo,bar,thing"}, map[string]interface{}{"foo": "bar,thing"}},
		{[]string{"foo,bar", "one,two"}, map[string]interface{}{"foo": "bar", "one": "two"}},
	}

	for _, c := range cases {
		attrs := parseAttrSlice(c.inputArgs)

		expect(t, attrs, c.expectedAttrs)
	}
}

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
		got, _ := buildStats(c.in)
		if got != c.out {
			t.Errorf("Median(%.1f) => %.1f != %.1f", c.in, got, c.out)
		}
	}
	_, err := buildStats([]float64{})
	if err == nil {
		t.Errorf("Results are empty")
	}
}
