package main

import (
	"fmt"
	"net"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

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
	if err := netlink.LinkSetName(iface, defaultContainerInterface); err != nil {
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

// vethPair creates a veth pair. Peername is renamed to eth0 in the container.
func vethPair(suffix string, bridgeName string) (*netlink.Veth, error) {
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, err
	}

	la := netlink.NewLinkAttrs()
	la.Name = defaultPortPrefix + suffix
	la.MasterIndex = br.Attrs().Index

	return &netlink.Veth{
		LinkAttrs: la,
		PeerName:  "ethc" + suffix,
	}, nil
}
