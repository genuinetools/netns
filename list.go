package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"text/tabwriter"

	"github.com/boltdb/bolt"
	"github.com/genuinetools/netns/ipallocator"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type network struct {
	vethPair *netlink.Veth
	ip       net.IP
	pid      int
	status   string
	fd       netns.NsHandle
}

func listNetworks() error {
	// Open the database.
	dbpath := path.Join(stateDir, ipallocator.DBFile)
	db, err := bolt.Open(dbpath, 0666, nil)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("You have not allocated any IPs")
		}
		return fmt.Errorf("opening database at %s failed: %v", dbpath, err)
	}
	defer db.Close()

	var networks []network
	if err := db.View(func(tx *bolt.Tx) error {
		// Retrieve the jobs bucket.
		b := tx.Bucket(ipallocator.IPBucket)

		return b.ForEach(func(k, v []byte) error {
			// skip last ip
			if len(k) == 1 && k[0] == 0 {
				return nil
			}

			n := network{
				ip: k,
			}

			// Get the pid.
			n.pid, err = strconv.Atoi(string(v))
			if err != nil {
				return fmt.Errorf("parsing pid %s as int failed: %v", v, err)
			}

			// Check the process.
			_, err := os.FindProcess(n.pid)
			if err != nil {
				n.status = "does not exist"
			} else {
				n.status = "running"
			}

			// Get the veth pair from the pid.
			n.vethPair, err = vethPair(n.pid, bridgeName)
			if err != nil {
				return fmt.Errorf("Getting vethpair failed for pid %d: %v", n.pid, err)
			}

			// Try to get the namespace handle.
			n.fd, _ = netns.GetFromPid(n.pid)
			if n.fd <= 0 {
				n.status = "destroyed"
			}

			networks = append(networks, n)

			return nil
		})
	}); err != nil {
		return fmt.Errorf("getting networks from db failed: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprint(w, "IP\tLOCAL VETH\tPID\tSTATUS\tNS FD\n")
	for _, n := range networks {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%d\n", n.ip.String(), n.vethPair.Attrs().Name, n.pid, n.status, n.fd)
	}
	w.Flush()

	return nil
}
