package services_test

import (
	"context"
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/lankeeper/internal/config"
	"github.com/KilimcininKorOglu/lankeeper/internal/netutil"
	"github.com/KilimcininKorOglu/lankeeper/internal/services"
)

// TestRebuildClientCountersEmitsExpectedNftCalls drives
// RebuildClientCounters through the production agent path and
// asserts on the recorded exec.run + file.write calls. Verifies:
//
//   - the lankeeper_qos table script is written to a /tmp/ path
//     covered by the agent's file-write whitelist.
//   - `nft -f <script>` is invoked exactly once.
//   - the rendered script flushes the table before declaring it,
//     so consecutive rebuilds remain idempotent.
//   - duplicate MACs in the lease list collapse into a single
//     counter pair.
func TestRebuildClientCountersEmitsExpectedNftCalls(t *testing.T) {
	agent := &fakeAgent{}
	netutil.SetAgentClient(agent)
	t.Cleanup(func() { netutil.SetAgentClient(nil) })

	cfg := &config.Config{}
	cfg.QoS.Enabled = true
	cfg.QoS.Profile = "cake"
	svc := services.NewQoSService(cfg)

	leases := []services.Lease{
		{MAC: "AA:BB:CC:DD:EE:01", IP: "10.10.10.5", Hostname: "alice"},
		{MAC: "aa:bb:cc:dd:ee:01", IP: "10.10.10.5", Hostname: "alice"}, // dup, different case
		{MAC: "AA:BB:CC:DD:EE:02", IP: "10.10.10.6", Hostname: "bob"},
	}

	if err := svc.RebuildClientCounters(context.Background(), leases); err != nil {
		t.Fatalf("RebuildClientCounters: %v", err)
	}

	// Inspect the written script.
	if !agent.wroteFile("lankeeper-qos.nft") {
		t.Fatalf("expected qos nft script under /tmp/, writes: %+v", agent.writeLog)
	}

	var script string
	for _, w := range agent.writeLog {
		if strings.HasSuffix(w.Path, "lankeeper-qos.nft") {
			script = w.Body
		}
	}
	if !strings.Contains(script, "delete table inet lankeeper_qos") {
		t.Errorf("script must flush the table for idempotent reapply, got:\n%s", script)
	}
	if !strings.Contains(script, "table inet lankeeper_qos") {
		t.Errorf("script must declare the table, got:\n%s", script)
	}
	// Two unique MACs → two saddr + two daddr rules + four counters.
	if got := strings.Count(script, "ether saddr "); got != 2 {
		t.Errorf("expected 2 saddr rules (one per unique MAC), got %d in:\n%s", got, script)
	}
	if got := strings.Count(script, "ether daddr "); got != 2 {
		t.Errorf("expected 2 daddr rules (one per unique MAC), got %d in:\n%s", got, script)
	}
	if got := strings.Count(script, "counter cli_"); got != 4 {
		t.Errorf("expected 4 counter declarations (in+out per MAC), got %d in:\n%s", got, script)
	}

	// nft -f should be the single exec.run we issued.
	if got := agent.execCount("nft"); got != 1 {
		t.Errorf("expected exactly one nft invocation, got %d (calls: %+v)",
			got, agent.execCallsCopy())
	}
	calls := agent.execCallsCopy()
	if len(calls) == 0 {
		t.Fatal("no exec calls recorded")
	}
	gotArgs := strings.Join(calls[0].Args, " ")
	if !strings.Contains(gotArgs, "-f") || !strings.Contains(gotArgs, "lankeeper-qos.nft") {
		t.Errorf("expected `nft -f /tmp/lankeeper-qos.nft`, got: %v", calls[0].Args)
	}
}

// TestRebuildClientCountersEmptyLeasesFlushes asserts that an empty
// lease list still produces a "delete table" line so a previous
// snapshot is fully torn down — but no per-MAC rules are emitted.
func TestRebuildClientCountersEmptyLeasesFlushes(t *testing.T) {
	agent := &fakeAgent{}
	netutil.SetAgentClient(agent)
	t.Cleanup(func() { netutil.SetAgentClient(nil) })

	cfg := &config.Config{}
	svc := services.NewQoSService(cfg)

	if err := svc.RebuildClientCounters(context.Background(), nil); err != nil {
		t.Fatalf("RebuildClientCounters(nil): %v", err)
	}

	var script string
	for _, w := range agent.writeLog {
		if strings.HasSuffix(w.Path, "lankeeper-qos.nft") {
			script = w.Body
		}
	}
	if !strings.Contains(script, "delete table inet lankeeper_qos") {
		t.Errorf("empty rebuild must still flush, got:\n%s", script)
	}
	if strings.Contains(script, "ether saddr ") || strings.Contains(script, "ether daddr ") {
		t.Errorf("empty rebuild must not emit per-MAC rules, got:\n%s", script)
	}
}
