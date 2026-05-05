package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KilimcininKorOglu/lankeeper/internal/web"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "TestPass123!"
	hash, err := web.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	auth := web.NewAuth("test-secret-32-bytes-long-enough!", hash)

	if !auth.VerifyPassword(password) {
		t.Error("correct password should verify")
	}

	if auth.VerifyPassword("wrong-password") {
		t.Error("wrong password should not verify")
	}
}

func TestSessionLoginLogout(t *testing.T) {
	hash, _ := web.HashPassword("test123")
	auth := web.NewAuth("test-secret-32-bytes-long-enough!", hash)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if auth.IsAuthenticated(req) {
		t.Error("should not be authenticated before login")
	}

	if err := auth.Login(rec, req); err != nil {
		t.Fatalf("login: %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		loginReq.AddCookie(c)
	}

	if !auth.IsAuthenticated(loginReq) {
		t.Error("should be authenticated after login")
	}

	rec2 := httptest.NewRecorder()
	if err := auth.Logout(rec2, loginReq); err != nil {
		t.Fatalf("logout: %v", err)
	}
}
