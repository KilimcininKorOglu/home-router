package services_test

import (
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/lankeeper/internal/config"
	"github.com/KilimcininKorOglu/lankeeper/internal/services"
)

const testDhcp6cConfTmpl = `interface {{ .WANInterface }} {
    send ia-pd 0;
{{- if .RapidCommit }}
    send rapid-commit;
{{- end }}
    request domain-name-servers;
    request domain-name;
    script "{{ .ScriptPath }}";
};
id-assoc pd 0 {
    prefix-interface {{ .LANInterface }} {
        sla-id 0;
        sla-len {{ .SLALen }};
    };
};
`

const testDhcp6cScriptTmpl = `#!/bin/sh
STATE_FILE="{{ .StatePath }}"
echo "lease event"
`

func newIPv6TestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Interfaces = []config.InterfaceConfig{
		{ID: "wan", Device: "eth0", Role: "wan"},
		{ID: "lan", Device: "eth1", Role: "lan"},
	}
	cfg.PPPoE.Username = "user@isp"
	return cfg
}

func newIPv6TestService(t *testing.T, cfg *config.Config) *services.IPv6Service {
	t.Helper()
	return services.NewIPv6ServiceFromFS(cfg, testDhcp6cConfTmpl, testDhcp6cScriptTmpl)
}

func TestNewIPv6Service(t *testing.T) {
	svc := services.NewIPv6Service(newIPv6TestConfig(t))
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestIPv6RenderConfigPPPoE(t *testing.T) {
	cfg := newIPv6TestConfig(t)
	svc := newIPv6TestService(t, cfg)

	out, err := svc.RenderConfig()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "interface ppp0") {
		t.Errorf("expected ppp0 (PPPoE WAN) in output, got:\n%s", out)
	}
	if !strings.Contains(out, "prefix-interface eth1") {
		t.Errorf("expected prefix-interface eth1 (LAN) in output, got:\n%s", out)
	}
	if !strings.Contains(out, "send rapid-commit") {
		t.Errorf("rapid-commit should be enabled by default, got:\n%s", out)
	}
	// /56 default delegation -> SLA len = 64-56 = 8.
	if !strings.Contains(out, "sla-len 8") {
		t.Errorf("expected sla-len 8 for /56, got:\n%s", out)
	}
}

func TestIPv6RenderConfigDirectWAN(t *testing.T) {
	cfg := newIPv6TestConfig(t)
	cfg.PPPoE.Username = "" // simulate non-PPPoE WAN (DHCP/static)
	svc := newIPv6TestService(t, cfg)

	out, err := svc.RenderConfig()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "interface eth0") {
		t.Errorf("expected eth0 (direct WAN) in output, got:\n%s", out)
	}
}

func TestIPv6RenderConfigCustomPrefixHint(t *testing.T) {
	cfg := newIPv6TestConfig(t)
	cfg.IPv6.WAN.PrefixHint = "/60"
	svc := newIPv6TestService(t, cfg)

	out, err := svc.RenderConfig()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// /60 -> SLA len = 64-60 = 4.
	if !strings.Contains(out, "sla-len 4") {
		t.Errorf("expected sla-len 4 for /60, got:\n%s", out)
	}
}

func TestIPv6RenderConfigInvalidHint(t *testing.T) {
	cases := []string{"/40", "/72", "abc", "/-5"}
	for _, hint := range cases {
		cfg := newIPv6TestConfig(t)
		cfg.IPv6.WAN.PrefixHint = hint
		svc := newIPv6TestService(t, cfg)
		if _, err := svc.RenderConfig(); err == nil {
			t.Errorf("expected error for hint %q, got nil", hint)
		}
	}
}

func TestIPv6RenderConfigMissingInterfaces(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Interfaces = nil
	svc := newIPv6TestService(t, cfg)
	if _, err := svc.RenderConfig(); err == nil {
		t.Error("expected error when no interfaces configured")
	}
}

func TestIPv6RenderScriptContainsStatePath(t *testing.T) {
	svc := newIPv6TestService(t, newIPv6TestConfig(t))
	out, err := svc.RenderScript()
	if err != nil {
		t.Fatalf("render script: %v", err)
	}
	if !strings.Contains(out, "/var/lib/lankeeper/state/ipv6-prefix.json") {
		t.Errorf("expected state path in script, got:\n%s", out)
	}
	if !strings.Contains(out, "#!/bin/sh") {
		t.Errorf("script should start with #!/bin/sh shebang, got:\n%s", out)
	}
}

func TestPrefixStateActive(t *testing.T) {
	cases := []struct {
		name string
		ps   services.PrefixState
		want bool
	}{
		{"empty", services.PrefixState{}, false},
		{"valid", services.PrefixState{Prefix: "2001:db8::", PrefixLength: 56, ValidLifetime: 3600, Reason: "REPLY"}, true},
		{"released", services.PrefixState{Prefix: "2001:db8::", PrefixLength: 56, ValidLifetime: 3600, Reason: "RELEASE"}, false},
		{"exit", services.PrefixState{Prefix: "2001:db8::", PrefixLength: 56, ValidLifetime: 3600, Reason: "EXIT"}, false},
		{"expired", services.PrefixState{Prefix: "2001:db8::", PrefixLength: 56, ValidLifetime: 0, Reason: "REPLY"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ps.Active(); got != tc.want {
				t.Errorf("Active() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrefixStateCIDR(t *testing.T) {
	ps := services.PrefixState{Prefix: "2001:db8::", PrefixLength: 56}
	if got := ps.CIDR(); got != "2001:db8::/56" {
		t.Errorf("CIDR() = %q, want 2001:db8::/56", got)
	}

	empty := services.PrefixState{}
	if got := empty.CIDR(); got != "" {
		t.Errorf("empty CIDR() = %q, want empty string", got)
	}
}

func TestIPv6IsDisabledRendersStub(t *testing.T) {
	cfg := newIPv6TestConfig(t)
	cfg.IPv6.Enabled = "off"
	svc := newIPv6TestService(t, cfg)

	// RenderToDisk needs file I/O which calls into netutil; we cannot
	// run that fully in unit tests. RenderConfig is the testable part:
	// when Enabled is "off" callers should simply skip rendering. The
	// RenderConfig stays usable so the caller decides.
	out, err := svc.RenderConfig()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out == "" {
		t.Error("RenderConfig should return content even when disabled")
	}
}
