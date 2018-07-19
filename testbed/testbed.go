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

	// Spec returns a spec for node n
	Spec(n int) (*NodeSpec, error)

	// Specs returns all specs
	Specs() (*NodeSpec, error)

	// Node returns node n, specified by spec n
	Node(n int) ([]testbedi.TestbedNode, error)

	// Node returns all nodes, specified by all specs
	Nodes() ([]testbedi.TestbedNode, error)

	/****************/
	/* Future Ideas */

	// Would be neat to have a TestBed Config interface
	// The node interface GetAttr and SetAttr should be a shortcut into this
	// Config() (map[interface{}]interface{}, error)

}

type testbed struct {
	dir   string
	specs []*NodeSpec
	nodes []testbedi.TestbedNode
}

func NewTestbed(dir string) testbed {
	return testbed{
		dir: dir,
	}
}

func (tb *testbed) Dir() string {
	return tb.dir
}

func (tb *testbed) Name() string {
	return tb.dir
}

func AlreadyInitCheck(dir string, force bool) error {
	if _, err := os.Stat(filepath.Join(dir, "nodespec")); !os.IsNotExist(err) {
		if !force && !iptbutil.YesNoPrompt("testbed nodes already exist, overwrite? [y/n]") {
			return nil
		}

		err = os.RemoveAll(dir)
		if err != nil {
			return err
		}
	}
	return nil
}

func BuildSpecs(base string, count int, typ, deploy string, extra map[string]interface{}) ([]*NodeSpec, error) {
	var specs []*NodeSpec

	for i := 0; i < count; i++ {
		dir := path.Join(base, fmt.Sprint(i))
		err := os.MkdirAll(dir, 0775)

		if err != nil {
			return nil, err
		}

		var spec *NodeSpec

		spec = &NodeSpec{
			Type:       typ,
			Deployment: deploy,
			Dir:        dir,
			Extra:      extra,
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

func (tb *testbed) Spec(n int) (*NodeSpec, error) {
	specs, err := tb.Specs()

	if err != nil {
		return nil, err
	}

	if n >= len(specs) {
		return nil, fmt.Errorf("Spec index out of range")
	}

	return specs[n], err
}

func (tb *testbed) Specs() ([]*NodeSpec, error) {
	if tb.specs != nil {
		return tb.specs, nil
	}

	return tb.loadSpecs()
}

func (tb *testbed) Node(n int) (testbedi.TestbedNode, error) {
	nodes, err := tb.Nodes()

	if err != nil {
		return nil, err
	}

	if n >= len(nodes) {
		return nil, fmt.Errorf("Node index out of range")
	}

	return nodes[n], err
}

func (tb *testbed) Nodes() ([]testbedi.TestbedNode, error) {
	if tb.nodes != nil {
		return tb.nodes, nil
	}

	return tb.loadNodes()
}

func (tb *testbed) loadSpecs() ([]*NodeSpec, error) {
	specs, err := ReadNodeSpecs(tb.dir)
	if err != nil {
		return nil, err
	}

	return specs, nil
}

func (tb *testbed) loadNodes() ([]testbedi.TestbedNode, error) {
	specs, err := tb.Specs()
	if err != nil {
		return nil, err
	}

	return NodesFromSpecs(specs)
}

func InitNodes(nodes []testbedi.TestbedNode) error {
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

func NodesFromSpecs(specs []*NodeSpec) ([]testbedi.TestbedNode, error) {
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

func ReadNodeSpecs(dir string) ([]*NodeSpec, error) {
	data, err := ioutil.ReadFile(filepath.Join(dir, "nodespec"))
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

func WriteNodeSpecs(dir string, specs []*NodeSpec) error {
	err := os.MkdirAll(dir, 0775)
	if err != nil {
		return err
	}

	fi, err := os.Create(filepath.Join(dir, "nodespec"))
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
