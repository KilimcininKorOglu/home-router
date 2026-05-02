package web_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/web"
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

	xcto := rec.Header().Get("X-Content-Type-Options")
	if xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", xcto)
	}

	xfo := rec.Header().Get("X-Frame-Options")
	if xfo != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", xfo)
	}
}
