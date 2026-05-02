package services_test

import (
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/services"
)

func TestNewNetworkService(t *testing.T) {
	cfg := &config.Config{}
	cfg.Interfaces = []config.InterfaceConfig{
		{ID: "wan", Device: "enp3s0", Label: "WAN", Role: "wan"},
		{ID: "lan", Device: "enp0s25", Label: "LAN", Role: "lan"},
	}

	svc := services.NewNetworkService(cfg)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestDetectInterfaces(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewNetworkService(cfg)

	ifaces, err := svc.DetectInterfaces()
	if err != nil {
		t.Fatalf("detect interfaces: %v", err)
	}

	for _, iface := range ifaces {
		if iface.IsVirtual {
			t.Errorf("DetectInterfaces should filter out virtual interfaces, got %s", iface.Name)
		}
	}
}

func TestGetInterfaceStatusNotFound(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewNetworkService(cfg)

	_, err := svc.GetInterfaceStatus("nonexistent999")
	if err == nil {
		t.Error("should fail for nonexistent interface")
	}
}

func TestGetInterfaceStatusWithConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Interfaces = []config.InterfaceConfig{
		{ID: "lo", Device: "lo", Label: "Loopback", Role: "unused", MTU: 65536},
	}

	svc := services.NewNetworkService(cfg)
	status, err := svc.GetInterfaceStatus("lo")
	if err != nil {
		t.Skipf("loopback not available: %v", err)
	}

	if status.Label != "Loopback" {
		t.Errorf("label = %q, want Loopback", status.Label)
	}
	if status.Role != "unused" {
		t.Errorf("role = %q, want unused", status.Role)
	}
}
