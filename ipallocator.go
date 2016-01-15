package main

import (
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/erikh/ping"
	"github.com/vishvananda/netlink"
)

func allocateIP(bridgeName string, firstip net.IP, ipNet *net.IPNet) (newip net.IP, err error) {
	br, err := net.InterfaceByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("Getting bridge interface %s failed: %v", bridgeName, err)
	}

	var list []netlink.Neigh
	if ipNet.IP.To4() == nil {
		list, err = netlink.NeighList(br.Index, netlink.FAMILY_V6)
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve IPv6 neighbor information for interface %s: %v", bridgeName, err)
		}
	} else {
		list, err = netlink.NeighList(br.Index, netlink.FAMILY_V4)
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve IPv4 neighbor information for interface %s: %v", bridgeName, err)
		}
	}

	ipMap := map[string]struct{}{}
	for _, entry := range list {
		ipMap[entry.String()] = struct{}{}
	}

	var cycled bool
	for {
		rawip := ipToBigInt(firstip)

		rawip.Add(rawip, big.NewInt(1))
		newip = bigIntToIP(rawip)

		if !ipNet.Contains(newip) {
			if cycled {
				return nil, fmt.Errorf("Could not find a suitable IP in network %s", ipNet.String())
			}

			firstip = ipNet.IP
			cycled = true
		}

		if _, ok := ipMap[newip.String()]; !ok {
			// use ICMP to check if the IP is in use, final sanity check.
			if !ping.Ping(&net.IPAddr{IP: newip, Zone: ""}, 150*time.Millisecond) {
				ipMap[newip.String()] = struct{}{}
				break
			} else if err != nil {
				//return nil, err
			}
		}

		firstip = newip
	}

	return newip, nil
}

// Converts a 4 bytes IP into a 128 bit integer
func ipToBigInt(ip net.IP) *big.Int {
	x := big.NewInt(0)
	if ip4 := ip.To4(); ip4 != nil {
		return x.SetBytes(ip4)
	}
	if ip6 := ip.To16(); ip6 != nil {
		return x.SetBytes(ip6)
	}

	logrus.Warnf("ipToBigInt: Wrong IP length! %s", ip)
	return nil
}

// Converts 128 bit integer into a 4 bytes IP address
func bigIntToIP(v *big.Int) net.IP {
	return net.IP(v.Bytes())
}
