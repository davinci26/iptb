package ipfs

func IpfsDirN(n int) (string, error) {
	tbd, err := TestBedDir()
	if err != nil {
		return "", err
	}
	return path.Join(tbd, fmt.Sprint(n)), nil
}

func waitOnAPI(n IpfsNode) error {
	for i := 0; i < 50; i++ {
		err := tryAPICheck(n)
		if err == nil {
			return nil
		}
		stump.VLog("temp error waiting on API: ", err)
		time.Sleep(time.Millisecond * 400)
	}
	return fmt.Errorf("node %s failed to come online in given time period", n.GetPeerID())
}

func tryAPICheck(n IpfsNode) error {
	addr, err := n.APIAddr()
	if err != nil {
		return err
	}

	stump.VLog("checking api addresss at: ", addr)
	resp, err := http.Get("http://" + addr + "/api/v0/id")
	if err != nil {
		return err
	}

	out := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return fmt.Errorf("liveness check failed: %s", err)
	}

	id, ok := out["ID"]
	if !ok {
		return fmt.Errorf("liveness check failed: ID field not present in output")
	}

	idstr := id.(string)
	if idstr != n.GetPeerID() {
		return fmt.Errorf("liveness check failed: unexpected peer at endpoint")
	}

	return nil
}

func waitOnSwarmPeers(n IpfsNode) error {
	addr, err := n.APIAddr()
	if err != nil {
		return err
	}

	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + addr + "/api/v0/swarm/peers")
		if err == nil {
			out := make(map[string]interface{})
			err := json.NewDecoder(resp.Body).Decode(&out)
			if err != nil {
				return fmt.Errorf("liveness check failed: %s", err)
			}

			pstrings, ok := out["Strings"]
			if ok {
				if len(pstrings.([]interface{})) == 0 {
					continue
				}
				return nil
			}

			peers, ok := out["Peers"]
			if !ok {
				return fmt.Errorf("object from swarm peers doesnt look right (api mismatch?)")
			}

			if peers == nil {
				time.Sleep(time.Millisecond * 200)
				continue
			}

			if plist, ok := peers.([]interface{}); ok && len(plist) == 0 {
				continue
			}

			return nil
		}
		time.Sleep(time.Millisecond * 200)
	}
	return fmt.Errorf("node at %s failed to bootstrap in given time period", addr)
}

func orderishAddresses(addrs []string) {
	for i, a := range addrs {
		if strings.Contains(a, "127.0.0.1") {
			addrs[i], addrs[0] = addrs[0], addrs[i]
			return
		}
	}
}

type BW struct {
	TotalIn  int
	TotalOut int
}

func GetBW(n IpfsNode) (*BW, error) {
	addr, err := n.APIAddr()
	if err != nil {
		return nil, err
	}

	resp, err := http.Get("http://" + addr + "/api/v0/stats/bw")
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
