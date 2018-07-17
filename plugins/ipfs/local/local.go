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
	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"

	config "github.com/ipfs/go-ipfs/repo/config"
	serial "github.com/ipfs/go-ipfs/repo/fsrepo/serialize"
)

var ErrTimeout = errors.New("timeout")

type Localipfs struct {
	binpath string
	dir     string
	peerid  cid.Cid
	api     multiaddr.Multiaddr
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

func NewNode(binpath, dir string) testbedi.TestbedNode {
	return &Localipfs{
		dir:     dir,
		binpath: binpath,
	}
}

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

func (l *Localipfs) Init(agrs ...string) (testbedi.TBOutput, error) {
	if err := os.MkdirAll(l.dir, 0755); err != nil {
		return nil, err
	}

	agrs = append([]string{"init"}, agrs...)
	output, oerr := l.RunCmd(agrs...)

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

func (l *Localipfs) Start(args ...string) (testbedi.TBOutput, error) {
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

func (l *Localipfs) Kill(wait bool) error {
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

func (l *Localipfs) RunCmd(args ...string) (testbedi.TBOutput, error) {
	return l.RunCmdWithStdin(nil, args...)
}

func (l *Localipfs) RunCmdWithStdin(stdin io.Reader, args ...string) (testbedi.TBOutput, error) {
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

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()

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

	return iptbutil.NewOutput(args, stdout, stderr, exitcode, err)
}

func (l *Localipfs) Connect(tbn *testbedi.TestbedNode, timeout time.Duration) error {
	panic("not implemented")
}

func (l *Localipfs) Shell() error {
	panic("not implemented")
}

func (l *Localipfs) String() string {
	panic("not implemented")
}

func (l *Localipfs) Infof(format string, args ...interface{}) {
	fmt.Printf(format, args...)
	fmt.Println()
}

func (l *Localipfs) Errorf(format string, args ...interface{}) {
	panic("not implemented")
}

func (l *Localipfs) APIAddr() (*multiaddr.Multiaddr, error) {
	panic("not implemented")
}

func (l *Localipfs) GetPeerID() (*cid.Cid, error) {
	panic("not implemented")
}

func (l *Localipfs) GetAttr(string) (string, error) {
	panic("not implemented")
}

func (l *Localipfs) SetAttr(string, string) error {
	panic("not implemented")
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
