package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime"

	"github.com/docker/libnetwork/iptables"
	"github.com/jessfraz/netns/ipallocator"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func createNetwork() error {
	// Get hook data
	h, err := readHookData()
	if err != nil {
		return err
	}

	// Initialize the bridge
	if err := initBridge(); err != nil {
		return err
	}

	// Create and attach local name to the bridge
	localVethPair, err := vethPair(h.Pid, bridgeName)
	if err != nil {
		return fmt.Errorf("Getting vethpair failed for pid %d: %v", h.Pid, err)
	}

	if err := netlink.LinkAdd(localVethPair); err != nil {
		return fmt.Errorf("Create veth pair named [ %#v ] failed: %v", localVethPair, err)
	}

	// Get the peer link
	peer, err := netlink.LinkByName(localVethPair.PeerName)
	if err != nil {
		return fmt.Errorf("Getting peer interface (%s) failed: %v", localVethPair.PeerName, err)
	}

	// Put peer interface into the network namespace of specified PID
	if err := netlink.LinkSetNsPid(peer, h.Pid); err != nil {
		return fmt.Errorf("Adding peer interface to network namespace of pid %d failed: %v", h.Pid, err)
	}

	// Bring the veth pair up
	if err := netlink.LinkSetUp(localVethPair); err != nil {
		return fmt.Errorf("Bringing local veth pair [ %#v ] up failed: %v", localVethPair, err)
	}

	// check the bridge IPNet as it may be different than the default
	brNet, err := getIfaceAddr(bridgeName)
	if err != nil {
		return fmt.Errorf("Retrieving IP/network of bridge %s failed: %v", bridgeName, err)
	}
	// Allocate an ip address for the interface
	ip, ipNet, err := net.ParseCIDR(brNet.String())
	if err != nil {
		return fmt.Errorf("Parsing CIDR for %s failed: %v", ipAddr, err)
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
	// Configure the interface in the network namespace
	if err := configureInterface(localVethPair.PeerName, h.Pid, newIP, ip.String()); err != nil {
		return err
	}

	logrus.Infof("Attached veth (%s) to bridge (%s)", localVethPair.Name, bridgeName)

	// save the ip to a file so other hooks can use it
	if err := ioutil.WriteFile(ipfile, []byte(nsip.String()), 0755); err != nil {
		return fmt.Errorf("Saving allocated ip address for container to %s failed: %v", ipfile, err)
	}

	return nil
}

func destroyNetwork() error {
	// Destroy the bridge
	return deleteBridge()
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

// getIfaceAddr returns the IPv4 address of a network interface.
func getIfaceAddr(name string) (*net.IPNet, error) {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := netlink.AddrList(iface, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("Interface %s has no IP addresses", name)
	}
	if len(addrs) > 1 {
		logrus.Infof("Interface [ %#v ] has more than 1 IPv4 address. Defaulting to using [ %#v ]\n", name, addrs[0].IP)
	}
	return addrs[0].IPNet, nil
}

// natOut adds NAT rules for iptables.
func natOut(cidr string, action iptables.Action) error {
	masquerade := []string{
		"POSTROUTING", "-t", "nat",
		"-s", cidr,
		"-j", "MASQUERADE",
	}

	incl := append([]string{string(action)}, masquerade...)
	if _, err := iptables.Raw(
		append([]string{"-C"}, masquerade...)...,
	); err != nil || action == iptables.Delete {
		if output, err := iptables.Raw(incl...); err != nil {
			return err
		} else if len(output) > 0 {
			return &iptables.ChainError{
				Chain:  "POSTROUTING",
				Output: output,
			}
		}
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
