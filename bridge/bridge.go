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

var (
	// ErrIPAddrEmpty holds the error for when the ip address is empty.
	ErrIPAddrEmpty = errors.New("ip address cannot be empty")
	// ErrNameEmpty holds the error for when the name is empty.
	ErrNameEmpty = errors.New("name cannot be empty")
)

// Opt holds the options for the bridge interface.
type Opt struct {
	MTU    int
	IPAddr string
	Name   string
}

// Init creates a bridge with the name specified if it does not exist.
func Init(opt Opt) (*net.Interface, error) {
	// Validate the options.
	if len(opt.IPAddr) < 1 {
		return nil, ErrIPAddrEmpty
	}
	if len(opt.Name) < 1 {
		return nil, ErrNameEmpty
	}

	// Set the defaults.
	if opt.MTU < 1 {
		opt.MTU = DefaultMTU
	}

	bridge, err := net.InterfaceByName(opt.Name)
	if err == nil {
		// Bridge already exists, return early.
		return bridge, nil
	}

	if !strings.Contains(err.Error(), "no such network interface") {
		return nil, fmt.Errorf("getting interface %s failed: %v", opt.Name, err)
	}

	// Create *netlink.Bridge object.
	la := netlink.NewLinkAttrs()
	la.Name = opt.Name
	la.MTU = opt.MTU
	br := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkAdd(br); err != nil {
		return nil, fmt.Errorf("bridge creation for %s failed: %v", opt.Name, err)
	}

	// Setup ip address for bridge.
	addr, err := netlink.ParseAddr(opt.IPAddr)
	if err != nil {
		return nil, fmt.Errorf("parsing address %s failed: %v", opt.IPAddr, err)
	}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return nil, fmt.Errorf("adding address %s to bridge %s failed: %v", addr.String(), opt.Name, err)
	}

	// Validate that the IPAddress is there!
	if _, err := netutils.GetInterfaceAddr(opt.Name); err != nil {
		return nil, err
	}

	// Add NAT rules for iptables.
	if err := netutils.SetupNATOut(opt.IPAddr, iptables.Insert); err != nil {
		return nil, fmt.Errorf("setting up NAT outbound for %s failed: %v", opt.Name, err)
	}

	// Bring the bridge up.
	if err := netlink.LinkSetUp(br); err != nil {
		return nil, fmt.Errorf("bringing bridge %s up failed: %v", opt.Name, err)
	}

	return net.InterfaceByName(opt.Name)
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
