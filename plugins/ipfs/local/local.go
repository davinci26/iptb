package localipfs

func IpfsKillAll(nds []IpfsNode) error {
	var errs []error
	for _, n := range nds {
		err := n.Kill()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		var errstr string
		for _, e := range errs {
			errstr += "\n" + e.Error()
		}
		return fmt.Errorf(strings.TrimSpace(errstr))
	}
	return nil
}

func IpfsStart(nodes []IpfsNode, waitall bool, args []string) error {
	for _, n := range nodes {
		if err := n.Start(args); err != nil {
			return err
		}
	}
	if waitall {
		for _, n := range nodes {
			err := waitOnSwarmPeers(n)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

// GetPeerID reads the config of node 'n' and returns its peer ID
func GetPeerID(ipfsdir string) (string, error) {
	cfg, err := serial.Load(path.Join(ipfsdir, "config"))
	if err != nil {
		return "", err
	}
	return cfg.Identity.PeerID, nil
}

func ConnectNodes(from, to IpfsNode, timeout string) error {
	if from == to {
		// skip connecting to self..
		return nil
	}

	out, err := to.RunCmd("ipfs", "id", "-f", "<addrs>")
	if err != nil {
		return fmt.Errorf("error checking node address: %s", err)
	}

	stump.Log("connecting %s -> %s\n", from, to)

	addrs := strings.Fields(string(out))
	fmt.Println("Addresses: ", addrs)
	orderishAddresses(addrs)
	for i := 0; i < len(addrs); i++ {
		addr := addrs[i]
		stump.Log("trying ipfs swarm connect %s", addr)

		args := []string{"ipfs", "swarm", "connect", addr}
		if timeout != "" {
			args = append(args, "--timeout="+timeout)
		}

		_, err = from.RunCmd(args...)

		if err == nil {
			stump.Log("connection success!")
			return nil
		}
		stump.Log("dial attempt to %s failed: %s", addr, err)
		time.Sleep(time.Second)
	}

	return errors.New("no dialable addresses")
}

func starBootstrap(nodes []IpfsNode, icfg *InitCfg) error {
	// '0' node is the bootstrap node
	king := nodes[0]

	bcfg, err := king.GetConfig()
	if err != nil {
		return err
	}

	bcfg.Bootstrap = nil
	bcfg.Addresses.Swarm = []string{icfg.swarmAddrForPeer(0)}
	bcfg.Addresses.API = icfg.apiAddrForPeer(0)
	bcfg.Addresses.Gateway = ""
	bcfg.Discovery.MDNS.Enabled = icfg.Mdns

	err = king.WriteConfig(bcfg)
	if err != nil {
		return err
	}

	for i, nd := range nodes[1:] {
		cfg, err := nd.GetConfig()
		if err != nil {
			return err
		}

		ba := fmt.Sprintf("%s/ipfs/%s", bcfg.Addresses.Swarm[0], bcfg.Identity.PeerID)
		ba = strings.Replace(ba, "0.0.0.0", "127.0.0.1", -1)
		cfg.Bootstrap = []string{ba}
		cfg.Addresses.Gateway = ""
		cfg.Discovery.MDNS.Enabled = icfg.Mdns
		cfg.Addresses.Swarm = []string{
			icfg.swarmAddrForPeer(i + 1),
		}
		cfg.Addresses.API = icfg.apiAddrForPeer(i + 1)

		err = nd.WriteConfig(cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func clearBootstrapping(nodes []IpfsNode, icfg *InitCfg) error {
	for i, nd := range nodes {
		cfg, err := nd.GetConfig()
		if err != nil {
			return err
		}

		cfg.Bootstrap = nil
		cfg.Addresses.Gateway = ""
		cfg.Addresses.Swarm = []string{icfg.swarmAddrForPeer(i)}
		cfg.Addresses.API = icfg.apiAddrForPeer(i)
		cfg.Discovery.MDNS.Enabled = icfg.Mdns
		err = nd.WriteConfig(cfg)
		if err != nil {
			return err
		}
	}
	return nil
}
