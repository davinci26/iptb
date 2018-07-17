package testbed

import (
	"github.com/ipfs/iptb/testbed/interfaces"
	"plugin"
)

type NodeSpec struct {
	Deployment string
	Type       string
	Dir        string
	Extra      map[string]interface{}
}

var ipfslocal *plugin.Plugin

func init() {
	var err error
	ipfslocal, err = plugin.Open("plugins/ipfs/local/local.so")
	if err != nil {
		panic(err)
	}
}

func (ns *NodeSpec) Load() (testbedi.TestbedNode, error) {
	switch ns.Type {
	case "ipfs":
		switch ns.Deployment {
		case "local":
			NewNodeRaw, err := ipfslocal.Lookup("NewNode")
			if err != nil {
				panic(err)
			}

			NewNode := NewNodeRaw.(func(string, string) testbedi.TestbedNode)

			return NewNode("/home/travis/bin/ipfs", ns.Dir), nil
		default:
			panic("unrecognized iptb node deplyomet")
		}
	default:
		panic("unrecognized iptb node type")
	}
	return nil, nil
}
