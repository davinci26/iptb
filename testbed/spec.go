package iptbutil

import ()

type NodeSpec struct {
	Deployment Deployment
	Type       string
	Dir        string
	Extra      map[string]interface{}
}

func (ns *NodeSpec) Load() (TestbedNode, error) {
	switch ns.Type {
	case "ipfs":
		switch ns.Deployment {
		case LOCAL:
		default:
			panic("unrecognized iptb node deplyomet")
		}
	default:
		panic("unrecognized iptb node type")
	}
	return nil, nil
}
