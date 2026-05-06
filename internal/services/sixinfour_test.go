package services_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/lankeeper/internal/config"
	"github.com/KilimcininKorOglu/lankeeper/internal/netutil"
	"github.com/KilimcininKorOglu/lankeeper/internal/services"
)

// newSixInFourTestConfig assembles the minimum viable Tunnel config
// plus a single Role:wan interface. PPPoE is OFF by default so the
// effective MTU stays at 1480 unless the caller flips Username on.
func newSixInFourTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.SetFilePath(filepath.Join(t.TempDir(), "router.yaml"))
	cfg.Interfaces = []config.InterfaceConfig{
		{ID: "wan", Device: "eth0", Role: "wan"},
		{ID: "lan", Device: "eth1", Role: "lan"},
	}
	cfg.IPv6.Mode = "6in4"
	cfg.IPv6.Tunnel = config.IPv6TunnelConfig{
		Provider:     "he.net",
		ServerIPv4:   "216.66.80.30",
		ClientIPv6:   "2001:470:1f0a:abc::2",
		RoutedPrefix: "2001:470:abcd::/48",
		TunnelID:     "1234567",
		Username:     "operator",
		UpdateKey:    "TESTKEY",
		AutoUpdate:   true,
		Device:       "lkt6in4",
	}
	cfg.PPPoE.Username = "" // direct WAN scenario
	return cfg
}

// fakeAgent is reused from ipv6_firewall_integration_test.go. We
// only need exec.run / file.write capture here, plus a passthrough
// for file.read so Status() reads the JSON we just persisted.

func TestSixInFourStartIssuesCorrectIPCommands(t *testing.T) {
	agent := &fakeAgent{}
	netutil.SetAgentClient(agent)
	t.Cleanup(func() { netutil.SetAgentClient(nil) })

	cfg := newSixInFourTestConfig(t)
	svc := services.NewSixInFourService(cfg)
	svc.SetStatePathForTest(filepath.Join(t.TempDir(), "ipv6-tunnel.json"))
	svc.SetLocalIPv4ForTest("203.0.113.5")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Expected exec sequence: del (cleanup) → tunnel add → link set →
	// addr add → route add. Cleanup is best-effort so we don't assert
	// its return code, only that it ran first.
	wantPrefixes := []string{
		"tunnel del lkt6in4",
		"tunnel add lkt6in4 mode sit remote 216.66.80.30 local",
		"link set lkt6in4 up mtu 1480",
		"addr add 2001:470:1f0a:abc::2 dev lkt6in4",
		"-6 route add ::/0 dev lkt6in4",
	}
	matched := 0
	for _, c := range agent.execLog {
		if c.Cmd != "ip" {
			continue
		}
		flat := strings.Join(c.Args, " ")
		if matched < len(wantPrefixes) && strings.HasPrefix(flat, wantPrefixes[matched]) {
			matched++
		}
	}
	if matched != len(wantPrefixes) {
		t.Errorf("ip command sequence mismatch; matched %d of %d, exec log: %+v",
			matched, len(wantPrefixes), agent.execLog)
	}

	// State JSON must have been written via file.write.
	if !agent.wroteFile("ipv6-tunnel.json") {
		t.Errorf("expected state JSON write, got: %+v", agent.writeLog)
	}
}

func TestSixInFourStopReversesIPCommandsLIFO(t *testing.T) {
	agent := &fakeAgent{}
	netutil.SetAgentClient(agent)
	t.Cleanup(func() { netutil.SetAgentClient(nil) })

	cfg := newSixInFourTestConfig(t)
	svc := services.NewSixInFourService(cfg)
	svc.SetStatePathForTest(filepath.Join(t.TempDir(), "ipv6-tunnel.json"))

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	want := []string{
		"-6 route del ::/0 dev lkt6in4",
		"link set lkt6in4 down",
		"tunnel del lkt6in4",
	}
	got := []string{}
	for _, c := range agent.execLog {
		if c.Cmd == "ip" {
			got = append(got, strings.Join(c.Args, " "))
		}
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d ip commands, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("Stop step %d: want prefix %q, got %q", i, want[i], got[i])
		}
	}
}

func TestSixInFourMTUClampsToPPPoE(t *testing.T) {
	agent := &fakeAgent{}
	netutil.SetAgentClient(agent)
	t.Cleanup(func() { netutil.SetAgentClient(nil) })

	cfg := newSixInFourTestConfig(t)
	cfg.PPPoE.Username = "user@isp" // PPPoE active → MTU 1452
	svc := services.NewSixInFourService(cfg)
	svc.SetStatePathForTest(filepath.Join(t.TempDir(), "ipv6-tunnel.json"))
	svc.SetLocalIPv4ForTest("100.64.1.42")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	sawMTU1452 := false
	for _, c := range agent.execLog {
		if c.Cmd != "ip" {
			continue
		}
		if strings.Contains(strings.Join(c.Args, " "), "mtu 1452") {
			sawMTU1452 = true
			break
		}
	}
	if !sawMTU1452 {
		t.Errorf("expected mtu 1452 under PPPoE, got: %+v", agent.execLog)
	}
}

func TestSixInFourStartFailsWhenConfigIncomplete(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*config.IPv6TunnelConfig)
	}{
		{"missing ServerIPv4", func(t *config.IPv6TunnelConfig) { t.ServerIPv4 = "" }},
		{"missing ClientIPv6", func(t *config.IPv6TunnelConfig) { t.ClientIPv6 = "" }},
		{"missing RoutedPrefix", func(t *config.IPv6TunnelConfig) { t.RoutedPrefix = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newSixInFourTestConfig(t)
			tc.mut(&cfg.IPv6.Tunnel)
			svc := services.NewSixInFourService(cfg)
			err := svc.Start(context.Background())
			if err == nil {
				t.Fatalf("Start with %s should fail", tc.name)
			}
			if !strings.Contains(err.Error(), "6in4:") {
				t.Errorf("expected 6in4: prefix, got %v", err)
			}
		})
	}
}

func TestSixInFourStatusReadsPersistedState(t *testing.T) {
	agent := &fakeAgent{}
	netutil.SetAgentClient(agent)
	t.Cleanup(func() { netutil.SetAgentClient(nil) })

	cfg := newSixInFourTestConfig(t)
	svc := services.NewSixInFourService(cfg)
	statePath := filepath.Join(t.TempDir(), "ipv6-tunnel.json")
	svc.SetStatePathForTest(statePath)
	svc.SetLocalIPv4ForTest("203.0.113.5")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Active {
		t.Errorf("Status.Active should be true, got %+v", st)
	}
	if st.MTU != 1480 {
		t.Errorf("Status.MTU = %d, want 1480", st.MTU)
	}
	if st.RoutedPrefix != "2001:470:abcd::/48" {
		t.Errorf("Status.RoutedPrefix = %q", st.RoutedPrefix)
	}

	// Sanity: the JSON we expose at the API matches the on-disk shape.
	raw, _ := json.Marshal(st)
	if !strings.Contains(string(raw), `"device":"lkt6in4"`) {
		t.Errorf("status JSON missing device field: %s", raw)
	}
}
