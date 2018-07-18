package testbed

import (
	"fmt"
	"github.com/ipfs/iptb/testbed/interfaces"
	"io/ioutil"
	"os"
	"path"
	"plugin"

	"github.com/ipfs/iptb/plugins/ipfs/docker"
	"github.com/ipfs/iptb/plugins/ipfs/local"
)

type NodeSpec struct {
	Deployment string
	Type       string
	Dir        string
	Extra      map[string]interface{}
}

type iptbplugin struct {
	NewNode    testbedi.NewNodeFunc
	PluginName string
	BuiltIn    bool
}

var plugins map[string]iptbplugin

func init() {
	plugins = make(map[string]iptbplugin)

	plugins[pluginlocalipfs.PluginName] = iptbplugin{
		NewNode:    pluginlocalipfs.NewNode,
		PluginName: pluginlocalipfs.PluginName,
		BuiltIn:    true,
	}

	plugins[plugindockeripfs.PluginName] = iptbplugin{
		NewNode:    plugindockeripfs.NewNode,
		PluginName: plugindockeripfs.PluginName,
		BuiltIn:    true,
	}

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

	NewNode := *(NewNodeSym.(*testbedi.NewNodeFunc))

	PluginNameSym, err := pl.Lookup("PluginName")
	if err != nil {
		return err
	}

	PluginName := *(PluginNameSym.(*string))

	if pl, exists := plugins[PluginName]; exists {
		if pl.BuiltIn {
			fmt.Printf("overriding built in plugin %s with %s", PluginName path)
		} else {
			fmt.Printf("plugin %s already loaded, overriding with %s", PluginName, path)
		}
	}

	plugins[PluginName] = iptbplugin{
		NewNode:    NewNode,
		PluginName: PluginName,
		BuiltIn:    false,
	}

	return nil
}

func (ns *NodeSpec) Load() (testbedi.TestbedNode, error) {
	pluginName := fmt.Sprintf("%s%s", ns.Deployment, ns.Type)

	if plg, ok := plugins[pluginName]; ok {
		return plg.NewNode(ns.Dir, ns.Extra)
	}

	return nil, fmt.Errorf("Could not find plugin %s", pluginName)
}

func (ns *NodeSpec) SetExtra(attr string, val interface{}) {
	ns.Extra[attr] = val
}

func (ns *NodeSpec) GetExtra(attr string) (interface{}, error) {
	if v, ok := ns.Extra[attr]; ok {
		return v, nil
	}

	return nil, fmt.Errorf("Extra not set")
}
