package bridge

import "testing"

const (
	defaultBridgeIP   = "172.19.0.1/16"
	defaultBridgeName = "testing0"
)

func TestInitBridgeIPAddrEmpty(t *testing.T) {
	_, err := Init(Opt{
		Name: defaultBridgeName,
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	if err != ErrIPAddrEmpty {
		t.Fatalf("expected %v got %v", ErrIPAddrEmpty, err)
	}
}

func TestInitBridgeNameEmpty(t *testing.T) {
	_, err := Init(Opt{
		IPAddr: defaultBridgeIP,
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	if err != ErrNameEmpty {
		t.Fatalf("expected %v got %v", ErrNameEmpty, err)
	}
}

func TestInitBridgeDefaults(t *testing.T) {
	defer Delete(defaultBridgeName)
	br, err := Init(Opt{
		IPAddr: defaultBridgeIP,
		Name:   defaultBridgeName,
	})
	if err != nil {
		t.Fatal(err)
	}

	if br.MTU != DefaultMTU {
		t.Fatalf("expected bridge MTU to be %d got %d", DefaultMTU, br.MTU)
	}
}

func TestInitBridgeExists(t *testing.T) {
	defer Delete(defaultBridgeName)
	br, err := Init(Opt{
		IPAddr: defaultBridgeIP,
		Name:   defaultBridgeName,
	})
	if err != nil {
		t.Fatal(err)
	}

	if br.Name != defaultBridgeName {
		t.Fatalf("expected bridge name to be %s got %s", defaultBridgeName, br.Name)
	}

	// Initialize the bridge again.
	br, err = Init(Opt{
		IPAddr: defaultBridgeIP,
		Name:   defaultBridgeName,
	})
	if err != nil {
		t.Fatal(err)
	}

	if br.Name != defaultBridgeName {
		t.Fatalf("expected bridge name to be %s got %s", defaultBridgeName, br.Name)
	}
}
