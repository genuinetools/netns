package network

import (
	"fmt"
	"net"
	"runtime"

	"github.com/genuinetools/netns/bridge"
	"github.com/genuinetools/netns/netutils"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	bolt "go.etcd.io/bbolt"
)

// Create returns a container IP that was created with the given bridge name,
// the settings from the HookState passed, and the bridge options.
func (c *Client) Create(hook configs.HookState, brOpt bridge.Opt, staticip string) (net.IP, error) {
	var nsip net.IP
	// Open the database.
	if err := c.openDB(false); err != nil {
		return nil, err
	}
	defer c.closeDB()

	// Initialize the bridge.
	var err error
	c.bridge, err = bridge.Init(brOpt)
	if err != nil {
		return nil, err
	}

	// Create and attach local name to the bridge.
	localVethPair, err := c.vethPair(hook.Pid, c.opt.BridgeName)
	if err != nil {
		return nil, fmt.Errorf("getting vethpair for pid %d failed: %v", hook.Pid, err)
	}
	if err := netlink.LinkAdd(localVethPair); err != nil {
		return nil, fmt.Errorf("create veth pair named [ %#v ] failed: %v", localVethPair, err)
	}

	// Get the peer link.
	peer, err := netlink.LinkByName(localVethPair.PeerName)
	if err != nil {
		return nil, fmt.Errorf("getting peer interface %s failed: %v", localVethPair.PeerName, err)
	}

	// Put peer interface into the network namespace of specified PID.
	if err := netlink.LinkSetNsPid(peer, hook.Pid); err != nil {
		return nil, fmt.Errorf("adding peer interface to network namespace of pid %d failed: %v", hook.Pid, err)
	}

	// Bring the veth pair up.
	if err := netlink.LinkSetUp(localVethPair); err != nil {
		return nil, fmt.Errorf("bringing local veth pair [ %#v ] up failed: %v", localVethPair, err)
	}

	// Check the bridge IPNet as it may be different than the default.
	brNet, err := netutils.GetInterfaceAddr(c.opt.BridgeName)
	if err != nil {
		return nil, fmt.Errorf("retrieving IP/network of bridge %s failed: %v", c.opt.BridgeName, err)
	}

	// Allocate an ip address for the interface.
	ip, ipNet, err := net.ParseCIDR(brNet.String())
	if err != nil {
		return nil, fmt.Errorf("parsing CIDR for %s failed: %v", brNet.String(), err)
	}
	c.ipNet = ipNet

	// Create the ip allocator bucket if it does not exist.
	if err := c.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(ipBucket); err != nil {
			return fmt.Errorf("creating bucket %s failed: %v", ipBucket, err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if staticip != "" {
		nsip = net.ParseIP(staticip)
	} else {
		nsip, err = c.AllocateIP(hook.Pid)
	}

	if err != nil {
		return nil, fmt.Errorf("allocating ip address failed: %v", err)
	}

	newIP := &net.IPNet{
		IP:   nsip,
		Mask: ipNet.Mask,
	}

	// Configure the interface in the network namespace.
	if err := c.configureInterface(localVethPair.PeerName, hook.Pid, newIP, ip.String()); err != nil {
		return nil, err
	}

	logrus.Debugf("attached veth (%s) to bridge (%s)", localVethPair.Name, c.opt.BridgeName)
	return nsip, nil
}

// configureInterface configures the network interface in the network namespace.
func (c *Client) configureInterface(name string, pid int, addr *net.IPNet, gatewayIP string) error {
	// Lock the OS Thread so we don't accidentally switch namespaces.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save the current network namespace.
	origns, err := netns.Get()
	if err != nil {
		return fmt.Errorf("getting current network namespace failed: %v", err)
	}
	defer origns.Close()

	// Get the namespace from the pid.
	newns, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("getting network namespace for pid %d failed: %v", pid, err)
	}
	defer newns.Close()

	// Enter the namespace.
	if err := netns.Set(newns); err != nil {
		return fmt.Errorf("entering network namespace failed: %v", err)
	}

	// Find the network interface identified by the name.
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("getting link %s failed: %v", name, err)
	}

	// Bring the interface down.
	if err := netlink.LinkSetDown(iface); err != nil {
		return fmt.Errorf("bringing interface [ %#v ] down failed: %v", iface, err)
	}

	// Change the interface name to eth0 in the namespace.
	if err := netlink.LinkSetName(iface, c.opt.ContainerInterface); err != nil {
		return fmt.Errorf("renaming interface %s to %s failed: %v", name, c.opt.ContainerInterface, err)
	}

	// Add the IP address.
	ipAddr := &netlink.Addr{IPNet: addr, Label: ""}
	if err := netlink.AddrAdd(iface, ipAddr); err != nil {
		return fmt.Errorf("setting %s interface ip to %s failed: %v", name, addr.String(), err)
	}

	// Bring the interface up.
	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("bringing interface [ %#v ] up failed: %v", iface, err)
	}

	// Add the gateway route.
	gw := net.ParseIP(gatewayIP)
	err = netlink.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: iface.Attrs().Index,
		Gw:        gw,
	})
	if err != nil {
		return fmt.Errorf("adding route %s to interface %s failed: %v", gw.String(), name, err)
	}

	// Switch back to the original namespace.
	if err := netns.Set(origns); err != nil {
		return fmt.Errorf("switching back to original namespace failed: %v", err)
	}

	return nil
}

// vethPair creates a veth pair. Peername is renamed to eth0 in the container.
func (c *Client) vethPair(pid int, bridgeName string) (*netlink.Veth, error) {
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("getting link %s failed: %v", bridgeName, err)
	}

	la := netlink.NewLinkAttrs()
	la.Name = fmt.Sprintf("%s-%d", c.opt.PortPrefix, pid)
	la.MasterIndex = br.Attrs().Index

	return &netlink.Veth{
		LinkAttrs: la,
		PeerName:  fmt.Sprintf("ethc%d", pid),
	}, nil
}
