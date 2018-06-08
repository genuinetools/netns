package network

import (
	"fmt"
	"os"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/vishvananda/netns"
)

// List returns the ip addresses being used from the database for the networks
// with the specified bridge name.
func (c *Client) List() ([]Network, error) {
	// Open the database.
	if err := c.openDB(true); err != nil {
		return nil, err
	}
	defer c.db.Close()

	var (
		networks = []Network{}
		err      error
	)
	if err := c.db.View(func(tx *bolt.Tx) error {
		// Retrieve the networks from the bucket.
		b := tx.Bucket(ipBucket)

		return b.ForEach(func(k, v []byte) error {
			// skip last ip
			if len(k) == 1 && k[0] == 0 {
				return nil
			}

			n := Network{
				IP: k,
			}

			// Get the pid.
			n.PID, err = strconv.Atoi(string(v))
			if err != nil {
				return fmt.Errorf("parsing pid %s as int failed: %v", v, err)
			}

			// Check the process.
			_, err := os.FindProcess(n.PID)
			if err != nil {
				n.Status = "does not exist"
			} else {
				n.Status = "running"
			}

			// Get the veth pair from the pid.
			n.VethPair, err = c.vethPair(n.PID, c.opt.BridgeName)
			if err != nil {
				return fmt.Errorf("getting vethpair %d failed: %v", n.PID, err)
			}

			// Try to get the namespace handle.
			n.FD, _ = netns.GetFromPid(n.PID)
			if n.FD <= 0 {
				n.Status = "destroyed"
			}

			networks = append(networks, n)

			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("getting networks failed: %v", err)
	}

	return networks, nil
}
