package netutil_test

import (
	"context"
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

func TestRunSimple(t *testing.T) {
	out, err := netutil.RunSimple(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("run echo: %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("got %q, want hello", out)
	}
}

func TestRunFailure(t *testing.T) {
	_, err := netutil.Run(context.Background(), "false")
	if err == nil {
		t.Error("running 'false' should return error")
	}
}

func TestRunNonexistent(t *testing.T) {
	_, err := netutil.Run(context.Background(), "nonexistent-command-xyz")
	if err == nil {
		t.Error("running nonexistent command should return error")
	}
}

func TestRunTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	_, err := netutil.Run(ctx, "sleep", "10")
	if err == nil {
		t.Error("should timeout")
	}
}
