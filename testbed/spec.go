package testbed

import (
	"fmt"
	"github.com/ipfs/iptb/testbed/interfaces"
	"io/ioutil"
	"os"
	"path"
	"plugin"
)

type NodeSpec struct {
	Deployment string
	Type       string
	Dir        string
	BinPath    string
	Extra      map[string]interface{}
}

type iptbplugin struct {
	NewNode    func(binpath, dir string) testbedi.TestbedNode
	PluginName string
}

var plugins map[string]iptbplugin

func init() {
	plugins = make(map[string]iptbplugin)

	loadPlugins()
}

func pluginDir() (string, error) {
	tbd := os.Getenv("IPTB_PLUGINS")
	if len(tbd) != 0 {
		return tbd, nil
	}

	home := os.Getenv("HOME")
	if len(home) == 0 {
		return "", fmt.Errorf("environment variable HOME is not set")
	}

	p := path.Join(home, ".iptbplugins")
	err := os.MkdirAll(p, 0775)

	return p, err
}

func loadPlugins() {
	dir, err := pluginDir()
	if err != nil {
		fmt.Println(err)
		return
	}

	plugs, err := ioutil.ReadDir(dir)

	if err != nil {
		fmt.Println(err)
		return
	}

	for _, f := range plugs {
		err := loadPlugin(path.Join(dir, f.Name()))

		if err != nil {
			fmt.Println(err)
			continue
		}
	}
}

func loadPlugin(path string) error {
	pl, err := plugin.Open(path)

	if err != nil {
		return err
	}

	NewNodeSym, err := pl.Lookup("NewNode")
	if err != nil {
		return err
	}

	NewNode := NewNodeSym.(func(binpath, dir string) testbedi.TestbedNode)

	PluginNameSym, err := pl.Lookup("PluginName")
	if err != nil {
		return err
	}

	PluginName := *(PluginNameSym.(*string))

	plugins[PluginName] = iptbplugin{
		NewNode:    NewNode,
		PluginName: PluginName,
	}

	return nil
}

func (ns *NodeSpec) Load() (testbedi.TestbedNode, error) {
	pluginName := fmt.Sprintf("%s%s", ns.Deployment, ns.Type)

	if plg, ok := plugins[pluginName]; ok {
		node := plg.NewNode(ns.BinPath, ns.Dir)
		return node, nil
	}

	return nil, fmt.Errorf("Could not find plugin %s", pluginName)
}
