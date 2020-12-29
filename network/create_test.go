package network

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/genuinetools/netns/bridge"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestCreateNetwork(t *testing.T) {
	process, err := createTestProcess()
	if err != nil {
		t.Fatal(err)
	}
	defer process.Kill()

	c, err := New(Opt{
		BridgeName: defaultBridgeName,
		StateDir:   defaultStateDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(defaultStateDir)

	ip, err := c.Create(&specs.State{
		Pid: process.Pid,
	}, bridge.Opt{
		IPAddr: defaultBridgeIP,
		Name:   defaultBridgeName,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Delete(defaultBridgeName)

	expected := "172.19.0.2"
	if ip.String() != expected {
		t.Fatalf("expected IP to be %s got %s", expected, ip.String())
	}

	if err := c.openDB(false); err != nil {
		t.Fatal(err)
	}

	process2, err := createTestProcess()
	if err != nil {
		t.Fatal(err)
	}
	defer process2.Kill()

	// Allocate another IP.
	ip, err = c.AllocateIP(process2.Pid)
	if err != nil {
		t.Fatal(err)
	}
	expected = "172.19.0.3"
	if ip.String() != expected {
		t.Fatalf("expected IP to be %s got %s", expected, ip.String())
	}
}

func createTestProcess() (*os.Process, error) {
	// Create a process in a new network namespace.
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Unshareflags: syscall.CLONE_NEWNET}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unsharing command failed: %v", err)
	}

	return cmd.Process, nil
}
