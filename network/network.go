package network

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	bolt "go.etcd.io/bbolt"
)

const (
	// dbFile is the file the bolt database is stored in.
	dbFile = "bolt.db"

	// DefaultContainerInterface is the default container interface.
	DefaultContainerInterface = "eth0"
	// DefaultPortPrefix is the default port prefix.
	DefaultPortPrefix = "netnsv0"
)

var (
	// ipBucket is the bolt database bucket for ip key value store.
	ipBucket = []byte("ipallocator")

	// ErrBridgeNameEmpty holds the error for when the bridge name is empty.
	ErrBridgeNameEmpty = errors.New("bridge name cannot be empty")
	// ErrStateDirPathEmpty holds the error for when the state directory path
	// is empty.
	ErrStateDirPathEmpty = errors.New("state directory path cannot be empty")
	// ErrDatabaseDoesNotExist holds the error for when the database does not
	// exit.
	ErrDatabaseDoesNotExist = errors.New("database does not exist")
)

// Opt holds the options for holding networks state, etc.
type Opt struct {
	StateDir           string
	ContainerInterface string
	PortPrefix         string
	BridgeName         string
}

// Network holds information about a network.
type Network struct {
	VethPair *netlink.Veth
	IP       net.IP
	PID      int
	Status   string
	FD       netns.NsHandle
}

// Client is the object used for interacting with networks.
type Client struct {
	dbPath string
	db     *bolt.DB
	opt    Opt

	bridge *net.Interface
	ipNet  *net.IPNet
}

// New creates a new Client for interacting with networks.
func New(opt Opt) (*Client, error) {
	// Validate the options.
	if len(opt.BridgeName) < 1 {
		return nil, ErrBridgeNameEmpty
	}
	if len(opt.StateDir) < 1 {
		return nil, ErrStateDirPathEmpty
	}

	// Set the defaults.
	if len(opt.ContainerInterface) < 1 {
		opt.ContainerInterface = DefaultContainerInterface
	}
	if len(opt.PortPrefix) < 1 {
		opt.PortPrefix = DefaultPortPrefix
	}

	// Create the state directory in case it does not exist.
	if err := os.MkdirAll(opt.StateDir, 0666); err != nil {
		return nil, fmt.Errorf("creating state directory %s failed: %v", opt.StateDir, err)
	}

	return &Client{
		dbPath: filepath.Join(opt.StateDir, dbFile),
		opt:    opt,
	}, nil
}

func (c *Client) openDB(readonly bool) (err error) {
	if c.db != nil {
		// The database is already opened.
		return nil
	}

	// This will block until other operations on it are closed which is fine
	// for our use case of assigning one IP and being done.
	c.db, err = bolt.Open(c.dbPath, 0666, &bolt.Options{
		ReadOnly: readonly,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return ErrDatabaseDoesNotExist
		}

		return fmt.Errorf("opening database at %s failed: %v", c.dbPath, err)
	}

	return nil
}

func (c *Client) closeDB() error {
	err := c.db.Close()
	c.db = nil
	return err
}
