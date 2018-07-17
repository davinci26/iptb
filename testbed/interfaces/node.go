package testbedi

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multiaddr"
)

type NewNodeFunc func(binpath, dir string) TestbedNode
type GetAttrListFunc func() []string
type GetAttrDescFunc func(attr string) (string, error)

type TestbedNode interface {
	Init(ctx context.Context, agrs ...string) (TBOutput, error)
	Start(ctx context.Context, args ...string) (TBOutput, error)
	// This needs to handle killing when iptb is imported as a package and with used via cli
	Kill(ctx context.Context, wait bool) error

	RunCmd(ctx context.Context, args ...string) (TBOutput, error)
	Connect(ctx context.Context, tbn TestbedNode, timeout time.Duration) error
	Shell(ctx context.Context) error

	String() string

	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})

	Dir() (string, error)
	PeerID() (*cid.Cid, error)
	APIAddr() (multiaddr.Multiaddr, error)
	SwarmAddrs() ([]multiaddr.Multiaddr, error)

	GetAttrList() []string
	GetAttrDesc(attr string) (string, error)

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
