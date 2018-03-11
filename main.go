package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/genuinetools/netns/version"
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

	defaultContainerInterface = "eth0"
	defaultPortPrefix         = "netnsv0"
	defaultBridgeName         = "netns0"
	defaultMTU                = 1500
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
	flag.IntVar(&mtu, "mtu", defaultMTU, "mtu for bridge")
	flag.StringVar(&stateDir, "state-dir", defaultStateDir, "directory for saving state, used for ip allocation")

	flag.StringVar(&ipfile, "ipfile", ".ip", "file in which to save the containers ip address")

	flag.BoolVar(&vrsn, "version", false, "print version and exit")
	flag.BoolVar(&vrsn, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "d", false, "run in debug mode")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, version.VERSION))
		flag.PrintDefaults()
	}
}

func main() {
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
		logrus.Error(strings.Join(ignored, " "))
		usageAndExit("Flags must be placed before command", 1)
	}

	if arg == "help" {
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
		if err := destroyNetwork(); err != nil {
			logrus.Fatal(err)
		}
	case "createbr":
		if err := initBridge(); err != nil {
			logrus.Fatal(err)
		}
	case "delbr":
		if err := deleteBridge(); err != nil {
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
