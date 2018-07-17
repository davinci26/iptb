package testbed

import (
	"encoding/json"
	"fmt"
	"os"
)

type InitCfg struct {
	Count     int
	Force     bool
	Bootstrap string
	PortStart int
	Mdns      bool
	Utp       bool
	Websocket bool
	Override  string
	NodeType  string
}

func (c *InitCfg) swarmAddrForPeer(i int) string {
	str := "/ip4/0.0.0.0/tcp/%d"
	if c.Utp {
		str = "/ip4/0.0.0.0/udp/%d/utp"
	}
	if c.Websocket {
		str = "/ip4/0.0.0.0/tcp/%d/ws"
	}

	if c.PortStart == 0 {
		return fmt.Sprintf(str, 0)
	}
	return fmt.Sprintf(str, c.PortStart+i)
}

func (c *InitCfg) apiAddrForPeer(i int) string {
	ip := "127.0.0.1"
	if c.NodeType == "docker" {
		ip = "0.0.0.0"
	}

	var port int
	if c.PortStart != 0 {
		port = c.PortStart + 1000 + i
	}

	return fmt.Sprintf("/ip4/%s/tcp/%d", ip, port)
}

func ApplyConfigOverride(cfg *InitCfg) error {
	fir, err := os.Open(cfg.Override)
	if err != nil {
		return err
	}
	defer fir.Close()

	var configs map[string]interface{}
	err = json.NewDecoder(fir).Decode(&configs)
	if err != nil {
		return err
	}

	for i := 0; i < cfg.Count; i++ {
		err := applyOverrideToNode(configs, i)
		if err != nil {
			return err
		}
	}

	return nil
}

func applyOverrideToNode(ovr map[string]interface{}, node int) error {
	for k, v := range ovr {
		_ = k
		switch v.(type) {
		case map[string]interface{}:
		default:
		}

	}

	panic("not implemented")
}
