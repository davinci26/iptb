package ipfs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gxed/errors"
	"github.com/ipfs/go-cid"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/multiformats/go-multiaddr"
)

const (
	attrId    = "id"
	attrPath  = "path"
	attrBwIn  = "bw_in"
	attrBwOut = "bw_out"
)

func InitIpfs(l testbedi.TestbedNode) error {
	return nil
}

func GetAttr(l testbedi.TestbedNode, attr string) (string, error) {
	switch attr {
	case attrId:
		pcid, err := l.PeerID()
		if err != nil {
			return "", err
		}
		return pcid.String(), nil
	case attrPath:
		return l.Dir(), nil
	case attrBwIn:
		bw, err := GetBW(l)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(bw.TotalIn), nil
	case attrBwOut:
		bw, err := GetBW(l)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(bw.TotalOut), nil
	default:
		return "", errors.New("unrecognized attribute: " + attr)
	}
}

func GetPeerID(l testbedi.TestbedNode) (*cid.Cid, error) {
	icfg, err := l.GetConfig()
	if err != nil {
		return nil, err
	}

	lcfg := icfg.(*config.Config)

	pcid, err := cid.Decode(lcfg.Identity.PeerID)
	if err != nil {
		return nil, err
	}

	return pcid, nil
}

func GetAttrList() []string {
	return []string{attrId, attrPath, attrBwIn, attrBwOut}
}

func GetAttrDesc(attr string) (string, error) {
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

func ReadLogs(l testbedi.TestbedNode) (io.ReadCloser, error) {
	addr, err := l.APIAddr()
	if err != nil {
		return nil, err
	}

	//TODO(tperson) ipv6
	ip, err := addr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		return nil, err
	}
	pt, err := addr.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(fmt.Sprintf("http://%s:%s/api/v0/log/tail", ip, pt))
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

type BW struct {
	TotalIn  int
	TotalOut int
}

func GetBW(l testbedi.TestbedNode) (*BW, error) {
	addr, err := l.APIAddr()
	if err != nil {
		return nil, err
	}

	//TODO(tperson) ipv6
	ip, err := addr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		return nil, err
	}
	pt, err := addr.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(fmt.Sprintf("http://%s:%s/api/v0/stats/bw", ip, pt))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var bw BW
	err = json.NewDecoder(resp.Body).Decode(&bw)
	if err != nil {
		return nil, err
	}

	return &bw, nil
}

func GetListOfAttr() []string {
	return []string{attrId, attrPath, attrBwIn, attrBwOut}
}

func GetAPIAddrFromRepo(dir string) (multiaddr.Multiaddr, error) {
	addrb, err := ioutil.ReadFile(filepath.Join(dir, "api"))
	if err != nil {
		return nil, err
	}

	maddr, err := multiaddr.NewMultiaddr(string(addrb))
	if err != nil {
		return nil, err
	}

	return maddr, nil
}

func SwarmAddrs(l testbedi.TestbedNode) ([]multiaddr.Multiaddr, error) {
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
}

func GetAttrDescr(attr string) (string, error) {
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
