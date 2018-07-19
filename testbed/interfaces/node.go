package testbedi

import (
	"context"
	"io"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multiaddr"
)

type NewNodeFunc func(dir string, extras map[string]interface{}) (TestbedNode, error)
type GetAttrListFunc func() []string
type GetAttrDescFunc func(attr string) (string, error)

// TestbedNode specifies the interface to a process controlled by iptb
type TestbedNode interface {
	// Allows a node to run an initialization it may require
	// Ex: Installing additional dependencies / setuping configuration
	Init(ctx context.Context, agrs ...string) (TBOutput, error)

	// Starts the node
	Start(ctx context.Context, args ...string) error

	// Stops the node
	Stop(ctx context.Context, wait bool) error

	// Runs a command in the context of the node
	RunCmd(ctx context.Context, args ...string) (TBOutput, error)

	// Runs a command in the context of the node with a stdin
	RunCmdWithStdin(ctx context.Context, stdin io.Reader, args ...string) (TBOutput, error)

	// Connect the node to another
	Connect(ctx context.Context, tbn TestbedNode, timeout time.Duration) error

	// Starts a shell in the context of the node
	Shell(ctx context.Context, nodes []TestbedNode) error

	// Writes a log line to stdout
	Infof(format string, args ...interface{})

	// Writes a log line to stderr
	Errorf(format string, args ...interface{})

	// PeerID returns the peer id
	PeerID() (*cid.Cid, error)

	// APIAddr returns the multiaddr for the api
	APIAddr() (multiaddr.Multiaddr, error)

	// SwarmAddrs returns the swarm addrs for the node
	SwarmAddrs() ([]multiaddr.Multiaddr, error)

	// GetAttrList returns a list of attrs that can be retreived
	GetAttrList() []string

	// GetAttrDesc returns the description of attr
	GetAttrDesc(attr string) (string, error)

	// GetAttr returns the value of attr
	GetAttr(attr string) (string, error)

	// SetAttr sets the attr to val
	SetAttr(attr string, val string) error

	// Events returns reader for events
	Events() (io.ReadCloser, error)

	// StderrReader returns reader of stderr for the node
	StderrReader() (io.ReadCloser, error)

	// StdoutReader returns reader of stdout for the node
	StdoutReader() (io.ReadCloser, error)

	// GetConfig returns the configuration of the node
	GetConfig() (interface{}, error)

	// WriteConfig writes the configuration of the node
	WriteConfig(interface{}) error

	// Dir returns the iptb directory assigned to the node
	Dir() string

	// Type returns the type of node
	Type() string

	// Type returns the deployment
	Deployment() string

	// String returns a unique identify
	String() string
}
