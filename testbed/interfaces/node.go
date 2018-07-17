package testbedi

import (
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multiaddr"
	"time"
)

type TestbedNode interface {
	Init(agrs ...string) (TBOutput, error)
	Start(args ...string) (TBOutput, error)
	// This needs to handle killing when iptb is imported as a package and with used via cli
	Kill(wait bool) error

	RunCmd(args ...string) (TBOutput, error)
	Connect(tbn *TestbedNode, timeout time.Duration) error
	Shell() error

	String() string

	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})

	APIAddr() (*multiaddr.Multiaddr, error)
	GetPeerID() (*cid.Cid, error)

	// Don't abuse!
	// also maybe have this be a typed return
	GetAttr(string) (string, error)
	SetAttr(string, string) error

	GetConfig() (interface{}, error)
	WriteConfig(interface{}) error

	// TP, FW: Thinks this should be defined in the impl, not on an interface
	//BinPath() string

	// What does this Node Represent
	Type() string // ipfs
	// How is it managed
	Deployment() string // process, docker, k8, (?remote?)
}
