package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
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
	defaultStateDir           = "/run/github.com/jfrazelle/netns"
)

var (
	arg                string
	bridgeName         string
	containerInterface string
	ipAddr             string
	mtu                int
	stateDir           string

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
	flag.StringVar(&stateDir, "state-dir", defaultStateDir, "directory for saving state, used for ip allocation")

	flag.StringVar(&ipfile, "ipfile", ".ip", "file in which to save the containers ip address")

	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&version, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "d", false, "run in debug mode")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, VERSION))
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() >= 1 {
		arg = flag.Args()[0]
	}

	if arg == "help" {
		usageAndExit("", 0)
	}

	if version || arg == "version" {
		fmt.Printf("%s", VERSION)
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
	default:
		if err := createNetwork(); err != nil {
			logrus.Fatal(err)
		}
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
