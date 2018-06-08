package bridge

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/docker/libnetwork/iptables"
	"github.com/genuinetools/netns/netutils"
	"github.com/vishvananda/netlink"
)

const (
	// DefaultMTU is the default MTU for new bridge interfaces.
	DefaultMTU = 1500
)

// Opt holds the options for the bridge interface.
type Opt struct {
	MTU    int
	IPAddr string
}

// Init creates a bridge with the name specified if it does not exist.
func Init(name string, opt *Opt) error {
	// TODO: add better validation of options.
	if opt == nil {
		return errors.New("bridge options cannot be nil")
	}

	_, err := net.InterfaceByName(name)
	if err == nil {
		// Bridge already exists, return early.
		return nil
	}

	if !strings.Contains(err.Error(), "no such network interface") {
		return fmt.Errorf("getting interface %s failed: %v", name, err)
	}

	// Create *netlink.Bridge object.
	la := netlink.NewLinkAttrs()
	la.Name = name
	la.MTU = opt.MTU
	br := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("bridge creation for %s failed: %v", name, err)
	}

	// Setup ip address for bridge.
	addr, err := netlink.ParseAddr(opt.IPAddr)
	if err != nil {
		return fmt.Errorf("parsing address %s failed: %v", opt.IPAddr, err)
	}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return fmt.Errorf("adding address %s to bridge %s failed: %v", addr.String(), name, err)
	}

	// Validate that the IPAddress is there!
	if _, err := netutils.GetInterfaceAddr(name); err != nil {
		return err
	}

	// Add NAT rules for iptables.
	if err := netutils.SetupNATOut(opt.IPAddr, iptables.Insert); err != nil {
		return fmt.Errorf("setting up NAT outbound for %s failed: %v", name, err)
	}

	// Bring the bridge up.
	return netlink.LinkSetUp(br)
}

// Delete removes the bridge by the specified name.
func Delete(name string) error {
	// Get the link.
	l, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("getting bridge %s failed: %v", name, err)
	}

	// Delete the link.
	if err := netlink.LinkDel(l); err != nil {
		return fmt.Errorf("deleting bridge %s failed: %v", name, err)
	}

	return nil
}
