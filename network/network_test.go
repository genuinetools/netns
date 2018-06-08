package network

import "testing"

const (
	defaultBridgeName = "netns0"
	defaultBridgeIP   = "172.19.0.1/16"
	defaultStateDir   = "/run/github.com/genuinetools/netns"
)

func TestNewNetworkBridgeNameEmpty(t *testing.T) {
	_, err := New(Opt{})
	if err == nil {
		t.Fatal("expected an error")
	}

	if err != ErrBridgeNameEmpty {
		t.Fatalf("expected %v got %v", ErrBridgeNameEmpty, err)
	}
}

func TestNewNetworkStateDirPathEmpty(t *testing.T) {
	_, err := New(Opt{
		BridgeName: defaultBridgeName,
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	if err != ErrStateDirPathEmpty {
		t.Fatalf("expected %v got %v", ErrStateDirPathEmpty, err)
	}
}

func TestNewNetworkDefaults(t *testing.T) {
	c, err := New(Opt{
		BridgeName: defaultBridgeName,
		StateDir:   defaultStateDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if c.opt.ContainerInterface != DefaultContainerInterface {
		t.Fatalf("expected container interface to be %s got %s", DefaultContainerInterface, c.opt.ContainerInterface)
	}

	if c.opt.PortPrefix != DefaultPortPrefix {
		t.Fatalf("expected port prefix to be %s got %s", DefaultPortPrefix, c.opt.PortPrefix)
	}
}
