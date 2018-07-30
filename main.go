package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	util "github.com/ipfs/iptb/util"
	cli "github.com/urfave/cli"
)

type SimulationResults struct {
	Avg_time   float64
	Std_Time   float64
	Delay_Min  float64
	Delay_Max  float64
	Users      int
	Date_Time  time.Time
	Results    []float64
	DuplBlocks int
}

func (res SimulationResults) ResultsSave() {

	resultsJSON, _ := json.Marshal(res)
	f, err := os.OpenFile("results.json", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err = f.WriteString(string(resultsJSON) + "\n"); err != nil {
		panic(err)
	}
}

func getDupBlocksFromNode(n int) (int, error) {

	nd, err := util.LoadNodeN(n)
	if err != nil {
		return -1, err
	}

	bstat, err := nd.RunCmd("ipfs", "bitswap", "stat")
	if err != nil {
		return -1, err
	}

	lines := strings.Split(bstat, "\n")
	for _, l := range lines {
		if strings.Contains(l, "dup blocks") {
			fs := strings.Fields(l)
			n, err := strconv.Atoi(fs[len(fs)-1])
			if err != nil {
				return -1, err
			}

			return int(n), nil
		}
	}

	return -1, fmt.Errorf("no dup blocks field in output")
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
		connectCmd,
		dumpStacksCmd,
		forEachCmd,
		getCmd,
		initCmd,
		killCmd,
		restartCmd,
		setCmd,
		shellCmd,
		startCmd,
		runCmd,
		connGraphCmd,
		distCmd,
		logsCmd,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var initCmd = cli.Command{
	Name:  "init",
	Usage: "create and initialize testbed nodes",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "count, n",
			Usage: "number of ipfs nodes to initialize",
		},
		cli.IntFlag{
			Name:  "port, p",
			Usage: "port to start allocations from",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force initialization (overwrite existing configs)",
		},
		cli.BoolFlag{
			Name:  "mdns",
			Usage: "turn on mdns for nodes",
		},
		cli.StringFlag{
			Name:  "bootstrap",
			Usage: "select bootstrapping style for cluster",
			Value: "star",
		},
		cli.BoolFlag{
			Name:  "utp",
			Usage: "use utp for addresses",
		},
		cli.BoolFlag{
			Name:  "ws",
			Usage: "use websocket for addresses",
		},
		cli.StringFlag{
			Name:  "cfg",
			Usage: "override default config with values from the given file",
		},
		cli.StringFlag{
			Name:  "type",
			Usage: "select type of nodes to initialize",
		},
	},
	Action: func(c *cli.Context) error {
		if c.Int("count") == 0 {
			fmt.Printf("please specify number of nodes: '%s init -n 10'\n", os.Args[0])
			os.Exit(1)
		}
		fmt.Println("Initializing users...")
		cfg := &util.InitCfg{
			Bootstrap: c.String("bootstrap"),
			Force:     c.Bool("f"),
			Count:     c.Int("count"),
			Mdns:      c.Bool("mdns"),
			Utp:       c.Bool("utp"),
			Websocket: c.Bool("ws"),
			PortStart: c.Int("port"),
			Override:  c.String("cfg"),
			NodeType:  c.String("type"),
		}

		err := util.IpfsInit(cfg)
		handleErr("ipfs init err: ", err)
		return nil
	},
}

var startCmd = cli.Command{
	Name:  "start",
	Usage: "starts up all testbed nodes",
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
		fmt.Print("Starting...")
		if len(args) > 0 {
			extra = strings.Fields(args)
		}

		if c.Args().Present() {
			nodes, err := parseRange(c.Args()[0])
			if err != nil {
				return err
			}

			for _, n := range nodes {
				nd, err := util.LoadNodeN(n)
				if err != nil {
					return fmt.Errorf("failed to load local node: %s\n", err)
				}

				err = nd.Start(extra)
				if err != nil {
					fmt.Println("failed to start node: ", err)
				}
			}
			return nil
		}

		nodes, err := util.LoadNodes()
		if err != nil {
			return err
		}
		return util.IpfsStart(nodes, c.Bool("wait"), extra)
	},
}

var killCmd = cli.Command{
	Name:    "kill",
	Usage:   "kill a given node (or all nodes if none specified)",
	Aliases: []string{"stop"},
	Action: func(c *cli.Context) error {
		if c.Args().Present() {
			nodes, err := parseRange(c.Args()[0])
			if err != nil {
				return fmt.Errorf("failed to parse node number: %s", err)
			}

			for _, n := range nodes {
				nd, err := util.LoadNodeN(n)
				if err != nil {
					return fmt.Errorf("failed to load local node: %s\n", err)
				}

				err = nd.Kill()
				if err != nil {
					fmt.Println("failed to kill node: ", err)
				}
			}
			return nil
		}
		nodes, err := util.LoadNodes()
		if err != nil {
			return err
		}

		err = util.IpfsKillAll(nodes)
		handleErr("ipfs kill err: ", err)
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
		if c.Args().Present() {
			nodes, err := parseRange(c.Args()[0])
			if err != nil {
				return err
			}

			for _, n := range nodes {
				nd, err := util.LoadNodeN(n)
				if err != nil {
					return fmt.Errorf("failed to load local node: %s\n", err)
				}

				err = nd.Kill()
				if err != nil {
					fmt.Println("restart: failed to kill node: ", err)
				}

				err = nd.Start(nil)
				if err != nil {
					fmt.Println("restart: failed to start node again: ", err)
				}
			}
			return nil
		}
		nodes, err := util.LoadNodes()
		if err != nil {
			return err
		}

		err = util.IpfsKillAll(nodes)
		if err != nil {
			return fmt.Errorf("ipfs kill err: %s", err)
		}

		err = util.IpfsStart(nodes, c.Bool("wait"), nil)
		handleErr("ipfs start err: ", err)
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

		n, err := util.LoadNodeN(i)
		if err != nil {
			return err
		}
		err = n.Shell()
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

		nodes, err := util.LoadNodes()
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

		timeout := c.String("timeout")

		for _, f := range from {
			for _, t := range to {
				err = util.ConnectNodes(nodes[f], nodes[t], timeout)
				if err != nil {
					return fmt.Errorf("failed to connect: %s", err)
				}
			}
		}
		return nil
	},
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "timeout",
			Usage: "timeout on the command",
		},
	},
}

var getCmd = cli.Command{
	Name:  "get",
	Usage: "get an attribute of the given node",
	Description: `Given an attribute name and a node number, prints the value of the attribute for the given node.

You can get the list of valid attributes by passing no arguments.`,
	Action: func(c *cli.Context) error {
		showUsage := func(w io.Writer) {
			fmt.Fprintln(w, "iptb get [attr] [node]")
			fmt.Fprintln(w, "Valid values of [attr] are:")
			attr_list := util.GetListOfAttr()
			for _, a := range attr_list {
				desc, err := util.GetAttrDescr(a)
				handleErr("error getting attribute description: ", err)
				fmt.Fprintf(w, "\t%s: %s\n", a, desc)
			}
		}
		switch len(c.Args()) {
		case 0:
			showUsage(os.Stdout)
		case 2:
			attr := c.Args().First()
			num, err := strconv.Atoi(c.Args()[1])
			handleErr("error parsing node number: ", err)

			ln, err := util.LoadNodeN(num)
			if err != nil {
				return err
			}

			val, err := ln.GetAttr(attr)
			handleErr("error getting attribute: ", err)
			fmt.Println(val)
		default:
			fmt.Fprintln(os.Stderr, "'iptb get' accepts exactly 0 or 2 arguments")
			showUsage(os.Stderr)
			os.Exit(1)
		}
		return nil
	},
}

var setCmd = cli.Command{
	Name:  "set",
	Usage: "set an attribute of the given node",
	Action: func(c *cli.Context) error {
		switch len(c.Args()) {
		case 3:
			attr := c.Args().First()
			val := c.Args()[1]
			nodes, err := parseRange(c.Args()[2])
			handleErr("error parsing node number: ", err)

			for _, i := range nodes {
				ln, err := util.LoadNodeN(i)
				if err != nil {
					return err
				}

				err = ln.SetAttr(attr, val)
				if err != nil {
					return fmt.Errorf("error setting attribute: %s", err)
				}
			}
		default:
			fmt.Fprintln(os.Stderr, "'iptb set' accepts exactly 3 arguments")
			os.Exit(1)
		}
		return nil
	},
}

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

var forEachCmd = cli.Command{
	Name:            "for-each",
	Usage:           "run a given command on each node",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		nodes, err := util.LoadNodes()
		if err != nil {
			return err
		}

		for _, n := range nodes {
			out, err := n.RunCmd(c.Args()...)
			if err != nil {
				return err
			}
			fmt.Print(out)
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

		nd, err := util.LoadNodeN(n)
		if err != nil {
			return err
		}

		out, err := nd.RunCmd(c.Args()[1:]...)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

var connGraphCmd = cli.Command{
	Name:        "make-topology",
	Usage:       "Connect nodes according to the connection graph",
	Description: "Connect all nodes according to specified network topology",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "input-topology",
			Usage: "Specify connection graph, if none is specified a star topology will be used with center node 0",
		},
	},
	Action: func(c *cli.Context) error {
		// Load all nodes
		nodes, err := util.LoadNodes()
		if err != nil {
			return err
		}
		graphDir := c.String("input-topology")
		// If no input topology is given make default connection and move on
		if len(graphDir) == 0 {
			fmt.Println("No connection graph is specified, creating default star topology")
			for i := 1; i < len(nodes); i++ {
				err = util.ConnectNodes(nodes[0], nodes[i], "")
				if err != nil {
					return err
				}
			}
			return nil
		}
		// If input topology is given parse and construct it
		// Scan Input file Line by Line //
		inFile, err := os.Open(graphDir)
		defer inFile.Close()
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(inFile)
		scanner.Split(bufio.ScanLines)
		lineNumber := 1

		for scanner.Scan() {
			var destinations []string
			var lineTokenized []string
			line := scanner.Text()
			// Check if the line is a comment or empty and skip it//
			if len(line) == 0 || line[0] == '#' {
				lineNumber++
				continue
			} else {
				lineTokenized = strings.Split(line, ":")
				// Check if the format is correct
				if len(lineTokenized) == 1 {
					return errors.New("Line " + strconv.Itoa(lineNumber) + " does not follow the correct format")
				}
				destinations = strings.Split(lineTokenized[1], ",")
			}
			// Parse origin in the line
			origin, err := strconv.Atoi(lineTokenized[0])
			// Check if it can be casted to integer
			if err != nil {
				return errors.New("Line: " + strconv.Itoa(lineNumber) + " of connection graph, could not be parsed")
			}
			// Check if the node is out of range
			if origin >= len(nodes) {
				return errors.New("Node origin in line: " + strconv.Itoa(lineNumber) + " out of range")
			}

			for _, destination := range destinations {
				// Check if it can be casted to integer
				target, err := strconv.Atoi(destination)
				if err != nil {
					return errors.New("Check line: " + strconv.Itoa(lineNumber) + " of connection graph, could not be parsed")
				}
				// Check if the node is out of range
				if target >= len(nodes) {
					return errors.New("Node target in line: " + strconv.Itoa(lineNumber) + " out of range")
				}
				// Establish the connection
				err = util.ConnectNodes(nodes[origin], nodes[target], "")
				if err != nil {
					fmt.Println("Connection failed!!!!")
					return err
				}
			}
			lineNumber++
		}
		return nil
	},
}

var distCmd = cli.Command{
	Name:        "dist",
	Usage:       "distribute a file to all other nodes",
	Description: "Distribute a single file to all nodes and calculate the statistics",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "hash",
			Usage: "Hash of the file",
		},
	},
	Action: func(c *cli.Context) error {
		// Get the file hash and check if its correct
		hash := c.String("hash")
		if len(hash) == 0 {
			return errors.New("No file hash is specified")
		}
		// Load all Nodes
		nodes, err := util.LoadNodes()
		if err != nil {
			return err
		}
		// Make a Happy print statement
		fmt.Printf("=========== Simulation Begins ================ \n")

		// Create channels to start asynchronous requests
		ch := make(chan float64)
		errorCh := make(chan error)
		// Create an asynchronous request from each node
		for i := 1; i < len(nodes); i++ {
			fmt.Printf("Downloading file: %d / %d \n", i, len(nodes)-1)
			go util.GetFile(hash, nodes[i], ch, errorCh)
		}

		// Parse results
		var delay []float64
		duplBlocks := 0
		for i := 1; i < len(nodes); i++ {
			val := <-ch
			err := <-errorCh
			if err != nil {
				return err
			}
			// Get delay and duplicate blocks//
			delay = append(delay, val)
			dB, err := getDupBlocksFromNode(i)
			if err != nil {
				fmt.Println("Failed to Parse duplicate blocks")
				return err
			}
			duplBlocks += dB
		}

		// Calculate Average Delay
		var sum float64
		DelayMin := delay[0]
		DelayMax := delay[0]
		for _, d := range delay {
			sum += d
			// Calculate Min
			if d < DelayMin {
				DelayMin = d
			}
			// Calculate Max
			if d > DelayMax {
				DelayMax = d
			}
		}
		avg := sum / float64(len(delay))
		// Calculate Delay Std
		var sumSq float64
		for _, f := range delay {
			sumSq += math.Pow((avg - f), 2)
		}
		std := math.Sqrt(sumSq / float64(len(delay)))
		fmt.Printf("Average Time to distribute file to all nodes: %.4f\nStd Time to distribute file to all nodes %.4f\nDuplicate Blocks: %d\n", avg, std, duplBlocks)
		// Save results to file
		res := SimulationResults{avg, std, DelayMin, DelayMax, len(nodes) - 1, time.Now().UTC(), delay, duplBlocks}
		res.ResultsSave()
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
		var nodes []util.IpfsNode
		var err error

		if c.Args()[0] == "*" {
			nodes, err = util.LoadNodes()
			if err != nil {
				return err
			}
		} else {
			for _, is := range c.Args() {
				i, err := strconv.Atoi(is)
				if err != nil {
					return err
				}
				n, err := util.LoadNodeN(i)
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
			n, ok := ns.(*util.LocalNode)
			if !ok {
				return errors.New("logs are supported only with local nodes")
			}
			if stdout {
				if !silent {
					fmt.Printf(">>>> %s", n.Dir)
					fmt.Println("/daemon.stdout")
				}
				st, err := n.StderrReader()
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
					fmt.Printf(">>>> %s", n.Dir)
					fmt.Println("/daemon.stderr")
				}
				st, err := n.StderrReader()
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
