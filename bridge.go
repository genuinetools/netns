package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/docker/libnetwork/iptables"
	"github.com/vishvananda/netlink"
)

// initBridge creates a bridge if it does not exist
func initBridge() error {
	_, err := net.InterfaceByName(bridgeName)
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// Create *netlink.Bridge object
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	la.MTU = mtu
	br := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("Bridge creation failed: %v", err)
	}

	// Setup ip address for bridge
	addr, err := netlink.ParseAddr(ipAddr)
	if err != nil {
		return fmt.Errorf("Parsing address %s: %v", ipAddr, err)
	}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return fmt.Errorf("Adding address %v to bridge %s failed: %v", addr, bridgeName, err)
	}

	// Validate that the IPAddress is there!
	_, err = getIfaceAddr(bridgeName)
	if err != nil {
		return fmt.Errorf("No IP address found on bridge %s", bridgeName)
	}

	// Add NAT rules for iptables
	if err = natOut(ipAddr, iptables.Insert); err != nil {
		return fmt.Errorf("Could not set NAT rules for bridge %s", bridgeName)
	}

	// Bring the bridge up
	return netlink.LinkSetUp(br)
}

// deleteBridge deletes the bridge.
func deleteBridge() error {
	// Get the link
	l, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("Getting link with name %s failed: %v", bridgeName, err)
	}

	// Delete the link
	if err := netlink.LinkDel(l); err != nil {
		return fmt.Errorf("Failed to remove bridge interface %s delete: %v", bridgeName, err)
	}

	return nil
}
