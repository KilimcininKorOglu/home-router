package services_test

import (
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/services"
)

func TestNewRoutingService(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewRoutingService(cfg)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestRoutingAddRemovePolicy(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewRoutingService(cfg)

	svc.AddPolicy(config.RoutingPolicy{
		Name:    "xbox-vpn",
		Enabled: true,
		SrcMACs: []string{"aa:bb:cc:dd:ee:ff"},
		Tunnel:  "nl-amsterdam",
	})

	policies := svc.GetPolicies()
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	if policies[0].Priority == 0 {
		t.Error("auto-priority should be non-zero")
	}

	if err := svc.RemovePolicy("xbox-vpn"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(svc.GetPolicies()) != 0 {
		t.Error("should be empty after removal")
	}
}

func TestRoutingTogglePolicy(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewRoutingService(cfg)

	svc.AddPolicy(config.RoutingPolicy{Name: "test", Enabled: true})

	if err := svc.TogglePolicy("test", false); err != nil {
		t.Fatalf("toggle: %v", err)
	}

	policies := svc.GetPolicies()
	if policies[0].Enabled {
		t.Error("should be disabled after toggle")
	}
}

func TestRoutingUpdatePriorities(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewRoutingService(cfg)

	svc.AddPolicy(config.RoutingPolicy{Name: "a", Enabled: true})
	svc.AddPolicy(config.RoutingPolicy{Name: "b", Enabled: true})
	svc.AddPolicy(config.RoutingPolicy{Name: "c", Enabled: true})

	svc.UpdatePriorities([]string{"c", "a", "b"})

	policies := svc.GetPolicies()
	if policies[0].Name != "c" {
		t.Errorf("first policy should be 'c', got %q", policies[0].Name)
	}
}

func TestRoutingGenerateNftRules(t *testing.T) {
	cfg := &config.Config{}
	cfg.VPN.Clients = []config.WGClientTunnel{
		{Name: "nl-amsterdam", Table: 100, Fwmark: 100},
	}

	svc := services.NewRoutingService(cfg)
	svc.AddPolicy(config.RoutingPolicy{
		Name:    "xbox",
		Enabled: true,
		SrcMACs: []string{"aa:bb:cc:dd:ee:ff"},
		Tunnel:  "nl-amsterdam",
	})

	rules := svc.GenerateNftRules()
	if !strings.Contains(rules, "ether saddr aa:bb:cc:dd:ee:ff meta mark set 100") {
		t.Errorf("expected fwmark rule, got:\n%s", rules)
	}
}

func TestRoutingRemovePolicyNotFound(t *testing.T) {
	cfg := &config.Config{}
	svc := services.NewRoutingService(cfg)

	if err := svc.RemovePolicy("nonexistent"); err == nil {
		t.Error("should error for nonexistent policy")
	}
}
