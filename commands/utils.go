package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli"

	"github.com/ipfs/iptb/testbed/interfaces"
)

// the flag terminator stops flag parsing, but it also swallowed if its the
// first argument into a command / subcommand. To find it, we have to look
// up to the parent command.
// iptb run 0 -- ipfs id => c.Args: 0 -- ipfs id
// iptb run -- ipfs id   => c.Args: ipfs id
func isTerminatorPresent(c *cli.Context) bool {
	argsParent := c.Parent().Args().Tail()
	argsSelf := c.Args()

	ls := len(argsSelf)
	lp := len(argsParent)

	term := lp - ls - 1

	if lp > ls && argsParent[term] == "--" {
		return true
	}

	return false
}

func parseAttrSlice(attrsraw []string) map[string]string {
	attrs := make(map[string]string)
	for _, attr := range attrsraw {
		parts := strings.Split(attr, ",")

		if len(parts) == 1 {
			attrs[parts[0]] = "true"
		} else {
			attrs[parts[0]] = strings.Join(parts[1:], ",")
		}
	}

	return attrs
}

func parseCommand(args []string, terminator bool) (string, []string) {
	if terminator {
		return "", args
	}

	if len(args) == 0 {
		return "", []string{}
	}

	if len(args) == 1 {
		return args[0], []string{}
	}

	arguments := args[1:]

	if arguments[0] == "--" {
		arguments = arguments[1:]
	}

	return args[0], arguments
}

func parseRange(s string) ([]int, error) {
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		ranges := strings.Split(s[1:len(s)-1], ",")
		var out []int
		for _, r := range ranges {
			rng, err := expandDashRange(r)
			if err != nil {
				return nil, err
			}

			out = append(out, rng...)
		}
		return out, nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return nil, err
	}

	return []int{i}, nil
}

func expandDashRange(s string) ([]int, error) {
	parts := strings.Split(s, "-")
	if len(parts) == 1 {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		return []int{i}, nil
	}
	low, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, err
	}

	hi, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}

	var out []int
	for i := low; i <= hi; i++ {
		out = append(out, i)
	}
	return out, nil
}

type Result struct {
	Node        int
	Output      testbedi.Output
	Error       error
	TimeElapsed float64
}

type outputFunc func(testbedi.Core) (testbedi.Output, error)

func mapWithOutput(list []int, nodes []testbedi.Core, fn outputFunc) ([]Result, error) {
	var wg sync.WaitGroup
	var lk sync.Mutex
	results := make([]Result, len(list))

	if err := validRange(list, len(nodes)); err != nil {
		return results, err
	}

	for i, n := range list {
		wg.Add(1)
		go func(i, n int, node testbedi.Core) {
			defer wg.Done()
			start := time.Now()
			out, err := fn(node)
			elapsed := time.Since(start)
			lk.Lock()
			defer lk.Unlock()

			results[i] = Result{
				Node:        n,
				Output:      out,
				Error:       errors.Wrapf(err, "node[%d]", n),
				TimeElapsed: elapsed.Seconds(),
			}
		}(i, n, nodes[n])
	}

	wg.Wait()

	return results, nil

}

func validRange(list []int, total int) error {
	max := 0
	for _, n := range list {
		if max < n {
			max = n
		}
	}

	if max >= total {
		return fmt.Errorf("Node range contains value (%d) outside of valid range [0-%d]", max, total-1)
	}

	return nil
}

func buildReport(results []Result, command string, statsFlag bool) error {
	var errs []error

	timeElapsedArray := make([]float64, len(results))
	nodeIDArray := make([]int, len(results))

	for i, rs := range results {
		if rs.Error != nil {
			errs = append(errs, rs.Error)
		}

		if rs.Output != nil {
			fmt.Printf("node[%d] exit %d\n", rs.Node, rs.Output.ExitCode())
			if rs.Output.Error() != nil {
				fmt.Printf("%s", rs.Output.Error())
			}

			fmt.Println()

			io.Copy(os.Stdout, rs.Output.Stdout())
			io.Copy(os.Stdout, rs.Output.Stderr())

			fmt.Println()
			timeElapsedArray[i] = rs.TimeElapsed
			nodeIDArray[i] = rs.Node
		}

	}
	if statsFlag {
		stats, err := buildStats(timeElapsedArray)
		if err != nil {
			errs = append(errs, err)
		} else {
			statsJSON, _ := json.Marshal(stats)
			fmt.Printf("Executed command < %s > on %d node(s) \nTime Statistics: %s \n", command, len(results), string(statsJSON))
			printIndivudualResults(timeElapsedArray, nodeIDArray, "time")
		}
	}

	if len(errs) != 0 {
		return cli.NewMultiError(errs...)
	}

	return nil
}

func buildMetricStats(metricsBefore []string, metricsAfter []string, metric string) error {
	var errs []error

	// Substract Before from After to generate the results
	results, err := substractArrays(metricsBefore, metricsAfter)
	if err != nil {
		return err
	}
	// Create an array of node IDs
	// TODO: Make this pretty :)
	nodesID := make([]int, len(metricsAfter))

	for i := range metricsAfter {
		nodesID[i] = i
	}
	// Use the build stats function to generate the stats
	stats, err := buildStats(results)
	if err != nil {
		errs = append(errs, err)
	} else {
		statsJSON, _ := json.Marshal(stats)
		fmt.Printf("Metric collected < %s > on %d node(s) \nStatistics: %s \n", metric, len(metricsAfter), string(statsJSON))
		printIndivudualResults(results, nodesID, metric)
	}
	if len(errs) != 0 {
		return cli.NewMultiError(errs...)
	}
	return nil
}

type Stats struct {
	Mean      float64
	Std       float64
	Max       float64
	Quantile3 float64
	Median    float64
	Quantile1 float64
	Min       float64
}

func buildStats(results []float64) (Stats, error) {
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

// Print individual results to JSON format
func printIndivudualResults(nodeTime []float64, nodeID []int, metric string) {
	fmt.Printf("Analytic Results: {[")
	for i, time := range nodeTime {
		fmt.Printf("{Node: %d, %s: %f}", nodeID[i], metric, time)
		if i != len(nodeTime)-1 {
			fmt.Printf(",")
		}
	}
	fmt.Printf("]},")
}

/*
Substract_arrays
Substract y from x returns [xi-yi] for i in len(x)
If the arrays do not have the same length or
the strings are not castable to float
the function returns an error
*/
func substractArrays(lhs []string, rhs []string) ([]float64, error) {

	if len(lhs) != len(rhs) {
		return nil, fmt.Errorf("Arrays do not have the same length")
	}
	results := make([]float64, len(lhs))

	for i := range lhs {
		valLHS, err := strconv.ParseFloat(lhs[i], 64)
		if err != nil {
			return nil, err
		}
		valRHS, err := strconv.ParseFloat(rhs[i], 64)
		if err != nil {
			return nil, err
		}

		results[i] = valLHS - valRHS
	}
	return results, nil
}
