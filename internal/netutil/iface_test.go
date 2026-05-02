package netutil_test

import (
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

func TestDetectInterfaces(t *testing.T) {
	ifaces, err := netutil.DetectInterfaces()
	if err != nil {
		t.Fatalf("detect interfaces: %v", err)
	}

	if len(ifaces) == 0 {
		t.Skip("no non-loopback interfaces found")
	}

	for _, iface := range ifaces {
		if iface.Name == "" {
			t.Error("interface name should not be empty")
		}
		if iface.State != "up" && iface.State != "down" {
			t.Errorf("interface %s state = %q, want up or down", iface.Name, iface.State)
		}
	}
}

func TestGetInterfaceState(t *testing.T) {
	_, err := netutil.GetInterfaceState("lo")
	if err != nil {
		t.Skipf("loopback not available: %v", err)
	}
}

func TestGetInterfaceStateNotFound(t *testing.T) {
	_, err := netutil.GetInterfaceState("nonexistent999")
	if err == nil {
		t.Error("should fail for nonexistent interface")
	}
}

func TestReadInterfaceStats(t *testing.T) {
	ifaces, _ := netutil.DetectInterfaces()
	if len(ifaces) == 0 {
		t.Skip("no interfaces")
	}

	rx, tx, err := netutil.ReadInterfaceStats(ifaces[0].Name)
	if err != nil {
		t.Skipf("cannot read stats for %s: %v", ifaces[0].Name, err)
	}

	_ = rx
	_ = tx
}
