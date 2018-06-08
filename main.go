package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/genuinetools/netns/bridge"
	"github.com/genuinetools/netns/ipallocator"
	"github.com/genuinetools/netns/netutils"
	"github.com/genuinetools/netns/version"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
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

	defaultContainerInterface = "eth0"
	defaultPortPrefix         = "netnsv0"
	defaultBridgeName         = "netns0"
	defaultStateDir           = "/run/github.com/genuinetools/netns"
)

var (
	arg                string
	bridgeName         string
	containerInterface string
	ipAddr             string
	mtu                int
	stateDir           string

	ipfile string

	debug bool
	vrsn  bool
)

func init() {
	// Parse flags
	flag.StringVar(&bridgeName, "bridge", defaultBridgeName, "name for bridge")
	flag.StringVar(&containerInterface, "iface", defaultContainerInterface, "name of interface in the namespace")
	flag.StringVar(&ipAddr, "ip", "172.19.0.1/16", "ip address for bridge")
	flag.IntVar(&mtu, "mtu", bridge.DefaultMTU, "mtu for bridge")
	flag.StringVar(&stateDir, "state-dir", defaultStateDir, "directory for saving state, used for ip allocation")

	flag.StringVar(&ipfile, "ipfile", ".ip", "file in which to save the containers ip address")

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

	if flag.Args()[0] == "help" {
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
}

func main() {
	switch arg {
	case "ls":
		if err := listNetworks(); err != nil {
			logrus.Fatal(err)
		}
	case "list":
		if err := listNetworks(); err != nil {
			logrus.Fatal(err)
		}
	case "delete":
		if err := bridge.Delete(bridgeName); err != nil {
			logrus.Fatal(err)
		}
	case "createbr":
		if err := bridge.Init(bridgeName, &bridge.Opt{
			MTU:    mtu,
			IPAddr: ipAddr,
		}); err != nil {
			logrus.Fatal(err)
		}
	case "delbr":
		if err := bridge.Delete(bridgeName); err != nil {
			logrus.Fatal(err)
		}
	case "":
		if err := createNetwork(); err != nil {
			logrus.Fatal(err)
		}
	default:
		logrus.Fatalf("Unknown command %s", arg)
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

func createNetwork() error {
	// Get hook data.
	h, err := readHookData()
	if err != nil {
		return err
	}

	// Initialize the bridge.
	if err := bridge.Init(bridgeName, &bridge.Opt{
		MTU:    mtu,
		IPAddr: ipAddr,
	}); err != nil {
		return err
	}

	// Create and attach local name to the bridge.
	localVethPair, err := vethPair(h.Pid, bridgeName)
	if err != nil {
		return fmt.Errorf("getting vethpair failed for pid %d: %v", h.Pid, err)
	}

	if err := netlink.LinkAdd(localVethPair); err != nil {
		return fmt.Errorf("create veth pair named [ %#v ] failed: %v", localVethPair, err)
	}

	// Get the peer link
	peer, err := netlink.LinkByName(localVethPair.PeerName)
	if err != nil {
		return fmt.Errorf("getting peer interface %s failed: %v", localVethPair.PeerName, err)
	}

	// Put peer interface into the network namespace of specified PID.
	if err := netlink.LinkSetNsPid(peer, h.Pid); err != nil {
		return fmt.Errorf("adding peer interface to network namespace of pid %d failed: %v", h.Pid, err)
	}

	// Bring the veth pair up.
	if err := netlink.LinkSetUp(localVethPair); err != nil {
		return fmt.Errorf("bringing local veth pair [ %#v ] up failed: %v", localVethPair, err)
	}

	// Check the bridge IPNet as it may be different than the default.
	brNet, err := netutils.GetInterfaceAddr(bridgeName)
	if err != nil {
		return fmt.Errorf("retrieving IP/network of bridge %s failed: %v", bridgeName, err)
	}

	// Allocate an ip address for the interface.
	ip, ipNet, err := net.ParseCIDR(brNet.String())
	if err != nil {
		return fmt.Errorf("parsing CIDR for %s failed: %v", ipAddr, err)
	}

	ipAllocator, err := ipallocator.New(bridgeName, stateDir, ipNet)
	if err != nil {
		return err
	}
	nsip, err := ipAllocator.Allocate(h.Pid)
	if err != nil {
		return fmt.Errorf("Allocating ip address failed: %v", err)
	}

	newIP := &net.IPNet{
		IP:   nsip,
		Mask: ipNet.Mask,
	}

	// Configure the interface in the network namespace.
	if err := configureInterface(localVethPair.PeerName, h.Pid, newIP, ip.String()); err != nil {
		return err
	}

	logrus.Infof("Attached veth (%s) to bridge (%s)", localVethPair.Name, bridgeName)

	// save the ip to a file so other hooks can use it.
	if err := ioutil.WriteFile(ipfile, []byte(nsip.String()), 0755); err != nil {
		return fmt.Errorf("Saving allocated ip address for container to %s failed: %v", ipfile, err)
	}

	return nil
}

// configureInterface configures the network interface in the network namespace.
func configureInterface(name string, pid int, addr *net.IPNet, gatewayIP string) error {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save the current network namespace
	origns, err := netns.Get()
	if err != nil {
		return fmt.Errorf("Getting current network namespace failed: %v", err)
	}
	defer origns.Close()

	// Get the namespace
	newns, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("Getting network namespace for pid %d failed: %v", pid, err)
	}
	defer newns.Close()

	// Enter the namespace
	if err := netns.Set(newns); err != nil {
		return fmt.Errorf("Entering network namespace failed: %v", err)
	}

	// Find the network interface identified by the name
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("Getting link by name %s failed: %v", name, err)
	}

	// Bring the interface down
	if err := netlink.LinkSetDown(iface); err != nil {
		return fmt.Errorf("Bringing interface [ %#v ] down failed: %v", iface, err)
	}

	// Change the interface name to eth0 in the namespace
	if err := netlink.LinkSetName(iface, containerInterface); err != nil {
		return fmt.Errorf("Renaming interface %s to %s failed: %v", name, defaultContainerInterface, err)
	}

	// Add the IP address
	ipAddr := &netlink.Addr{IPNet: addr, Label: ""}
	if err := netlink.AddrAdd(iface, ipAddr); err != nil {
		return fmt.Errorf("Setting interface %s ip to %s failed: %v", name, addr.String(), err)
	}

	// Bring the interface up
	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("Bringing interface [ %#v ] up failed: %v", iface, err)
	}

	// Add the gateway route
	gw := net.ParseIP(gatewayIP)
	err = netlink.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: iface.Attrs().Index,
		Gw:        gw,
	})
	if err != nil {
		return fmt.Errorf("Adding route %s to interface %s failed: %v", gw.String(), name, err)
	}

	// Switch back to the original namespace
	if err := netns.Set(origns); err != nil {
		return fmt.Errorf("Switching back to original namespace failed: %v", err)
	}

	return nil
}

// readHookData decodes stdin as HookState
func readHookData() (hook configs.HookState, err error) {
	// Read hook data from stdin
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return hook, fmt.Errorf("Reading hook data from stdin failed: %v", err)
	}

	// Umarshal the hook state
	if err := json.Unmarshal(b, &hook); err != nil {
		return hook, fmt.Errorf("Unmarshal stdin as HookState failed: %v", err)
	}

	logrus.Debugf("Hooks State: %#v", hook)
	return hook, nil

}

// vethPair creates a veth pair. Peername is renamed to eth0 in the container.
func vethPair(pid int, bridgeName string) (*netlink.Veth, error) {
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, err
	}

	la := netlink.NewLinkAttrs()
	la.Name = fmt.Sprintf("%s-%d", defaultPortPrefix, pid)
	la.MasterIndex = br.Attrs().Index

	return &netlink.Veth{
		LinkAttrs: la,
		PeerName:  fmt.Sprintf("ethc%d", pid),
	}, nil
}
