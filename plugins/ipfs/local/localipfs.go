package main

import (
	"context"
	"fmt"
	"path/filepath"

	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multiaddr"

	"github.com/gxed/errors"
	"github.com/ipfs/iptb/plugins/ipfs"
	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"

	config "github.com/ipfs/go-ipfs/repo/config"
	serial "github.com/ipfs/go-ipfs/repo/fsrepo/serialize"
)

var ErrTimeout = errors.New("timeout")

var PluginName = "localipfs"

var NewNode testbedi.NewNodeFunc
var GetAttrDesc testbedi.GetAttrDescFunc
var GetAttrList testbedi.GetAttrListFunc

const (
	attrId    = "id"
	attrPath  = "path"
	attrBwIn  = "bw_in"
	attrBwOut = "bw_out"
)

func init() {
	NewNode = func(binpath, dir string) testbedi.TestbedNode {
		return &Localipfs{
			dir:     dir,
			binpath: binpath,
		}
	}

	GetAttrList = func() []string {
		return []string{attrId, attrPath, attrBwIn, attrBwOut}
	}

	GetAttrDesc = func(attr string) (string, error) {
		switch attr {
		case attrId:
			return "node ID", nil
		case attrPath:
			return "node IPFS_PATH", nil
		case attrBwIn:
			return "node input bandwidth", nil
		case attrBwOut:
			return "node output bandwidth", nil
		default:
			return "", errors.New("unrecognized attribute")
		}
	}
}

type Localipfs struct {
	binpath    string
	dir        string
	peerid     *cid.Cid
	apiaddr    *multiaddr.Multiaddr
	swarmaddrs []multiaddr.Multiaddr
}

func (l *Localipfs) signalAndWait(p *os.Process, waitch <-chan struct{}, signal os.Signal, t time.Duration) error {
	err := p.Signal(signal)
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s\n", l.dir, err)
	}

	select {
	case <-waitch:
		return nil
	case <-time.After(t):
		return ErrTimeout
	}
}

/*
func Bootstrap(nodes []testbedi.TestbedNode, port uint) error {
	leader := nodes[0]

	icfg, err := leader.GetConfig()
	if err != nil {
		return err
	}

	lcfg := icfg.(config.Config)

	lcfg.Bootstrap = nil
	lcfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 0)}
	lcfg.Addresses.API = fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)
	lcfg.Addresses.Gateway = ""
	lcfg.Discovery.MDNS.Enabled = false

	err = leader.WriteConfig(lcfg)
	if err != nil {
		return err
	}

	ba := fmt.Sprintf("%s/ipfs/%s", bcfg.Addresses.Swarm[0], bcfg.Identity.PeerID)
	ba = strings.Replace(ba, "0.0.0.0", "127.0.0.1", -1)

	for i, nd := range nodes[1:] {
		icfg, err := nd.GetConfig()
		if err != nil {
			return err
		}

		lcfg := icfg.(config.Config)

		lcfg.Bootstrap = []string{ba}
		lcfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 0)}
		lcfg.Addresses.API = fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port+i+1)
		lcfg.Addresses.Gateway = ""
		lcfg.Discovery.MDNS.Enabled = false

		err = nd.WriteConfig(lcfg)
		if err != nil {
			return err
		}
	}

	return nil
}
*/

func (l *Localipfs) getPID() (int, error) {
	b, err := ioutil.ReadFile(filepath.Join(l.dir, "daemon.pid"))
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(string(b))
}

func (l *Localipfs) isAlive() (bool, error) {
	pid, err := l.getPID()
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (l *Localipfs) env() ([]string, error) {
	envs := os.Environ()
	dir := l.dir
	repopath := "IPFS_PATH=" + dir

	for i, e := range envs {
		p := strings.Split(e, "=")
		if p[0] == "IPFS_PATH" {
			envs[i] = repopath
			return envs, nil
		}
	}

	return append(envs, repopath), nil
}

/// TestbedNode Interface

func (l *Localipfs) Init(ctx context.Context, agrs ...string) (testbedi.TBOutput, error) {
	if err := os.MkdirAll(l.dir, 0755); err != nil {
		return nil, err
	}

	agrs = append([]string{"init"}, agrs...)
	output, oerr := l.RunCmd(ctx, agrs...)

	icfg, err := l.GetConfig()
	if err != nil {
		return nil, err
	}

	lcfg := icfg.(*config.Config)

	lcfg.Bootstrap = nil
	lcfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 0)}
	lcfg.Addresses.API = fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 0)
	lcfg.Addresses.Gateway = ""
	lcfg.Discovery.MDNS.Enabled = false

	err = l.WriteConfig(lcfg)
	if err != nil {
		return nil, err
	}

	return output, oerr
}

func (l *Localipfs) Start(ctx context.Context, args ...string) (testbedi.TBOutput, error) {
	alive, err := l.isAlive()
	if err != nil {
		return nil, err
	}

	if alive {
		return nil, fmt.Errorf("node is already running")
	}

	dir := l.dir
	dargs := append([]string{"daemon"}, args...)
	cmd := exec.Command(l.binpath, dargs...)
	cmd.Dir = dir

	cmd.Env, err = l.env()
	if err != nil {
		return nil, err
	}

	iptbutil.SetupOpt(cmd)

	stdout, err := os.Create(filepath.Join(dir, "daemon.stdout"))
	if err != nil {
		return nil, err
	}

	stderr, err := os.Create(filepath.Join(dir, "daemon.stderr"))
	if err != nil {
		return nil, err
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	pid := cmd.Process.Pid

	l.Infof("Started daemon %s, pid = %d\n", dir, pid)
	err = ioutil.WriteFile(filepath.Join(dir, "daemon.pid"), []byte(fmt.Sprint(pid)), 0666)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (l *Localipfs) Stop(ctx context.Context, wait bool) error {
	pid, err := l.getPID()
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s", l.dir, err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s", l.dir, err)
	}

	waitch := make(chan struct{}, 1)
	go func() {
		p.Wait() //TODO: pass return state
		waitch <- struct{}{}
	}()

	defer func() {
		err := os.Remove(filepath.Join(l.dir, "daemon.pid"))
		if err != nil && !os.IsNotExist(err) {
			panic(fmt.Errorf("error removing pid file for daemon at %s: %s\n", l.dir, err))
		}
	}()

	if err := l.signalAndWait(p, waitch, syscall.SIGTERM, 1*time.Second); err != ErrTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGTERM, 2*time.Second); err != ErrTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGQUIT, 5*time.Second); err != ErrTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGKILL, 5*time.Second); err != ErrTimeout {
		return err
	}

	for {
		err := p.Signal(syscall.Signal(0))
		if err != nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	return nil
}

func (l *Localipfs) RunCmd(ctx context.Context, args ...string) (testbedi.TBOutput, error) {
	return l.RunCmdWithStdin(ctx, nil, args...)
}

func (l *Localipfs) RunCmdWithStdin(ctx context.Context, stdin io.Reader, args ...string) (testbedi.TBOutput, error) {
	env, err := l.env()

	if err != nil {
		return nil, fmt.Errorf("error getting env: %s", err)
	}

	bin := l.binpath

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) //TODO(tperson)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	cmd.Stdin = stdin

	l.Infof("%#v", args)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()

	stderrbytes, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, err
	}

	stdoutbytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	exiterr := cmd.Wait()

	var exitcode = 0
	switch oerr := exiterr.(type) {
	case *exec.ExitError:
		if ctx.Err() == context.DeadlineExceeded {
			err = errors.Wrapf(oerr, "context deadline exceeded for command: %q", strings.Join(cmd.Args, " "))
		}

		exitcode = 1
	case nil:
		err = oerr
	}

	return iptbutil.NewOutput(args, stdoutbytes, stderrbytes, exitcode, err)
}

func (l *Localipfs) Connect(ctx context.Context, tbn testbedi.TestbedNode, timeout time.Duration) error {
	swarmaddrs, err := tbn.SwarmAddrs()
	if err != nil {
		return err
	}

	_, err = l.RunCmd(ctx, "swarm", "connect", swarmaddrs[0].String())

	return err
}

func (l *Localipfs) Shell(ctx context.Context) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return fmt.Errorf("couldnt find shell!")
	}

	nenvs := []string{"IPFS_PATH=" + l.dir}

	return syscall.Exec(shell, []string{shell}, nenvs)
}

func (l *Localipfs) String() string {
	return fmt.Sprintf("localipfs")
}

func (l *Localipfs) Infof(format string, args ...interface{}) {
	nformat := fmt.Sprintf("%s %s\n", l, format)
	fmt.Fprintf(os.Stdout, nformat, args...)
}

func (l *Localipfs) Errorf(format string, args ...interface{}) {
	nformat := fmt.Sprintf("%s %s\n", l, format)
	fmt.Fprintf(os.Stderr, nformat, args...)
}

func (l *Localipfs) APIAddr() (multiaddr.Multiaddr, error) {
	if l.apiaddr != nil {
		return *l.apiaddr, nil
	}

	dir := l.dir

	addrb, err := ioutil.ReadFile(filepath.Join(dir, "api"))
	if err != nil {
		return nil, err
	}

	maddr, err := multiaddr.NewMultiaddr(string(addrb))
	if err != nil {
		return nil, err
	}

	l.apiaddr = &maddr

	return *l.apiaddr, nil
}

func (l *Localipfs) SwarmAddrs() ([]multiaddr.Multiaddr, error) {
	pcid, err := l.PeerID()
	if err != nil {
		return nil, err
	}

	output, err := l.RunCmd(context.TODO(), "swarm", "addrs", "local")
	if err != nil {
		return nil, err
	}

	bs, err := ioutil.ReadAll(output.Stdout())
	if err != nil {
		return nil, err
	}

	straddrs := strings.Split(string(bs), "\n")

	var maddrs []multiaddr.Multiaddr
	for _, straddr := range straddrs {
		fstraddr := fmt.Sprintf("%s/ipfs/%s", straddr, pcid)
		maddr, err := multiaddr.NewMultiaddr(fstraddr)
		if err != nil {
			return nil, err
		}

		maddrs = append(maddrs, maddr)
	}

	return maddrs, nil

	l.swarmaddrs = maddrs

	return l.swarmaddrs, err
}

func (l *Localipfs) Dir() string {
	return l.dir
}

func (l *Localipfs) PeerID() (*cid.Cid, error) {
	if l.peerid != nil {
		return l.peerid, nil
	}

	icfg, err := l.GetConfig()
	if err != nil {
		return nil, err
	}

	lcfg := icfg.(*config.Config)

	pcid, err := cid.Decode(lcfg.Identity.PeerID)
	if err != nil {
		return nil, err
	}

	l.peerid = pcid

	return l.peerid, err
}

func (l *Localipfs) GetAttrList() []string {
	return GetAttrList()
}

func (l *Localipfs) GetAttrDesc(attr string) (string, error) {
	return GetAttrDesc(attr)
}

func (l *Localipfs) GetAttr(attr string) (string, error) {
	switch attr {
	case attrId:
		pcid, err := l.PeerID()
		if err != nil {
			return "", err
		}
		return pcid.String(), nil
	case attrPath:
		return l.dir, nil
	case attrBwIn:
		bw, err := ipfs.GetBW(l)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(bw.TotalIn), nil
	case attrBwOut:
		bw, err := ipfs.GetBW(l)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(bw.TotalOut), nil
	default:
		return "", errors.New("unrecognized attribute: " + attr)
	}
}

func (l *Localipfs) SetAttr(string, string) error {
	return fmt.Errorf("no attribute to set")
}

func (l *Localipfs) Events() (io.ReadCloser, error) {
	return ipfs.ReadLogs(l)
}

func (l *Localipfs) StderrReader() (io.ReadCloser, error) {
	return l.readerFor("daemon.stdout")
}

func (l *Localipfs) StdoutReader() (io.ReadCloser, error) {
	return l.readerFor("daemon.stdout")
}

func (l *Localipfs) GetConfig() (interface{}, error) {
	return serial.Load(filepath.Join(l.dir, "config"))
}

func (l *Localipfs) WriteConfig(cfg interface{}) error {
	return serial.WriteConfigFile(filepath.Join(l.dir, "config"), cfg)
}

func (l *Localipfs) Type() string {
	return "ipfs"
}

func (l *Localipfs) Deployment() string {
	return "local"
}

func (l *Localipfs) readerFor(file string) (io.ReadCloser, error) {
	return os.OpenFile(filepath.Join(l.dir, file), os.O_RDONLY, 0)
}
