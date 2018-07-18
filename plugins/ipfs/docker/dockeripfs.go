package plugindockeripfs

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gxed/errors"
	"github.com/ipfs/go-cid"
	config "github.com/ipfs/go-ipfs/repo/config"
	serial "github.com/ipfs/go-ipfs/repo/fsrepo/serialize"
	"github.com/multiformats/go-multiaddr"
	cnet "github.com/whyrusleeping/go-ctrlnet"

	"github.com/ipfs/iptb/plugins/ipfs"
	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"
)

var ErrTimeout = errors.New("timeout")

var PluginName = "dockeripfs"

var NewNode testbedi.NewNodeFunc
var GetAttrDesc testbedi.GetAttrDescFunc
var GetAttrList testbedi.GetAttrListFunc

const (
	attrId     = "id"
	attrPath   = "path"
	attrBwIn   = "bw_in"
	attrBwOut  = "bw_out"
	attrIfName = "ifname"
)

func init() {
	NewNode = func(dir string, extras map[string]interface{}) (testbedi.TestbedNode, error) {
		var imagename string
		var repobuilder string

		if v, ok := extras["image"]; ok {
			imagename, ok = v.(string)

			if !ok {
				return nil, fmt.Errorf("Extra `image` should be a string")
			}

		} else {
			return nil, fmt.Errorf("No `image` provided")
		}

		if v, ok := extras["repobuilder"]; ok {
			repobuilder, ok = v.(string)

			if !ok {
				return nil, fmt.Errorf("Extra `repobuilder` should be a string")
			}

		} else {
			return nil, fmt.Errorf("No `repobuilder` provided")
		}

		return &Dockeripfs{
			dir:         dir,
			image:       imagename,
			repobuilder: repobuilder,
		}, nil
	}

	GetAttrList = func() []string {
		return []string{attrId, attrPath, attrBwIn, attrBwOut, attrIfName}
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
		case attrIfName:
			return "docker ifname", nil
		default:
			return "", errors.New("unrecognized attribute")
		}
	}
}

type Dockeripfs struct {
	image       string
	id          string
	dir         string
	repobuilder string
	peerid      *cid.Cid
	apiaddr     *multiaddr.Multiaddr
	swarmaddrs  []multiaddr.Multiaddr
}

func (l *Dockeripfs) getInterfaceName() (string, error) {
	out, err := l.RunCmd(context.TODO(), "ip", "link")
	if err != nil {
		return "", err
	}

	stdout, err := ioutil.ReadAll(out.Stdout())
	if err != nil {
		return "", err
	}

	var cside string
	for _, l := range strings.Split(string(stdout), "\n") {
		if strings.Contains(l, "@if") {
			ifnum := strings.Split(strings.Split(l, " ")[1], "@")[1]
			cside = ifnum[2 : len(ifnum)-1]
			break
		}
	}

	if cside == "" {
		return "", fmt.Errorf("container-side interface not found")
	}

	localout, err := exec.Command("ip", "link").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, localout)
	}

	for _, l := range strings.Split(string(localout), "\n") {
		if strings.HasPrefix(l, cside+": ") {
			return strings.Split(strings.Fields(l)[1], "@")[0], nil
		}
	}

	return "", fmt.Errorf("could not determine interface")
}

func (l *Dockeripfs) setLatency(val string) error {
	dur, err := time.ParseDuration(val)
	if err != nil {
		return err
	}

	ifn, err := l.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		Latency: uint(dur.Nanoseconds() / 1000000),
	}

	return cnet.SetLink(ifn, settings)
}

func (l *Dockeripfs) setJitter(val string) error {
	dur, err := time.ParseDuration(val)
	if err != nil {
		return err
	}

	ifn, err := l.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		Jitter: uint(dur.Nanoseconds() / 1000000),
	}

	return cnet.SetLink(ifn, settings)
}

// set bandwidth (expects Mbps)
func (l *Dockeripfs) setBandwidth(val string) error {
	bw, err := strconv.ParseFloat(val, 32)
	if err != nil {
		return err
	}

	ifn, err := l.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		Bandwidth: uint(bw * 1000000),
	}

	return cnet.SetLink(ifn, settings)
}

// set packet loss percentage (dropped / total)
func (l *Dockeripfs) setPacketLoss(val string) error {
	ratio, err := strconv.ParseUint(val, 10, 8)
	if err != nil {
		return err
	}

	ifn, err := l.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		PacketLoss: uint8(ratio),
	}

	return cnet.SetLink(ifn, settings)
}

func (l *Dockeripfs) signalAndWait(p *os.Process, waitch <-chan struct{}, signal os.Signal, t time.Duration) error {
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

func (l *Dockeripfs) getID() (string, error) {
	if len(l.id) != 0 {
		return l.id, nil
	}

	b, err := ioutil.ReadFile(filepath.Join(l.dir, "dockerid"))
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (l *Dockeripfs) isAlive() (bool, error) {
	return false, nil
}

func (l *Dockeripfs) env() ([]string, error) {
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

func (l *Dockeripfs) Init(ctx context.Context, agrs ...string) (testbedi.TBOutput, error) {
	if err := os.MkdirAll(l.dir, 0755); err != nil {
		return nil, err
	}

	env, err := l.env()
	if err != nil {
		return nil, fmt.Errorf("error getting env: %s", err)
	}

	cmd := exec.Command(l.repobuilder, "init")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, string(out))
	}

	icfg, err := l.GetConfig()
	if err != nil {
		return nil, err
	}

	lcfg := icfg.(*config.Config)

	lcfg.Bootstrap = nil
	lcfg.Addresses.Gateway = ""
	lcfg.Discovery.MDNS.Enabled = false

	err = l.WriteConfig(lcfg)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (l *Dockeripfs) Start(ctx context.Context, args ...string) (testbedi.TBOutput, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("cannot yet pass daemon args to docker nodes")
	}

	alive, err := l.isAlive()
	if err != nil {
		return nil, err
	}

	if alive {
		return nil, fmt.Errorf("node is already running")
	}

	cmd := exec.Command("docker", "run", "-d", "-v", l.dir+":/data/ipfs", l.image)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, string(out))
	}

	id := bytes.TrimSpace(out)
	l.id = string(id)

	idfile := filepath.Join(l.dir, "dockerid")
	err = ioutil.WriteFile(idfile, id, 0664)

	if err != nil {
		killErr := l.killContainer()
		if killErr != nil {
			return nil, combineErrors(err, killErr)
		}
		return nil, err
	}

	return nil, nil
}

func (l *Dockeripfs) killContainer() error {
	id, err := l.getID()
	if err != nil {
		return err
	}
	out, err := exec.Command("docker", "kill", "--signal=INT", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}
	return nil
}

func (l *Dockeripfs) Stop(ctx context.Context, wait bool) error {
	err := l.killContainer()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(l.dir, "dockerid"))
}

func (l *Dockeripfs) RunCmd(ctx context.Context, args ...string) (testbedi.TBOutput, error) {
	return l.RunCmdWithStdin(ctx, nil, args...)
}

func (l *Dockeripfs) RunCmdWithStdin(ctx context.Context, stdin io.Reader, args ...string) (testbedi.TBOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) //TODO(tperson)
	defer cancel()

	id, err := l.getID()
	if err != nil {
		return nil, err
	}

	args = append([]string{"exec", "-t", id, "ipfs"}, args...)
	cmd := exec.CommandContext(ctx, "docker", args...)
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

func (l *Dockeripfs) Connect(ctx context.Context, tbn testbedi.TestbedNode, timeout time.Duration) error {
	swarmaddrs, err := tbn.SwarmAddrs()
	if err != nil {
		return err
	}

	_, err = l.RunCmd(ctx, "swarm", "connect", swarmaddrs[0].String())

	return err
}

func (l *Dockeripfs) Shell(ctx context.Context, nodes []testbedi.TestbedNode) error {
	id, err := l.getID()
	if err != nil {
		return err
	}

	nenvs := []string{}
	for i, n := range nodes {
		peerid, err := n.PeerID()

		if err != nil {
			return err
		}

		nenvs = append(nenvs, fmt.Sprintf("NODE%d=%s", i, peerid))
	}

	args := []string{"exec", "-it"}
	for _, e := range nenvs {
		args = append(args, "-e", e)
	}

	args = append(args, id, "/bin/sh")
	cmd := exec.Command("docker", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (l *Dockeripfs) String() string {
	return fmt.Sprintf("dockeripfs")
}

func (l *Dockeripfs) Infof(format string, args ...interface{}) {
	nformat := fmt.Sprintf("%s %s\n", l, format)
	fmt.Fprintf(os.Stdout, nformat, args...)
}

func (l *Dockeripfs) Errorf(format string, args ...interface{}) {
	nformat := fmt.Sprintf("%s %s\n", l, format)
	fmt.Fprintf(os.Stderr, nformat, args...)
}

func (l *Dockeripfs) APIAddr() (multiaddr.Multiaddr, error) {
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

func (l *Dockeripfs) SwarmAddrs() ([]multiaddr.Multiaddr, error) {
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

func (l *Dockeripfs) Dir() string {
	return l.dir
}

func (l *Dockeripfs) PeerID() (*cid.Cid, error) {
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

func (l *Dockeripfs) GetAttrList() []string {
	return GetAttrList()
}

func (l *Dockeripfs) GetAttrDesc(attr string) (string, error) {
	return GetAttrDesc(attr)
}

func (l *Dockeripfs) GetAttr(attr string) (string, error) {
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
	case attrIfName:
		l.getInterfaceName()
	default:
		return "", errors.New("unrecognized attribute: " + attr)
	}

	return "", nil
}

func (l *Dockeripfs) SetAttr(attr string, val string) error {
	switch attr {
	case "latency":
		return l.setLatency(val)
	case "bandwidth":
		return l.setBandwidth(val)
	case "jitter":
		return l.setJitter(val)
	case "loss":
		return l.setPacketLoss(val)
	default:
		return fmt.Errorf("no attribute named: %s", attr)
	}
}

func (l *Dockeripfs) Events() (io.ReadCloser, error) {
	return ipfs.ReadLogs(l)
}

func (l *Dockeripfs) StderrReader() (io.ReadCloser, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (l *Dockeripfs) StdoutReader() (io.ReadCloser, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (l *Dockeripfs) GetConfig() (interface{}, error) {
	return serial.Load(filepath.Join(l.dir, "config"))
}

func (l *Dockeripfs) WriteConfig(cfg interface{}) error {
	return serial.WriteConfigFile(filepath.Join(l.dir, "config"), cfg)
}

func (l *Dockeripfs) Type() string {
	return "ipfs"
}

func (l *Dockeripfs) Deployment() string {
	return "docker"
}

func (l *Dockeripfs) readerFor(file string) (io.ReadCloser, error) {
	return os.OpenFile(filepath.Join(l.dir, file), os.O_RDONLY, 0)
}

func combineErrors(err1, err2 error) error {
	return fmt.Errorf("%v\nwhile handling the above error, the following error occurred:\n%v", err1, err2)
}
