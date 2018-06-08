package network

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/erikh/ping"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// AllocateIP returns an unused IP for a specific process ID
// and saves it in the database.
func (c *Client) AllocateIP(pid int) (ip net.IP, err error) {
	// Refresh the ipMap.
	ipMap, err := c.getIPMap()
	if err != nil {
		return nil, err
	}

	// Find the last IP used by the allocator.
	lastip := c.ipNet.IP
	if err := c.db.View(func(tx *bolt.Tx) error {
		if result := tx.Bucket(ipBucket).Get([]byte{0}); result != nil {
			lastip = result
		}
		return nil
	}); err != nil {
		return nil, err
	}

	bridgeAddrs, _ := c.bridge.Addrs()

	ip = increaseIP(lastip)

	for {
		switch {
		case !c.ipNet.Contains(ip):
			ip = c.ipNet.IP

		// Skip bridge IP.
		case func() bool {
			for _, addr := range bridgeAddrs {
				itfIP, _, _ := net.ParseCIDR(addr.String())
				if ip.Equal(itfIP) {
					return true
				}
			}
			return false
		}():
			logrus.Debugf("[ipallocator] ip %s belongs to the bridge. Skipped.", ip.String())

		// Skip broadcast ip
		case !isUnicastIP(ip, c.ipNet.Mask):
			logrus.Debugf("[ipallocator] ip %s is not unicast. Skipped.", ip.String())

		case !func() bool { _, ok := ipMap[ip.String()]; return ok }():
			// use ICMP to check if the IP is in use, final sanity check.
			if !ping.Ping(&net.IPAddr{IP: ip, Zone: ""}, 150*time.Millisecond) {
				// save the new ip in the database
				if err := c.db.Update(func(tx *bolt.Tx) error {
					if err := tx.Bucket(ipBucket).Put(ip, []byte(strconv.Itoa(pid))); err != nil {
						return err
					}
					if err := tx.Bucket(ipBucket).Put([]byte{0}, ip); err != nil {
						return err
					}
					return nil
				}); err != nil {
					return nil, fmt.Errorf("Adding ip %s to database for %d failed: %v", ip.String(), pid, err)
				}
				logrus.Debugf("[ipallocator] ip %s is selected.", ip.String())

				return ip, nil
			}

			logrus.Debugf("[ipallocator] ip %s is already allocated. Skipped.", ip.String())
		}

		ip = increaseIP(ip)

		if ip.Equal(increaseIP(lastip)) {
			break
		}
	}

	return nil, fmt.Errorf("could not find a suitable IP in network %s", c.ipNet.String())
}

func (c *Client) getIPMap() (map[string]struct{}, error) {
	// get the neighbors
	var (
		list []netlink.Neigh
		err  error
	)
	if c.ipNet.IP.To4() == nil {
		list, err = netlink.NeighList(c.bridge.Index, netlink.FAMILY_V6)
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve IPv6 neighbor information for interface %s: %v", c.bridge.Name, err)
		}
	} else {
		list, err = netlink.NeighList(c.bridge.Index, netlink.FAMILY_V4)
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve IPv4 neighbor information for interface %s: %v", c.bridge.Name, err)
		}
	}

	ipMap := map[string]struct{}{}
	for _, entry := range list {
		ipMap[entry.String()] = struct{}{}
	}

	return ipMap, nil
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

// Increases IP address
func increaseIP(ip net.IP) net.IP {
	rawip := ipToBigInt(ip)
	rawip.Add(rawip, big.NewInt(1))
	return bigIntToIP(rawip)
}

func isUnicastIP(ip net.IP, mask net.IPMask) bool {
	// broadcast v4 ip
	if len(ip) == net.IPv4len && binary.BigEndian.Uint32(ip)&^binary.BigEndian.Uint32(mask) == ^binary.BigEndian.Uint32(mask) {
		return false
	}

	// global unicast
	return ip.IsGlobalUnicast()
}
