package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/genuinetools/netns/bridge"
	"github.com/genuinetools/netns/network"
	"github.com/genuinetools/netns/version"
	"github.com/genuinetools/pkg/cli"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
)

const (
	defaultBridgeName = "netns0"
	defaultBridgeIP   = "172.19.0.1/16"
	defaultStateDir   = "/run/github.com/genuinetools/netns"
)

var (
	ipfile   string
	staticip string

	netOpt network.Opt
	brOpt  bridge.Opt

	debug bool

	client *network.Client
)

func main() {
	// Create a new cli program.
	p := cli.NewProgram()
	p.Name = "netns"
	p.Description = "Runc hook for setting up default bridge networking"
	// Set the GitCommit and Version.
	p.GitCommit = version.GITCOMMIT
	p.Version = version.VERSION

	// Build the list of available commands.
	p.Commands = []cli.Command{
		&createCommand{},
		&listCommand{},
		&removeCommand{},
	}

	// Setup the global flags.
	p.FlagSet = flag.NewFlagSet("global", flag.ExitOnError)
	p.FlagSet.StringVar(&ipfile, "ipfile", ".ip", "file in which to save the containers ip address")

	p.FlagSet.StringVar(&netOpt.ContainerInterface, "iface", network.DefaultContainerInterface, "name of interface in the namespace")
	p.FlagSet.StringVar(&netOpt.StateDir, "state-dir", defaultStateDir, "directory for saving state, used for ip allocation")

	p.FlagSet.StringVar(&brOpt.Name, "bridge", defaultBridgeName, "name for bridge")
	p.FlagSet.StringVar(&brOpt.IPAddr, "ip", defaultBridgeIP, "ip address for bridge")
	p.FlagSet.IntVar(&brOpt.MTU, "mtu", bridge.DefaultMTU, "mtu for bridge")

	p.FlagSet.BoolVar(&debug, "d", false, "enable debug logging")
	p.FlagSet.StringVar(&staticip, "static-ip", "", "Enable static IP Address")

	// Set the before function.
	p.Before = func(ctx context.Context) error {
		// Set the log level.
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}

		netOpt.BridgeName = brOpt.Name

		// Create the network client.
		var err error
		client, err = network.New(netOpt)
		return err
	}

	// Set the main program action.
	p.Action = func(ctx context.Context, args []string) error {
		hook, err := readHookData()
		if err != nil {
			return err
		}

		ip, err := client.Create(hook, brOpt, staticip)
		if err != nil {
			return err
		}

		// Save the ip to a file so other hooks can use it.
		if err := ioutil.WriteFile(ipfile, []byte(ip.String()), 0755); err != nil {
			return fmt.Errorf("saving allocated ip address for container to %s failed: %v", ipfile, err)
		}

		return nil
	}

	// Run our program.
	p.Run()
}

// readHookData decodes stdin as HookState.
func readHookData() (hook configs.HookState, err error) {
	// Read hook data from stdin.
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return hook, fmt.Errorf("reading hook data from stdin failed: %v", err)
	}

	// Umarshal the hook state.
	if err := json.Unmarshal(b, &hook); err != nil {
		return hook, fmt.Errorf("unmarshaling stdin as HookState failed: %v", err)
	}

	logrus.Debugf("hooks state: %#v", hook)

	return hook, nil
}
