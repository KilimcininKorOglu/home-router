package web_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/lankeeper/internal/web"
)

func TestRateLimiter(t *testing.T) {
	rl := web.NewRateLimiter(10, 5)

	for i := 0; i < 5; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Errorf("request %d should be allowed (within burst)", i)
		}
	}

	if rl.Allow("192.168.1.1") {
		t.Error("request beyond burst should be denied")
	}

	if !rl.Allow("192.168.1.2") {
		t.Error("different IP should be allowed")
	}
}

// TestRateLimiterMiddlewareReturns429 guarantees that a denied
// request short-circuits with 429 Too Many Requests instead of
// reaching the wrapped handler. This is the contract the DoT-probe
// route relies on to cap goroutine occupancy: at burst+1 the
// blocking inner handler is never invoked, so the per-IP request
// rate caps the per-IP goroutine count.
func TestRateLimiterMiddlewareReturns429(t *testing.T) {
	rl := web.NewRateLimiter(1, 2)
	calls := 0
	wrapped := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))

	doRequest := func() int {
		req := httptest.NewRequest(http.MethodPost, "/probe", nil)
		req.RemoteAddr = "10.10.10.50:12345"
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 2; i++ {
		if code := doRequest(); code != http.StatusOK {
			t.Fatalf("burst request %d: code = %d, want 200", i, code)
		}
	}
	if code := doRequest(); code != http.StatusTooManyRequests {
		t.Fatalf("post-burst request: code = %d, want 429", code)
	}
	if calls != 2 {
		t.Fatalf("inner handler ran %d times, want 2 (burst limit)", calls)
	}
}

func TestLANOnlyMiddleware(t *testing.T) {
	_, lanNet, _ := net.ParseCIDR("10.10.10.0/24")
	middleware := web.LANOnly([]*net.IPNet{lanNet})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		remoteAddr string
		wantCode   int
	}{
		{"10.10.10.50:12345", http.StatusOK},
		{"10.10.10.1:8080", http.StatusOK},
		{"127.0.0.1:9999", http.StatusOK},
		{"8.8.8.8:443", http.StatusForbidden},
		{"192.168.1.1:80", http.StatusForbidden},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tt.remoteAddr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != tt.wantCode {
			t.Errorf("LANOnly(%s) = %d, want %d", tt.remoteAddr, rec.Code, tt.wantCode)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := web.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("CSP header should be set")
	}
	// frame-ancestors 'none' must be present so CSP-conforming
	// browsers keep clickjacking protection even if they ignore the
	// (now-obsolete) X-Frame-Options header.
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP missing frame-ancestors 'none', got %q", csp)
	}

	xcto := rec.Header().Get("X-Content-Type-Options")
	if xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", xcto)
	}

	xfo := rec.Header().Get("X-Frame-Options")
	if xfo != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", xfo)
	}
}
