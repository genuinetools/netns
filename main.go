package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/vishvananda/netlink"
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

`
	// VERSION is the binary version.
	VERSION = "v0.1.0"

	defaultContainerInterface = "eth0"
	defaultPortPrefix         = "netnsv0"
	defaultBridgeName         = "netns0"
	defaultMTU                = 1500
)

var (
	bridgeName         string
	containerInterface string
	ipAddr             string
	mtu                int

	ipfile string

	debug   bool
	version bool
)

func init() {
	// Parse flags
	flag.StringVar(&bridgeName, "bridge", defaultBridgeName, "name for bridge")
	flag.StringVar(&containerInterface, "iface", defaultContainerInterface, "name of interface in the namespace")
	flag.StringVar(&ipAddr, "ip", "172.19.0.1/16", "ip address for bridge")
	flag.IntVar(&mtu, "mtu", defaultMTU, "mtu for bridge")

	flag.StringVar(&ipfile, "ipfile", ".ip", "file in which to save the containers ip address")

	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&version, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "d", false, "run in debug mode")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, VERSION))
		flag.PrintDefaults()
	}

	flag.Parse()

	if version {
		fmt.Printf("%s", VERSION)
		os.Exit(0)
	}

	// Set log level
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func main() {
	// Read hook data from stdin
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		logrus.Fatalf("reading hook data from stdin failed: %v", err)
	}

	// Umarshal the hook state
	var h configs.HookState
	if err := json.Unmarshal(b, &h); err != nil {
		logrus.Fatalf("umarshal stdin as HookState failed: %v", err)
	}

	logrus.Debugf("Hooks State: %#v", h)

	// Initialize the bridge
	if err := initBridge(); err != nil {
		logrus.Fatal(err)
	}

	// Create and attach local name to the bridge
	localVethPair, err := vethPair(h.Pid, bridgeName)
	if err != nil {
		logrus.Fatalf("Getting vethpair failed: %v", err)
	}

	if err := netlink.LinkAdd(localVethPair); err != nil {
		logrus.Fatalf("Create veth pair named [ %#v ] failed: %v", localVethPair, err)
	}

	// Get the peer link
	peer, err := netlink.LinkByName(localVethPair.PeerName)
	if err != nil {
		logrus.Fatalf("Getting peer interface (%s) failed: %v", localVethPair.PeerName, err)
	}

	// Put peer interface into the network namespace of specified PID
	if err := netlink.LinkSetNsPid(peer, h.Pid); err != nil {
		logrus.Fatalf("Adding peer interface to network namespace of pid %d failed: %v", h.Pid, err)
	}

	// Bring the veth pair up
	if err := netlink.LinkSetUp(localVethPair); err != nil {
		logrus.Fatalf("Bringing local veth pair [ %#v ] up failed: %v", localVethPair, err)
	}

	// Allocate an ip address for the interface
	ip, ipNet, err := net.ParseCIDR(ipAddr)
	if err != nil {
		logrus.Fatalf("Parsing CIDR for %s failed: %v", ipAddr, err)
	}
	ipNet.IP = ip
	nsip, err := allocateIP(bridgeName, ip, ipNet)
	if err != nil {
		logrus.Fatalf("Allocating ip address failed: %v", err)
	}

	newIP := &net.IPNet{
		IP:   nsip,
		Mask: ipNet.Mask,
	}
	// Configure the interface in the network namespace
	if err := configureInterface(localVethPair.PeerName, h.Pid, newIP, ip.String()); err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Attached veth (%s) to bridge (%s)", localVethPair.Name, bridgeName)

	// save the ip to a file so other hooks can use it
	if err := ioutil.WriteFile(ipfile, []byte(nsip.String()), 0755); err != nil {
		logrus.Fatalf("Saving allocated ip address for container to %s failed: %v", ipfile, err)
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
