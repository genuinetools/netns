package ping

import (
	"net"
	"testing"
	"time"
)

func TestPingIPv4(t *testing.T) {
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			t.Fatal(err)
		}

		for _, addr := range addrs {
			if addr.(*net.IPNet).IP.To4() == nil { // ipv6, skip
				continue
			}

			t.Logf("pinging %q on interface %q", addr.(*net.IPNet).IP.String(), iface.Name)

			// these are all local interfaces; should not fail.
			if err := Pinger(&net.IPAddr{IP: addr.(*net.IPNet).IP, Zone: iface.Name}, 150*time.Millisecond); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPingIPv6(t *testing.T) {
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			t.Fatal(err)
		}

		for _, addr := range addrs {
			if addr.(*net.IPNet).IP.To4() != nil { // ipv4, skip
				continue
			}

			t.Logf("pinging %q on interface %q", addr.(*net.IPNet).IP.String(), iface.Name)

			// these are all local interfaces; should not fail.
			if err := Pinger(&net.IPAddr{IP: addr.(*net.IPNet).IP, Zone: iface.Name}, 150*time.Millisecond); err != nil {
				t.Fatal(err)
			}
		}
	}
}
