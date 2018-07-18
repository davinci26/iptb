package main

import (
	"fmt"
	"io"

	"context"
	"os"
	"strconv"
	"strings"
	"time"

	util "github.com/ipfs/iptb/testbed"
	"github.com/ipfs/iptb/testbed/interfaces"

	cli "github.com/urfave/cli"
)

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
	} else {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}

		return []int{i}, nil
	}
}

func expandDashRange(s string) ([]int, error) {
	parts := strings.Split(s, "-")
	if len(parts) == 0 {
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

func handleErr(s string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, s, err)
		os.Exit(1)
	}
}

func main() {
	app := cli.NewApp()
	app.Usage = "iptb is a tool for managing test clusters of ipfs nodes"
	app.Commands = []cli.Command{
		initCmd,
		createNodespecCmd,
		initNodespecCmd,

		startCmd,
		killCmd,
		restartCmd,

		connectCmd,

		attrGetCmd,
		attrSetCmd,
		attrListCmd,

		logsCmd,
		eventsCmd,

		runCmd,
		shellCmd,
		forEachCmd,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func extrasToMap(extrasraw []string) (map[string]interface{}, error) {
	extras := make(map[string]interface{})
	for _, extra := range extrasraw {
		parts := strings.Split(extra, ",")

		if len(parts) == 1 {
			extras[parts[0]] = "true"
		} else {
			extras[parts[0]] = strings.Join(parts[1:], ",")
		}
	}

	return extras, nil
}

var createNodespecCmd = cli.Command{
	Name:  "create-nodespec",
	Usage: "generate testbed nodes layout file",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "count, n",
			Usage: "number of ipfs nodes to initialize",
		},
		cli.StringFlag{
			Name:  "type",
			Usage: "select type of nodes to initialize",
		},
		cli.StringFlag{
			Name:  "deployment",
			Usage: "how to deploy node (local)",
		},
		cli.StringFlag{
			Name:  "bin",
			Usage: "path to the binary",
		},
		cli.StringSliceFlag{
			Name:  "extra",
			Usage: "specify addition information for the spec",
		},
	},
	Action: func(c *cli.Context) error {
		if c.Int("count") == 0 {
			fmt.Printf("please specify number of nodes: '%s init -n 10'\n", os.Args[0])
			os.Exit(1)
		}

		if len(c.String("type")) == 0 {
			fmt.Printf("please specify a type: '%s init -type ipfs'\n", os.Args[0])
			os.Exit(1)
		}

		if len(c.String("deployment")) == 0 {
			fmt.Printf("please specify a deployment: '%s init -deployment local'\n", os.Args[0])
			os.Exit(1)
		}

		extrasraw := c.StringSlice("extra")
		extras, err := extrasToMap(extrasraw)
		if err != nil {
			return err
		}

		// Setup testbed
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		// Check if already init
		if err := tb.AlreadyInitCheck(false); err != nil {
			return err
		}

		// Initalize specs
		// InitSpecs(count, type, deployment, etc)
		specs, err := tb.InitSpecs(c.Int("count"), c.String("type"), c.String("deployment"), extras)
		if err != nil {
			return err
		}

		// Write specs out to testbed
		// WriteNodeSpecs(specs)
		if err := tb.WriteNodeSpecs(specs); err != nil {
			return err
		}

		return nil
	},
}

var initNodespecCmd = cli.Command{
	Name:  "init-nodespec",
	Usage: "create and initialize testbed nodes",
	Flags: []cli.Flag{},
	Action: func(c *cli.Context) error {
		// Setup testbed
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		specs, err := tb.ReadNodeSpecs()
		if err != nil {
			return err
		}

		// Create nodes from specs
		// NodesFromSpecs(specs)
		nodes, err := tb.NodesFromSpecs(specs)
		if err != nil {
			return err
		}

		// Run node.Init() for all nodes
		// InitNodes()
		return tb.InitNodes(nodes)
	},
}

var initCmd = cli.Command{
	Name:  "init",
	Usage: "create and initialize testbed nodes",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "count, n",
			Usage: "number of ipfs nodes to initialize",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force initialization (overwrite existing configs)",
		},
		cli.StringFlag{
			Name:  "type",
			Usage: "select type of nodes to initialize",
		},
		cli.StringFlag{
			Name:  "deployment",
			Usage: "how to deploy node (local)",
		},
		cli.StringSliceFlag{
			Name:  "extra",
			Usage: "specify addition information for the spec",
		},
	},
	Action: func(c *cli.Context) error {
		if c.Int("count") == 0 {
			fmt.Printf("please specify number of nodes: '%s init -n 10'\n", os.Args[0])
			os.Exit(1)
		}

		if len(c.String("type")) == 0 {
			fmt.Printf("please specify a type: '%s init -type ipfs'\n", os.Args[0])
			os.Exit(1)
		}

		if len(c.String("deployment")) == 0 {
			fmt.Printf("please specify a deployment: '%s init -deployment local'\n", os.Args[0])
			os.Exit(1)
		}

		extrasraw := c.StringSlice("extra")
		extras, err := extrasToMap(extrasraw)
		if err != nil {
			return err
		}

		// Setup testbed
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		// Check if already init
		if err := tb.AlreadyInitCheck(c.Bool("f")); err != nil {
			return err
		}

		// Initalize specs
		// InitSpecs(count, type, deployment, etc)
		specs, err := tb.InitSpecs(c.Int("count"), c.String("type"), c.String("deployment"), extras)
		if err != nil {
			return err
		}

		// Create nodes from specs
		// NodesFromSpecs(specs)
		nodes, err := tb.NodesFromSpecs(specs)
		if err != nil {
			return err
		}

		// Write specs out to testbed
		// WriteNodeSpecs(specs)
		if err := tb.WriteNodeSpecs(specs); err != nil {
			return err
		}

		// Run node.Init() for all nodes
		// InitNodes()
		return tb.InitNodes(nodes)
	},
}

var startCmd = cli.Command{
	Name:  "start",
	Usage: "starts up specified testbed nodes",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "wait",
			Usage: "wait for nodes to fully come online before returning",
		},
		cli.StringFlag{
			Name:  "args",
			Usage: "extra args to pass on to the ipfs daemon",
		},
	},
	Action: func(c *cli.Context) error {
		var extra []string
		args := c.String("args")
		if len(args) > 0 {
			extra = strings.Fields(args)
		}

		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		var nodes []testbedi.TestbedNode
		if c.Args().Present() {
			ndlist, err := parseRange(c.Args()[0])
			if err != nil {
				return err
			}

			for _, n := range ndlist {
				nd, err := tb.LoadNode(n)
				if err != nil {
					return fmt.Errorf("failed to load local node: %s\n", err)
				}

				nodes = append(nodes, nd)
			}
		} else {
			var err error

			nodes, err = tb.LoadNodes()
			if err != nil {
				return err
			}
		}

		for _, n := range nodes {
			_, err := n.Start(context.TODO(), extra...)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

var killCmd = cli.Command{
	Name:    "kill",
	Usage:   "kill a given node (or all nodes if none specified)",
	Aliases: []string{"stop"},
	Action: func(c *cli.Context) error {
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		var nodes []testbedi.TestbedNode
		if c.Args().Present() {
			ndlist, err := parseRange(c.Args()[0])
			if err != nil {
				return err
			}

			for _, n := range ndlist {
				nd, err := tb.LoadNode(n)
				if err != nil {
					return fmt.Errorf("failed to load local node: %s\n", err)
				}

				nodes = append(nodes, nd)
			}
		} else {
			var err error

			nodes, err = tb.LoadNodes()
			if err != nil {
				return err
			}
		}

		for _, n := range nodes {
			err := n.Stop(context.TODO(), false)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

var restartCmd = cli.Command{
	Name:  "restart",
	Usage: "kill all nodes, then restart",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "wait",
			Usage: "wait for nodes to come online before returning",
		},
	},
	Action: func(c *cli.Context) error {
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		var nodes []testbedi.TestbedNode
		if c.Args().Present() {
			ndlist, err := parseRange(c.Args()[0])
			if err != nil {
				return err
			}

			for _, n := range ndlist {
				nd, err := tb.LoadNode(n)
				if err != nil {
					return fmt.Errorf("failed to load local node: %s\n", err)
				}

				nodes = append(nodes, nd)
			}
		} else {
			var err error

			nodes, err = tb.LoadNodes()
			if err != nil {
				return err
			}
		}

		for _, n := range nodes {
			err := n.Stop(context.TODO(), false)
			if err != nil {
				return err
			}
			_, err = n.Start(context.TODO())
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var shellCmd = cli.Command{
	Name:  "shell",
	Usage: "execs your shell with certain environment variables set",
	Description: `Starts a new shell and sets some environment variables for you:

IPFS_PATH - set to testbed node 'n's IPFS_PATH
NODE[x] - set to the peer ID of node x
`,
	Action: func(c *cli.Context) error {
		if !c.Args().Present() {
			fmt.Println("please specify which node you want a shell for")
			os.Exit(1)
		}
		i, err := strconv.Atoi(c.Args()[0])
		if err != nil {
			return fmt.Errorf("parse err: %s", err)
		}

		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		n, err := tb.LoadNode(i)
		if err != nil {
			return err
		}

		err = n.Shell(context.TODO())
		handleErr("ipfs shell err: ", err)
		return nil
	},
}

var connectCmd = cli.Command{
	Name:  "connect",
	Usage: "connect two nodes together",
	Action: func(c *cli.Context) error {
		if len(c.Args()) < 2 {
			fmt.Println("iptb connect [node] [node]")
			os.Exit(1)
		}

		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		nodes, err := tb.LoadNodes()
		if err != nil {
			return err
		}

		from, err := parseRange(c.Args()[0])
		if err != nil {
			return fmt.Errorf("failed to parse: %s", err)
		}

		to, err := parseRange(c.Args()[1])
		if err != nil {
			return fmt.Errorf("failed to parse: %s", err)
		}

		timeout := c.Uint64("timeout")

		for _, f := range from {
			for _, t := range to {
				err = nodes[f].Connect(context.TODO(), nodes[t], time.Duration(timeout))
				if err != nil {
					return fmt.Errorf("failed to connect: %s", err)
				}
			}
		}
		return nil
	},
	Flags: []cli.Flag{
		cli.Uint64Flag{
			Name:  "timeout",
			Usage: "timeout on the command",
		},
	},
}

var attrListCmd = cli.Command{
	Name:  "attr-list",
	Usage: "list attrs for a given type and deployment",
	Action: func(c *cli.Context) error {
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		showUsage := func(w io.Writer) {
			fmt.Fprintln(w, "iptb attr list [node]")
		}

		switch len(c.Args()) {
		case 1:
			num, err := strconv.Atoi(c.Args().First())
			handleErr("error parsing node number: ", err)

			ln, err := tb.LoadNode(num)
			if err != nil {
				return err
			}

			attr_list := ln.GetAttrList()
			for _, a := range attr_list {
				desc, err := ln.GetAttrDesc(a)
				handleErr("error getting attribute description: ", err)
				fmt.Printf("\t%s: %s\n", a, desc)
			}
		default:
			fmt.Fprintln(os.Stderr, "'iptb get' accepts exactly 1 argument")
			showUsage(os.Stderr)
			os.Exit(1)
		}
		return nil
	},
}

var attrGetCmd = cli.Command{
	Name:        "attr-get",
	Usage:       "get an attribute of the given node",
	Description: `Given an attribute name and a node number, prints the value of the attribute for the given node.`,
	Action: func(c *cli.Context) error {
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		showUsage := func(w io.Writer) {
			fmt.Fprintln(w, "iptb attr-get [attr] [node]")
			fmt.Fprintln(w, "Use iptb attr-list [node] to see a list")
		}

		switch len(c.Args()) {
		case 2:
			attr := c.Args().First()
			num, err := strconv.Atoi(c.Args()[1])
			handleErr("error parsing node number: ", err)

			ln, err := tb.LoadNode(num)
			if err != nil {
				return err
			}

			val, err := ln.GetAttr(attr)
			handleErr("error getting attribute: ", err)
			fmt.Println(val)
		default:
			fmt.Fprintln(os.Stderr, "'iptb-get' accepts exactly 2 arguments")
			showUsage(os.Stderr)
			os.Exit(1)
		}
		return nil
	},
}

var attrSetCmd = cli.Command{
	Name:  "attr-set",
	Usage: "set an attribute of the given node",
	Action: func(c *cli.Context) error {
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		switch len(c.Args()) {
		case 3:
			attr := c.Args().First()
			val := c.Args()[1]
			nodes, err := parseRange(c.Args()[2])
			handleErr("error parsing node number: ", err)

			for _, i := range nodes {
				ln, err := tb.LoadNode(i)
				if err != nil {
					return err
				}

				err = ln.SetAttr(attr, val)
				if err != nil {
					return fmt.Errorf("error setting attribute: %s", err)
				}
			}
		default:
			fmt.Fprintln(os.Stderr, "'iptb attr-set' accepts exactly 3 arguments")
			os.Exit(1)
		}
		return nil
	},
}

/*
var dumpStacksCmd = cli.Command{
	Name:  "dump-stack",
	Usage: "get a stack dump from the given daemon",
	Action: func(c *cli.Context) error {
		if len(c.Args()) < 1 {
			fmt.Println("iptb dump-stack [node]")
			os.Exit(1)
		}

		num, err := strconv.Atoi(c.Args()[0])
		handleErr("error parsing node number: ", err)

		ln, err := util.LoadNodeN(num)
		if err != nil {
			return err
		}

		addr, err := ln.APIAddr()
		if err != nil {
			return fmt.Errorf("failed to get api addr: %s", err)
		}

		resp, err := http.Get("http://" + addr + "/debug/pprof/goroutine?debug=2")
		handleErr("GET stack dump failed: ", err)
		defer resp.Body.Close()

		io.Copy(os.Stdout, resp.Body)
		return nil
	},
}
*/

var forEachCmd = cli.Command{
	Name:            "for-each",
	Usage:           "run a given command on each node",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		nodes, err := tb.LoadNodes()
		if err != nil {
			return err
		}

		for _, n := range nodes {
			out, err := n.RunCmd(context.TODO(), c.Args()...)
			if err != nil {
				return err
			}
			io.Copy(os.Stdout, out.Stdout())
			io.Copy(os.Stderr, out.Stderr())
		}
		return nil
	},
}

var runCmd = cli.Command{
	Name:            "run",
	Usage:           "run a command on a given node",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		n, err := strconv.Atoi(c.Args()[0])
		if err != nil {
			return err
		}

		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		nd, err := tb.LoadNode(n)
		if err != nil {
			return err
		}

		out, err := nd.RunCmd(context.TODO(), c.Args()[1:]...)
		if err != nil {
			return err
		}

		io.Copy(os.Stdout, out.Stdout())
		io.Copy(os.Stderr, out.Stderr())

		return nil
	},
}

var logsCmd = cli.Command{
	Name:  "logs",
	Usage: "shows logs of given node(s), use '*' for all nodes",
	Flags: []cli.Flag{
		cli.BoolTFlag{
			Name:  "err",
			Usage: "show stderr stream",
		},
		cli.BoolTFlag{
			Name:  "out",
			Usage: "show stdout stream",
		},
		cli.BoolFlag{
			Name:  "s",
			Usage: "don't show additional info, just the log",
		},
	},
	Action: func(c *cli.Context) error {
		if len(c.Args()) == 0 {
			return fmt.Errorf("'iptb logs' accepts at least 1 argument")
		}

		var nodes []testbedi.TestbedNode
		var err error

		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		if c.Args()[0] == "*" {
			nodes, err = tb.LoadNodes()
			if err != nil {
				return err
			}
		} else {
			for _, is := range c.Args() {
				i, err := strconv.Atoi(is)
				if err != nil {
					return err
				}
				n, err := tb.LoadNode(i)
				if err != nil {
					return err
				}
				nodes = append(nodes, n)
			}
		}

		silent := c.Bool("s")
		stderr := c.BoolT("err")
		stdout := c.BoolT("out")

		for _, ns := range nodes {
			if stdout {
				if !silent {
					fmt.Printf(">>>> %s", ns.Dir())
					fmt.Println("/daemon.stdout")
				}
				st, err := ns.StderrReader()
				if err != nil {
					return err
				}
				io.Copy(os.Stdout, st)
				st.Close()
				if !silent {
					fmt.Println("<<<<")
				}
			}
			if stderr {
				if !silent {
					fmt.Printf(">>>> %s", ns.Dir())
					fmt.Println("/daemon.stderr")
				}
				st, err := ns.StderrReader()
				if err != nil {
					return err
				}
				io.Copy(os.Stdout, st)
				st.Close()
				if !silent {
					fmt.Println("<<<<")
				}
			}
		}

		return nil
	},
}

var eventsCmd = cli.Command{
	Name:  "events",
	Usage: "stream events from a given node",
	Action: func(c *cli.Context) error {
		if len(c.Args()) != 1 {
			return fmt.Errorf("'iptb logs' accepts at exactly 1 argument")
		}

		tb, err := util.NewTestbed()
		if err != nil {
			return err
		}

		num, err := strconv.Atoi(c.Args().First())
		if err != nil {
			return err
		}

		n, err := tb.LoadNode(num)
		if err != nil {
			return err
		}

		el, err := n.Events()
		if err != nil {
			return err
		}

		io.Copy(os.Stdout, el)

		return nil
	},
}
