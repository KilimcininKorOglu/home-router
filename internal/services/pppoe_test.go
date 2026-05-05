package services_test

import (
	"context"
	"testing"

	"github.com/KilimcininKorOglu/lankeeper/internal/config"
	"github.com/KilimcininKorOglu/lankeeper/internal/services"
)

func TestNewPPPoEService(t *testing.T) {
	cfg := &config.Config{}
	cfg.PPPoE.MTU = 1492
	cfg.PPPoE.MRU = 1492
	cfg.PPPoE.Username = "test@isp"

	svc := services.NewPPPoEService(cfg)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestPPPoEStatusDisconnected(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewPPPoEService(cfg)

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.Connected {
		t.Error("should not be connected when no pppd is running")
	}
}

func TestPPPoEIsConnectedDefault(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewPPPoEService(cfg)

	if svc.IsConnected() {
		t.Error("should not be connected by default")
	}
}
