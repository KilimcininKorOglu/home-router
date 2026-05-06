// Package services provides the IPv6Service that orchestrates the
// wide-dhcpv6 client (dhcp6c) for ISP prefix delegation.
//
// Lifecycle: render dhcp6c.conf and the lease-event hook script ->
// systemctl start lankeeper-dhcp6c.service -> dhcp6c writes the
// current prefix to a JSON state file -> Status() reads that file.
//
// The 3-layer config rendering pattern (RenderConfig / RenderToDisk
// / ApplyConfig) matches every other service in this package so
// install-time `render-configs` and runtime CRUD share one codepath.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/KilimcininKorOglu/lankeeper/internal/config"
	"github.com/KilimcininKorOglu/lankeeper/internal/netutil"
)

const (
	dhcp6cConfPath   = "/etc/wide-dhcpv6/dhcp6c.conf"
	dhcp6cScriptPath = "/etc/wide-dhcpv6/dhcp6c-script"
	ipv6StatePath    = "/var/lib/lankeeper/state/ipv6-prefix.json"
	dhcp6cUnitName   = "lankeeper-dhcp6c.service"
)

// PrefixState is the parsed view of the JSON document the dhcp6c hook
// script writes on every lease event. The zero value (Prefix == "")
// represents "no delegation yet" — distinct from an explicit RELEASE.
type PrefixState struct {
	Timestamp         int64  `json:"timestamp"`
	Reason            string `json:"reason"`
	Prefix            string `json:"prefix"`
	PrefixLength      int    `json:"prefixLength"`
	PreferredLifetime int64  `json:"preferredLifetime"`
	ValidLifetime     int64  `json:"validLifetime"`
	RDNSS             string `json:"rdnss"`
}

// Active reports whether a usable prefix is currently delegated.
func (p PrefixState) Active() bool {
	return p.Prefix != "" && p.ValidLifetime > 0 && p.Reason != "RELEASE" && p.Reason != "EXIT"
}

// CIDR returns "<prefix>/<length>" or "" if no prefix is held.
func (p PrefixState) CIDR() string {
	if p.Prefix == "" || p.PrefixLength == 0 {
		return ""
	}
	return fmt.Sprintf("%s/%d", p.Prefix, p.PrefixLength)
}

type IPv6Service struct {
	cfg *config.Config
	mu  sync.Mutex

	// Template overrides for tests; empty in production. When set,
	// RenderConfig / RenderScript skip filesystem ParseFiles and use
	// these strings directly. Mirrors the pattern used by FirewallService.
	confTmpl   string
	scriptTmpl string
}

func NewIPv6Service(cfg *config.Config) *IPv6Service {
	return &IPv6Service{cfg: cfg}
}

// NewIPv6ServiceFromFS creates a service that uses the given template
// strings instead of reading them off disk. Intended for unit tests.
func NewIPv6ServiceFromFS(cfg *config.Config, confTmpl, scriptTmpl string) *IPv6Service {
	return &IPv6Service{cfg: cfg, confTmpl: confTmpl, scriptTmpl: scriptTmpl}
}

type dhcp6cTemplateData struct {
	WANInterface string
	RapidCommit  bool
	// ScriptPath is the absolute path to the lease-event hook script
	// that dhcp6c invokes on every state change.
	ScriptPath string
	// SLALen is the suffix length given to each LAN sub-prefix:
	// `delegated_length + SLALen <= 64` for SLAAC compatibility.
	// For a /56 delegation we want /64 sub-prefixes -> SLALen = 8.
	SLALen int
	// PrefixInterfaces is one entry per downstream interface (LAN
	// bridge + each VLAN) that receives a sub-prefix.
	PrefixInterfaces []prefixInterface
}

type prefixInterface struct {
	Device string
	SLAID  int
}

type dhcp6cScriptTemplateData struct {
	StatePath string
}

// RenderConfig returns the rendered dhcp6c.conf as a string. Pure
// computation — no I/O.
func (s *IPv6Service) RenderConfig() (string, error) {
	wan, lan, err := s.resolveInterfaces()
	if err != nil {
		return "", err
	}

	hint := s.cfg.IPv6.WAN.PrefixHint
	if hint == "" {
		hint = "/56"
	}
	delegatedLen, err := parsePrefixHint(hint)
	if err != nil {
		return "", err
	}
	slaLen := 64 - delegatedLen
	if slaLen < 0 {
		slaLen = 0
	}

	data := dhcp6cTemplateData{
		WANInterface:     wan,
		RapidCommit:      s.cfg.IPv6.WAN.RapidCommit,
		ScriptPath:       dhcp6cScriptPath,
		SLALen:           slaLen,
		PrefixInterfaces: s.buildPrefixInterfaces(lan, slaLen),
	}

	tmpl, err := s.parseConfTemplate()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render dhcp6c.conf: %w", err)
	}
	return buf.String(), nil
}

// RenderScript returns the rendered dhcp6c lease-event hook script.
func (s *IPv6Service) RenderScript() (string, error) {
	tmpl, err := s.parseScriptTemplate()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, dhcp6cScriptTemplateData{StatePath: ipv6StatePath}); err != nil {
		return "", fmt.Errorf("render dhcp6c-script: %w", err)
	}
	return buf.String(), nil
}

func (s *IPv6Service) parseConfTemplate() (*template.Template, error) {
	if s.confTmpl != "" {
		t, err := template.New("dhcp6c.conf.tmpl").Parse(s.confTmpl)
		if err != nil {
			return nil, fmt.Errorf("parse inline dhcp6c template: %w", err)
		}
		return t, nil
	}
	t, err := template.New("dhcp6c.conf.tmpl").ParseFiles("configs/sysconf/dhcp6c.conf.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse dhcp6c template: %w", err)
	}
	return t, nil
}

func (s *IPv6Service) parseScriptTemplate() (*template.Template, error) {
	if s.scriptTmpl != "" {
		t, err := template.New("dhcp6c-script.tmpl").Parse(s.scriptTmpl)
		if err != nil {
			return nil, fmt.Errorf("parse inline dhcp6c-script template: %w", err)
		}
		return t, nil
	}
	t, err := template.New("dhcp6c-script.tmpl").ParseFiles("configs/sysconf/dhcp6c-script.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse dhcp6c-script template: %w", err)
	}
	return t, nil
}

// RenderToDisk writes both the daemon config and the hook script to
// /etc/wide-dhcpv6/. Suitable for install-time invocation. The state
// directory is created so the hook script does not fail on first run.
func (s *IPv6Service) RenderToDisk(ctx context.Context) error {
	if s.cfg.IPv6.Enabled == "off" || !s.cfg.IPv6.WAN.RequestPrefix {
		// Caller asked for IPv6 off or PD disabled — nothing to render
		// but make sure stale config does not linger.
		_ = netutil.WriteFile(dhcp6cConfPath, []byte("# IPv6 PD disabled by LANKeeper config.\n"), 0o644)
		return nil
	}

	conf, err := s.RenderConfig()
	if err != nil {
		return err
	}
	script, err := s.RenderScript()
	if err != nil {
		return err
	}
	if err := netutil.MkdirAll(filepath.Dir(dhcp6cConfPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dhcp6cConfPath), err)
	}
	if err := netutil.MkdirAll(filepath.Dir(ipv6StatePath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(ipv6StatePath), err)
	}
	if err := netutil.WriteFile(dhcp6cConfPath, []byte(conf), 0o644); err != nil {
		return fmt.Errorf("write dhcp6c.conf: %w", err)
	}
	if err := netutil.WriteFile(dhcp6cScriptPath, []byte(script), 0o755); err != nil {
		return fmt.Errorf("write dhcp6c-script: %w", err)
	}
	return nil
}

// ApplyConfig renders to disk and restarts the dhcp6c unit so the new
// settings take effect immediately.
func (s *IPv6Service) ApplyConfig(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.RenderToDisk(ctx); err != nil {
		return err
	}
	if s.cfg.IPv6.Enabled == "off" || !s.cfg.IPv6.WAN.RequestPrefix {
		return s.stopUnitLocked(ctx)
	}
	return s.restartUnitLocked(ctx)
}

// Start enables and starts the dhcp6c unit. Idempotent.
func (s *IPv6Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := netutil.Run(ctx, "systemctl", "enable", "--now", dhcp6cUnitName)
	if err != nil {
		return fmt.Errorf("start %s: %w", dhcp6cUnitName, err)
	}
	return nil
}

// Stop disables and stops the dhcp6c unit. Idempotent.
func (s *IPv6Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopUnitLocked(ctx)
}

// Restart bounces the dhcp6c unit. Used after WAN reconnect.
func (s *IPv6Service) Restart(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restartUnitLocked(ctx)
}

// Renew triggers an immediate Renew message via SIGHUP. dhcp6c reloads
// the config and re-solicits the ISP without dropping the lease.
func (s *IPv6Service) Renew(ctx context.Context) error {
	_, err := netutil.Run(ctx, "systemctl", "reload-or-restart", dhcp6cUnitName)
	if err != nil {
		return fmt.Errorf("renew %s: %w", dhcp6cUnitName, err)
	}
	return nil
}

// Release sends a DHCPv6 RELEASE and stops the unit. Used when the
// operator wants to drop the prefix explicitly.
func (s *IPv6Service) Release(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Best effort: dhcp6c does not have a direct release CLI; stopping
	// the unit triggers the EXIT reason in the hook script which clears
	// the state file. Operators can re-enable via Start.
	return s.stopUnitLocked(ctx)
}

// Status reads the JSON state file written by the dhcp6c hook script.
// Returns a zero PrefixState (Active() == false) when no lease has
// been recorded yet.
func (s *IPv6Service) Status(ctx context.Context) (PrefixState, error) {
	raw, err := netutil.ReadFile(ipv6StatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return PrefixState{}, nil
		}
		return PrefixState{}, fmt.Errorf("read %s: %w", ipv6StatePath, err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return PrefixState{}, nil
	}
	var st PrefixState
	if err := json.Unmarshal(raw, &st); err != nil {
		return PrefixState{}, fmt.Errorf("parse ipv6 state: %w", err)
	}
	return st, nil
}

// PrefixAge returns how long ago the lease event was recorded. Useful
// for "last updated 5m ago" display.
func (p PrefixState) PrefixAge() time.Duration {
	if p.Timestamp == 0 {
		return 0
	}
	return time.Since(time.Unix(p.Timestamp, 0))
}

// buildPrefixInterfaces returns one entry per downstream interface
// (LAN bridge + each VLAN) that should receive a sub-prefix. SLA-IDs
// are taken from cfg.IPv6.LAN.SubnetMap when present, otherwise
// auto-assigned in declaration order starting at 0 for the LAN.
//
// When slaLen == 0 the ISP delegated a /64 — no room to subdivide, so
// only the primary LAN gets a prefix-interface entry.
func (s *IPv6Service) buildPrefixInterfaces(lanDev string, slaLen int) []prefixInterface {
	subnetMap := s.cfg.IPv6.LAN.SubnetMap

	// Helper: pick from override map, fall back to caller-supplied default.
	pick := func(key string, dflt int) int {
		if v, ok := subnetMap[key]; ok {
			return v
		}
		return dflt
	}

	out := []prefixInterface{
		{Device: lanDev, SLAID: pick("lan", 0)},
	}

	if slaLen == 0 {
		// /64 delegation has zero subnet bits — VLANs cannot get
		// distinct sub-prefixes from this delegation.
		return out
	}

	maxSLA := 1<<slaLen - 1
	auto := 1
	for _, vlan := range s.cfg.VLANs {
		if vlan.ID == "" {
			continue
		}
		var parentDev string
		for _, iface := range s.cfg.Interfaces {
			if iface.ID == vlan.Parent {
				parentDev = iface.Device
				break
			}
		}
		if parentDev == "" || vlan.VID == 0 {
			continue
		}
		dev := fmt.Sprintf("%s.%d", parentDev, vlan.VID)
		sla := pick(vlan.ID, auto)
		if sla > maxSLA {
			// Skip silently; operator will see fewer interfaces in
			// the rendered config than expected. The Web UI surface
			// can flag this in a follow-up commit.
			continue
		}
		out = append(out, prefixInterface{Device: dev, SLAID: sla})
		auto++
	}
	return out
}

// resolveInterfaces returns the WAN and LAN device names from the
// router configuration. WAN is the PPPoE interface name (ppp0 by
// convention) when PPPoE is the WAN method; LAN is the first
// interface tagged role=lan.
func (s *IPv6Service) resolveInterfaces() (wan, lan string, err error) {
	for _, iface := range s.cfg.Interfaces {
		switch iface.Role {
		case "wan":
			// dhcp6c needs the L3 interface that actually carries the
			// IPv6 link to the ISP. With PPPoE this is ppp0, not the
			// underlying physical NIC.
			if s.cfg.PPPoE.Username != "" {
				wan = "ppp0"
			} else {
				wan = iface.Device
			}
		case "lan":
			if lan == "" {
				lan = iface.Device
			}
		}
	}
	if wan == "" {
		return "", "", fmt.Errorf("no WAN interface configured")
	}
	if lan == "" {
		return "", "", fmt.Errorf("no LAN interface configured")
	}
	return wan, lan, nil
}

// parsePrefixHint converts "/56" into 56. Accepts the bare number or
// the slash-prefixed form. Rejects values outside [48, 64].
func parsePrefixHint(s string) (int, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(s), "/")
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid prefix hint %q: %w", s, err)
	}
	if n < 48 || n > 64 {
		return 0, fmt.Errorf("prefix hint %d outside [48,64]", n)
	}
	return n, nil
}

func (s *IPv6Service) restartUnitLocked(ctx context.Context) error {
	_, err := netutil.Run(ctx, "systemctl", "restart", dhcp6cUnitName)
	if err != nil {
		return fmt.Errorf("restart %s: %w", dhcp6cUnitName, err)
	}
	return nil
}

func (s *IPv6Service) stopUnitLocked(ctx context.Context) error {
	_, err := netutil.Run(ctx, "systemctl", "stop", dhcp6cUnitName)
	if err != nil {
		// systemctl exit 5 = unit not loaded; tolerate.
		if strings.Contains(err.Error(), "exit status 5") {
			return nil
		}
		return fmt.Errorf("stop %s: %w", dhcp6cUnitName, err)
	}
	return nil
}
