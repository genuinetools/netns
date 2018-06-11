package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/genuinetools/netns/bridge"
	"github.com/genuinetools/netns/network"
	"github.com/genuinetools/netns/version"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
)

const (
	// BANNER is what is printed for help/info output
	BANNER = `            _
 _ __   ___| |_ _ __  ___
| '_ \ / _ \ __| '_ \/ __|
| | | |  __/ |_| | | \__ \
|_| |_|\___|\__|_| |_|___/

 Runc hook for setting up default bridge networking.
 Version: %s

 Netns provides the following commands. Usage format:

    netns [-flag value] [-flag value] command

  Where command is one of:

    createbr, delbr, [ls|list], delete

  If command is blank (e.g. when called via a hook) it
  will create a network endpoint in the expected net
  namespace details for that PID.

`

	defaultBridgeName = "netns0"
	defaultBridgeIP   = "172.19.0.1/16"
	defaultStateDir   = "/run/github.com/genuinetools/netns"
)

var (
	arg    string
	ipfile string

	netOpt network.Opt
	brOpt  bridge.Opt

	debug bool
	vrsn  bool
)

func init() {
	// Parse flags
	flag.StringVar(&ipfile, "ipfile", ".ip", "file in which to save the containers ip address")

	flag.StringVar(&netOpt.ContainerInterface, "iface", network.DefaultContainerInterface, "name of interface in the namespace")
	flag.StringVar(&netOpt.StateDir, "state-dir", defaultStateDir, "directory for saving state, used for ip allocation")

	flag.StringVar(&brOpt.Name, "bridge", defaultBridgeName, "name for bridge")
	flag.StringVar(&brOpt.IPAddr, "ip", defaultBridgeIP, "ip address for bridge")
	flag.IntVar(&brOpt.MTU, "mtu", bridge.DefaultMTU, "mtu for bridge")

	flag.BoolVar(&vrsn, "version", false, "print version and exit")
	flag.BoolVar(&vrsn, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "d", false, "run in debug mode")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, version.VERSION))
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() == 1 {
		arg = flag.Args()[0]
	}

	if flag.NArg() > 1 {
		ignored := []string{"Ignoring parameters:"}
		argList := flag.Args()[1:]
		for i := range argList {
			ignored = append(ignored, argList[i])
		}
		usageAndExit("Flags must be placed before the command. "+strings.Join(ignored, " "), 1)
	}

	if len(flag.Args()) > 0 && flag.Args()[0] == "help" {
		usageAndExit("", 0)
	}

	if vrsn || arg == "version" {
		fmt.Printf("netns version %s, build %s", version.VERSION, version.GITCOMMIT)
		os.Exit(0)
	}

	// Set log level
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	netOpt.BridgeName = brOpt.Name
}

func main() {
	// Create the network client.
	client, err := network.New(netOpt)
	if err != nil {
		logrus.Fatal(err)
	}

	switch arg {
	case "ls":
		networks, err := client.List()
		if err != nil {
			logrus.Fatal(err)
		}

		// Print the networks.
		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprint(w, "IP\tLOCAL VETH\tPID\tSTATUS\tNS FD\n")
		for _, n := range networks {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%d\n", n.IP.String(), n.VethPair.Attrs().Name, n.PID, n.Status, n.FD)
		}
		w.Flush()
	case "list":
		networks, err := client.List()
		if err != nil {
			logrus.Fatal(err)
		}

		// Print the networks.
		w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
		fmt.Fprint(w, "IP\tLOCAL VETH\tPID\tSTATUS\tNS FD\n")
		for _, n := range networks {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%d\n", n.IP.String(), n.VethPair.Attrs().Name, n.PID, n.Status, n.FD)
		}
		w.Flush()
	case "delete":
		if err := bridge.Delete(brOpt.Name); err != nil {
			logrus.Fatal(err)
		}
	case "createbr":
		if _, err := bridge.Init(brOpt); err != nil {
			logrus.Fatal(err)
		}
	case "delbr":
		if err := bridge.Delete(brOpt.Name); err != nil {
			logrus.Fatal(err)
		}
	case "":
		hook, err := readHookData()
		if err != nil {
			logrus.Fatal(err)
		}

		ip, err := client.Create(hook, brOpt)
		if err != nil {
			logrus.Fatal(err)
		}

		// Save the ip to a file so other hooks can use it.
		if err := ioutil.WriteFile(ipfile, []byte(ip.String()), 0755); err != nil {
			logrus.Fatalf("saving allocated ip address for container to %s failed: %v", ipfile, err)
		}
	default:
		logrus.Fatalf("unknown command %s", arg)
	}
}

func usageAndExit(message string, exitCode int) {
	if message != "" {
		fmt.Fprintf(os.Stderr, message)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(exitCode)
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
