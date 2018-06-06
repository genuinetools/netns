package ipallocator

import (
	"fmt"
	"math/big"
	"net"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/erikh/ping"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

var (
	// DBFile is the file the bolt ndatabase is stored in.
	DBFile = "bolt.db"
	// IPBucket is the bolt database bucket for ip key value store.
	IPBucket = []byte("ipallocator")
)

// IPAllocator defines the data structure for allocating a new IP.
type IPAllocator struct {
	Bridge *net.Interface
	IPNet  *net.IPNet
	db     *bolt.DB
}

// New returns a new instance of IPAllocator for the bridge interface passed.
func New(bridgeName, stateDir string, ipNet *net.IPNet) (*IPAllocator, error) {
	if err := os.MkdirAll(stateDir, 0666); err != nil {
		return nil, fmt.Errorf("attempt to create state directory %s failed: %v", stateDir, err)
	}

	// open the database
	// this will block until closed which is file for our use case of assigning
	// one IP and being done.
	// TODO: make this more graceful if someone else wants to use this as a lib.
	dbpath := path.Join(stateDir, DBFile)
	db, err := bolt.Open(dbpath, 0666, nil)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("You have not allocated any IPs")
		}
		return nil, fmt.Errorf("Opening database at %s failed: %v", dbpath, err)
	}

	// create the ip allocator bucket if it does not exist
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(IPBucket); err != nil {
			return fmt.Errorf("Creating bucket %s failed: %v", IPBucket, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	br, err := net.InterfaceByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("Getting bridge interface %s failed: %v", bridgeName, err)
	}

	ipAllocator := &IPAllocator{
		Bridge: br,
		IPNet:  ipNet,
		db:     db,
	}

	return ipAllocator, nil
}

// Allocate returns an unused IP from the IPAllocator
func (i *IPAllocator) Allocate(pid int) (ip net.IP, err error) {
	// refresh ipMap
	ipMap, err := i.getIPMap()
	if err != nil {
		return nil, err
	}

	// find the last IP used by the allocator
	lastip := i.IPNet.IP
	if err := i.db.View(func(tx *bolt.Tx) error {
		k, _ := tx.Bucket(IPBucket).Cursor().Last()

		if string(k) != "" {
			lastip = k
		}

		return nil
	}); err != nil {
		return nil, err
	}

	var cycled bool
	for {
		rawip := ipToBigInt(lastip)

		rawip.Add(rawip, big.NewInt(1))
		ip = bigIntToIP(rawip)

		if !i.IPNet.Contains(ip) {
			if cycled {
				return nil, fmt.Errorf("Could not find a suitable IP in network %s", i.IPNet.String())
			}

			lastip = i.IPNet.IP
			cycled = true
		}

		if _, ok := ipMap[ip.String()]; !ok {
			// use ICMP to check if the IP is in use, final sanity check.
			if !ping.Ping(&net.IPAddr{IP: ip, Zone: ""}, 150*time.Millisecond) {
				ipMap[ip.String()] = struct{}{}
				break
			} else if err != nil {
				// for now just keep cycling
				// return nil, err
				logrus.Debugf("[ipallocator]: ping ip %s failed: %v", ip.String(), err)
			}
		}

		lastip = ip
	}

	// save the new ip in the database
	if err := i.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(IPBucket).Put(ip, []byte(strconv.Itoa(pid)))
	}); err != nil {
		return nil, fmt.Errorf("Adding ip %s to database for %d failed: %v", ip.String(), pid, err)
	}

	return ip, nil
}

func (i *IPAllocator) getIPMap() (map[string]struct{}, error) {
	// get the neighbors
	var (
		list []netlink.Neigh
		err  error
	)
	if i.IPNet.IP.To4() == nil {
		list, err = netlink.NeighList(i.Bridge.Index, netlink.FAMILY_V6)
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve IPv6 neighbor information for interface %s: %v", i.Bridge.Name, err)
		}
	} else {
		list, err = netlink.NeighList(i.Bridge.Index, netlink.FAMILY_V4)
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve IPv4 neighbor information for interface %s: %v", i.Bridge.Name, err)
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
