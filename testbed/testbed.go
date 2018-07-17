package testbed

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
)

type Testbed interface {
	Name() string
	Nodes() ([]testbedi.TestbedNode, error)

	RunCmdForEach(args ...string) ([]testbedi.TBOutput, error) // most errors should be in the TBOutput

	LoadNodesFromSpec(specs []*NodeSpec) ([]testbedi.TestbedNode, error)

	ReadNodeSpecs() ([]*NodeSpec, error)
	WriteNodeSpecs(specs []*NodeSpec) error

	/****************/
	/* Future Ideas */

	// Would be neat to have a TestBed Config interface
	// The node interface GetAttr and SetAttr should be a shortcut into this
	// Config() (map[interface{}]interface{}, error)

}

type testbed struct {
	dir string
}

// LoadNodesFromSpecs accepts a NodeSpec `spec` and returns interfaces of the TestBedNode type derived from `spec`
func (tb *testbed) LoadNodesFromSpecs(specs []*NodeSpec) ([]testbedi.TestbedNode, error) {
	var out []testbedi.TestbedNode
	for _, s := range specs {
		nd, err := s.Load()
		if err != nil {
			return nil, err
		}
		out = append(out, nd)
	}
	return out, nil
}

func (tb *testbed) ReadNodeSpecs() ([]*NodeSpec, error) {
	data, err := ioutil.ReadFile(filepath.Join(tb.dir, "nodespec"))
	if err != nil {
		return nil, err
	}

	var specs []*NodeSpec
	err = json.Unmarshal(data, &specs)
	if err != nil {
		return nil, err
	}

	return specs, nil
}

func (tb *testbed) WriteNodeSpecs(specs []*NodeSpec) error {
	err := os.MkdirAll(tb.dir, 0775)
	if err != nil {
		return err
	}

	fi, err := os.Create(filepath.Join(tb.dir, "nodespec"))
	if err != nil {
		return err
	}

	defer fi.Close()
	err = json.NewEncoder(fi).Encode(specs)
	if err != nil {
		return err
	}

	return nil
}

func NewTestbed() (*testbed, error) {
	tbd, err := testBedDir()
	if err != nil {
		return nil, err
	}
	return &testbed{
		dir: tbd,
	}, nil
}

func nodeDirN(n int) (string, error) {
	tbd, err := testBedDir()
	if err != nil {
		return "", err
	}
	return path.Join(tbd, fmt.Sprint(n)), nil
}

func TBNInit(cfg *InitCfg) error {
	tbd, err := testBedDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(tbd, "nodespec")); !os.IsNotExist(err) {
		if !cfg.Force && !iptbutil.YesNoPrompt("testbed nodes already exist, overwrite? [y/n]") {
			return nil
		}
		tbd, err := testBedDir()
		err = os.RemoveAll(tbd)
		if err != nil {
			return err
		}
	}

	tb, err := NewTestbed()
	if err != nil {
		return err
	}

	specs, err := initSpecs(cfg)

	nodes, err := tb.LoadNodesFromSpecs(specs)

	err = tb.WriteNodeSpecs(specs)

	wait := sync.WaitGroup{}
	for i, n := range nodes {
		wait.Add(1)
		go func(nd testbedi.TestbedNode, i int) {
			defer wait.Done()
			_, err := nd.Init(context.TODO())
			if err != nil {
				panic(err)
				return
			}
		}(n, i)
	}

	wait.Wait()

	return nil
}

func initSpecs(cfg *InitCfg) ([]*NodeSpec, error) {
	var specs []*NodeSpec

	for i := 0; i < cfg.Count; i++ {
		dir, err := nodeDirN(i)

		if err != nil {
			return nil, err
		}

		var spec *NodeSpec

		spec = &NodeSpec{
			Type:       cfg.NodeType,
			Deployment: cfg.Deployment,
			Dir:        dir,
			BinPath:    cfg.BinPath,
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

func testBedDir() (string, error) {
	tbd := os.Getenv("IPTB_ROOT")
	if len(tbd) != 0 {
		return tbd, nil
	}

	home := os.Getenv("HOME")
	if len(home) == 0 {
		return "", fmt.Errorf("environment variable HOME is not set")
	}

	return path.Join(home, "testbed"), nil
}

func (tb *testbed) LoadNodeN(n int) (testbedi.TestbedNode, error) {
	specs, err := tb.ReadNodeSpecs()
	if err != nil {
		return nil, err
	}

	return specs[n].Load()
}

func (tb *testbed) LoadNodes() ([]testbedi.TestbedNode, error) {
	specs, err := tb.ReadNodeSpecs()
	if err != nil {
		return nil, err
	}

	return tb.LoadNodesFromSpecs(specs)
}
