package ipfs

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gxed/errors"
	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/multiformats/go-multiaddr"
)

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

const (
	attrId    = "id"
	attrPath  = "path"
	attrBwIn  = "bw_in"
	attrBwOut = "bw_out"
)

func GetListOfAttr() []string {
	return []string{attrId, attrPath, attrBwIn, attrBwOut}
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
