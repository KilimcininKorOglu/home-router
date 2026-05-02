package services

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

type RoutingService struct {
	cfg *config.Config
	mu  sync.RWMutex
}

func NewRoutingService(cfg *config.Config) *RoutingService {
	return &RoutingService{cfg: cfg}
}

func (s *RoutingService) GetPolicies() []config.RoutingPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policies := make([]config.RoutingPolicy, len(s.cfg.Routing.Policies))
	copy(policies, s.cfg.Routing.Policies)

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority < policies[j].Priority
	})

	return policies
}

func (s *RoutingService) AddPolicy(policy config.RoutingPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if policy.Priority == 0 {
		maxPrio := 0
		for _, p := range s.cfg.Routing.Policies {
			if p.Priority > maxPrio {
				maxPrio = p.Priority
			}
		}
		policy.Priority = maxPrio + 10
	}

	s.cfg.Routing.Policies = append(s.cfg.Routing.Policies, policy)
}

func (s *RoutingService) RemovePolicy(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.cfg.Routing.Policies {
		if p.Name == name {
			s.cfg.Routing.Policies = append(s.cfg.Routing.Policies[:i], s.cfg.Routing.Policies[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("policy %q not found", name)
}

func (s *RoutingService) UpdatePriorities(orderedNames []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policyMap := make(map[string]*config.RoutingPolicy, len(s.cfg.Routing.Policies))
	for i := range s.cfg.Routing.Policies {
		policyMap[s.cfg.Routing.Policies[i].Name] = &s.cfg.Routing.Policies[i]
	}

	for i, name := range orderedNames {
		if p, ok := policyMap[name]; ok {
			p.Priority = (i + 1) * 10
		}
	}
}

func (s *RoutingService) TogglePolicy(name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.cfg.Routing.Policies {
		if s.cfg.Routing.Policies[i].Name == name {
			s.cfg.Routing.Policies[i].Enabled = enabled
			return nil
		}
	}
	return fmt.Errorf("policy %q not found", name)
}

func (s *RoutingService) Apply(ctx context.Context) error {
	s.mu.RLock()
	policies := make([]config.RoutingPolicy, len(s.cfg.Routing.Policies))
	copy(policies, s.cfg.Routing.Policies)
	s.mu.RUnlock()

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority < policies[j].Priority
	})

	for _, p := range policies {
		if !p.Enabled {
			continue
		}
		if err := s.applyPolicy(ctx, p); err != nil {
			log.Printf("apply policy %q: %v", p.Name, err)
		}
	}

	log.Printf("PBR applied: %d policies", len(policies))
	return nil
}

func (s *RoutingService) Clear(ctx context.Context) error {
	for _, p := range s.cfg.Routing.Policies {
		s.clearPolicy(ctx, p)
	}
	return nil
}

func (s *RoutingService) applyPolicy(ctx context.Context, p config.RoutingPolicy) error {
	tunnel := s.findTunnel(p.Tunnel)
	if tunnel == nil {
		return fmt.Errorf("tunnel %q not found for policy %q", p.Tunnel, p.Name)
	}

	for _, mac := range p.SrcMACs {
		rule := fmt.Sprintf("ether saddr %s meta mark set %d", mac, tunnel.Fwmark)
		log.Printf("PBR: %s → %s (%s)", mac, p.Tunnel, rule)
	}

	for _, ip := range p.SrcIPs {
		rule := fmt.Sprintf("ip saddr %s meta mark set %d", ip, tunnel.Fwmark)
		log.Printf("PBR: %s → %s (%s)", ip, p.Tunnel, rule)
	}

	_, err := netutil.Run(ctx, "ip", "rule", "add", "fwmark",
		fmt.Sprintf("%d", tunnel.Fwmark), "lookup", fmt.Sprintf("%d", tunnel.Table),
		"priority", fmt.Sprintf("%d", p.Priority))
	if err != nil {
		log.Printf("ip rule add for policy %q: %v", p.Name, err)
	}

	return nil
}

func (s *RoutingService) clearPolicy(ctx context.Context, p config.RoutingPolicy) {
	tunnel := s.findTunnel(p.Tunnel)
	if tunnel == nil {
		return
	}

	netutil.Run(ctx, "ip", "rule", "del", "fwmark",
		fmt.Sprintf("%d", tunnel.Fwmark), "lookup", fmt.Sprintf("%d", tunnel.Table))
}

type tunnelRef struct {
	Table  int
	Fwmark int
}

func (s *RoutingService) findTunnel(name string) *tunnelRef {
	for _, t := range s.cfg.VPN.Clients {
		if t.Name == name {
			return &tunnelRef{Table: t.Table, Fwmark: t.Fwmark}
		}
	}
	for _, t := range s.cfg.OpenVPN.Clients {
		if t.Name == name {
			return &tunnelRef{Table: t.Table, Fwmark: t.Fwmark}
		}
	}
	return nil
}

func (s *RoutingService) GenerateNftRules() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	for _, p := range s.cfg.Routing.Policies {
		if !p.Enabled {
			continue
		}

		tunnel := s.findTunnel(p.Tunnel)
		if tunnel == nil {
			continue
		}

		for _, mac := range p.SrcMACs {
			fmt.Fprintf(&sb, "        ether saddr %s meta mark set %d\n", mac, tunnel.Fwmark)
		}
		for _, ip := range p.SrcIPs {
			fmt.Fprintf(&sb, "        ip saddr %s meta mark set %d\n", ip, tunnel.Fwmark)
		}
		for _, dst := range p.DstIPs {
			fmt.Fprintf(&sb, "        ip daddr %s meta mark set %d\n", dst, tunnel.Fwmark)
		}
		for _, port := range p.DstPorts {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			fmt.Fprintf(&sb, "        %s dport %d meta mark set %d\n", proto, port, tunnel.Fwmark)
		}
	}

	return sb.String()
}
