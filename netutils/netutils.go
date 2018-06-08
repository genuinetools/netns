package netutils

import (
	"fmt"
	"net"

	"github.com/docker/libnetwork/iptables"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// GetInterfaceAddr returns the IPv4 address of a network interface.
func GetInterfaceAddr(name string) (*net.IPNet, error) {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("getting interface %s failed: %v", name, err)
	}

	addrs, err := netlink.AddrList(iface, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("listings addresses for %s failed: %v", name, err)
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("interface %s has no IP addresses", name)
	}

	if len(addrs) > 1 {
		logrus.Debugf("interface %s has more than 1 IPv4 address, using: %s", name, addrs[0].IP.String())
	}

	return addrs[0].IPNet, nil
}

// SetupNATOut adds NAT rules for outbound traffic with iptables.
func SetupNATOut(cidr string, action iptables.Action) error {
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
