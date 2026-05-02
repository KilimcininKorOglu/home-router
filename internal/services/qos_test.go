package services_test

import (
	"context"
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/services"
)

func TestNewQoSService(t *testing.T) {
	cfg := &config.Config{}
	cfg.QoS.Enabled = true
	cfg.QoS.Profile = "cake"
	cfg.QoS.UploadKbps = 40000
	cfg.QoS.DownloadKbps = 950000
	cfg.QoS.CongestionControl = "bbr"

	svc := services.NewQoSService(cfg)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestQoSStatusDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.QoS.Enabled = false
	cfg.QoS.Profile = "none"
	cfg.Interfaces = []config.InterfaceConfig{
		{ID: "wan", Device: "ppp0", Role: "wan", Type: "pppoe"},
	}

	svc := services.NewQoSService(cfg)
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.Enabled {
		t.Error("should not be enabled")
	}
	if status.EgressActive {
		t.Error("egress should not be active when no tc rules")
	}
}
