package services_test

import (
	"context"
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/services"
)

func TestNewOpenVPNService(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewOpenVPNService(cfg)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestOpenVPNServerStatus(t *testing.T) {
	cfg := &config.Config{}
	cfg.OpenVPN.Server.Enabled = false
	cfg.OpenVPN.Server.Port = 1194
	cfg.OpenVPN.Server.Protocol = "udp"

	svc := services.NewOpenVPNService(cfg)
	status, err := svc.ServerStatus(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.Enabled {
		t.Error("should not be enabled")
	}
	if status.Active {
		t.Error("should not be active when openvpn is not running")
	}
}

func TestOpenVPNImportClient(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewOpenVPNService(cfg)

	svc.ImportClientConfig("work-vpn", "client\ndev tun\nproto udp\n")

	if len(cfg.OpenVPN.Clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(cfg.OpenVPN.Clients))
	}
	if cfg.OpenVPN.Clients[0].Name != "work-vpn" {
		t.Errorf("name = %q, want work-vpn", cfg.OpenVPN.Clients[0].Name)
	}
}
