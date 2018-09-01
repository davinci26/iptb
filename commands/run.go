package commands

import (
	"context"
	"fmt"
	"path"
	"strings"

	cli "github.com/urfave/cli"

	"github.com/ipfs/iptb/testbed"
	"github.com/ipfs/iptb/testbed/interfaces"
)

var RunCmd = cli.Command{
	Category:  "CORE",
	Name:      "run",
	Usage:     "run command on specified nodes (or all)",
	ArgsUsage: "[nodes] -- <command...>",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:   "terminator",
			Hidden: true,
		},
		cli.BoolFlag{
			Name:  "time",
			Usage: "Output statistics on the command execution",
		},
		cli.StringSliceFlag{
			Name:  "collect,c",
			Usage: "Collect a list of specified metrics, Note that you can use the flag multiple times to collect multiple metrics",
		},
	},
	Before: func(c *cli.Context) error {
		if present := isTerminatorPresent(c); present {
			return c.Set("terminator", "true")
		}

		return nil
	},
	Action: func(c *cli.Context) error {
		flagRoot := c.GlobalString("IPTB_ROOT")
		flagTestbed := c.GlobalString("testbed")
		flagStats := c.Bool("time")
		collectFlag := c.StringSlice("collect")

		tb := testbed.NewTestbed(path.Join(flagRoot, "testbeds", flagTestbed))
		nodes, err := tb.Nodes()
		if err != nil {
			return err
		}

		nodeRange, args := parseCommand(c.Args(), c.IsSet("terminator"))

		if nodeRange == "" {
			nodeRange = fmt.Sprintf("[0-%d]", len(nodes)-1)
		}

		list, err := parseRange(nodeRange)
		if err != nil {
			return fmt.Errorf("could not parse node range %s", nodeRange)
		}

		runCmd := func(node testbedi.Core) (testbedi.Output, error) {
			return node.RunCmd(context.Background(), nil, args...)
		}

		// Create a list of list of the specified metrics by the user
		var metricsBefore [][]string
		if len(collectFlag) != 0 {
			for _, metric := range collectFlag {
				// Calculate the values of the metric before the command execution
				tmp, err := collectMetric(nodes, metric)
				if err != nil {
					return err
				}
				metricsBefore = append(metricsBefore, tmp)
			}
		}

		results, err := mapWithOutput(list, nodes, runCmd)
		if err != nil {
			return err
		}

		if len(collectFlag) != 0 {
			for i, metric := range collectFlag {
				// Calculate the values of the metric after the command execution
				tmpMetricsAfter, err := collectMetric(nodes, metric)
				if err != nil {
					return err
				}
				buildMetricStats(metricsBefore[i], tmpMetricsAfter, metric)
			}
		}
		return buildReport(results, strings.Join(args[:], " "), flagStats)
	},
}

func collectMetric(nodes []testbedi.Core, metric string) ([]string, error) {
	nodeMetricValue := make([]string, len(nodes))
	for i, node := range nodes {
		metricNode, ok := node.(testbedi.Metric)
		if !ok {
			return nil, fmt.Errorf("node %d does not implement metrics", i)
		}
		value, err := metricNode.Metric(metric)
		if err != nil {
			return nil, err
		}

		nodeMetricValue[i] = value
	}
	return nodeMetricValue, nil
}
