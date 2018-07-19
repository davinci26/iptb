package pluginlocalipfs

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

var errTimeout = errors.New("timeout")

var PluginName = "localipfs"

var NewNode testbedi.NewNodeFunc
var GetAttrDesc testbedi.GetAttrDescFunc
var GetAttrList testbedi.GetAttrListFunc

func init() {
	NewNode = func(dir string, extras map[string]interface{}) (testbedi.TestbedNode, error) {
		if _, err := exec.LookPath("ipfs"); err != nil {
			return nil, err
		}

		return &Localipfs{
			dir: dir,
		}, nil

	}

	GetAttrList = func() []string {
		return ipfs.GetAttrList()
	}

	GetAttrDesc = func(attr string) (string, error) {
		return ipfs.GetAttrDesc(attr)
	}
}

type Localipfs struct {
	dir        string
	peerid     *cid.Cid
	apiaddr    multiaddr.Multiaddr
	swarmaddrs []multiaddr.Multiaddr
}

/// TestbedNode Interface

func (l *Localipfs) Init(ctx context.Context, agrs ...string) (testbedi.TBOutput, error) {
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

func (l *Localipfs) Start(ctx context.Context, args ...string) error {
	alive, err := l.isAlive()
	if err != nil {
		return err
	}

	if alive {
		return fmt.Errorf("node is already running")
	}

	dir := l.dir
	dargs := append([]string{"daemon"}, args...)
	cmd := exec.Command("ipfs", dargs...)
	cmd.Dir = dir

	cmd.Env, err = l.env()
	if err != nil {
		return err
	}

	iptbutil.SetupOpt(cmd)

	stdout, err := os.Create(filepath.Join(dir, "daemon.stdout"))
	if err != nil {
		return err
	}

	stderr, err := os.Create(filepath.Join(dir, "daemon.stderr"))
	if err != nil {
		return err
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()
	if err != nil {
		return err
	}

	pid := cmd.Process.Pid

	l.Infof("Started daemon %s, pid = %d\n", dir, pid)
	err = ioutil.WriteFile(filepath.Join(dir, "daemon.pid"), []byte(fmt.Sprint(pid)), 0666)
	if err != nil {
		return err
	}

	return ipfs.WaitOnAPI(l)
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

	if err := l.signalAndWait(p, waitch, syscall.SIGTERM, 1*time.Second); err != errTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGTERM, 2*time.Second); err != errTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGQUIT, 5*time.Second); err != errTimeout {
		return err
	}

	if err := l.signalAndWait(p, waitch, syscall.SIGKILL, 5*time.Second); err != errTimeout {
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

	cmd := exec.CommandContext(ctx, "ipfs", args...)
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

func (l *Localipfs) Shell(ctx context.Context, nodes []testbedi.TestbedNode) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return fmt.Errorf("couldnt find shell!")
	}

	if len(os.Getenv("IPFS_PATH")) != 0 {
		// If the users shell sets IPFS_PATH, it will just be overridden by the shell again
		return fmt.Errorf("Your shell has IPFS_PATH set, please unset before trying to use iptb shell")
	}

	nenvs, err := l.env()
	if err != nil {
		return err
	}

	// TODO(tperson): It would be great if we could guarantee that the shell
	// is using the same binary. However, the users shell may prepend anything
	// we change in the PATH

	for i, n := range nodes {
		peerid, err := n.PeerID()

		if err != nil {
			return err
		}

		nenvs = append(nenvs, fmt.Sprintf("NODE%d=%s", i, peerid))
	}

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
		return l.apiaddr, nil
	}

	var err error
	l.apiaddr, err = ipfs.GetAPIAddrFromRepo(l.dir)

	return l.apiaddr, err
}

func (l *Localipfs) SwarmAddrs() ([]multiaddr.Multiaddr, error) {
	if l.swarmaddrs != nil {
		return l.swarmaddrs, nil
	}

	var err error
	l.swarmaddrs, err = ipfs.SwarmAddrs(l)

	return l.swarmaddrs, err
}

func (l *Localipfs) Dir() string {
	return l.dir
}

func (l *Localipfs) PeerID() (*cid.Cid, error) {
	if l.peerid != nil {
		return l.peerid, nil
	}

	var err error
	l.peerid, err = ipfs.GetPeerID(l)

	return l.peerid, err
}

func (l *Localipfs) GetAttrList() []string {
	return GetAttrList()
}

func (l *Localipfs) GetAttrDesc(attr string) (string, error) {
	return GetAttrDesc(attr)
}

func (l *Localipfs) GetAttr(attr string) (string, error) {
	return ipfs.GetAttr(l, attr)
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

func (l *Localipfs) signalAndWait(p *os.Process, waitch <-chan struct{}, signal os.Signal, t time.Duration) error {
	err := p.Signal(signal)
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s\n", l.dir, err)
	}

	select {
	case <-waitch:
		return nil
	case <-time.After(t):
		return errTimeout
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
	ipfspath := "IPFS_PATH=" + l.dir

	for i, e := range envs {
		if strings.HasPrefix(e, "IPFS_PATH=") {
			envs[i] = ipfspath
			return envs, nil
		}
	}
	return append(envs), nil
}
