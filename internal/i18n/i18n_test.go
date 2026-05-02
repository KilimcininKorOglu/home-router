package i18n_test

import (
	"testing"
	"testing/fstest"

	"github.com/KilimcininKorOglu/home-router/internal/i18n"
)

func newTestI18n(t *testing.T) *i18n.I18n {
	t.Helper()

	fsys := fstest.MapFS{
		"locales/tr.json": &fstest.MapFile{
			Data: []byte(`{
				"nav.dashboard": "Gösterge Paneli",
				"common.save": "Kaydet",
				"greeting": "Hoş geldin, {{name}}"
			}`),
		},
		"locales/en.json": &fstest.MapFile{
			Data: []byte(`{
				"nav.dashboard": "Dashboard",
				"common.save": "Save",
				"greeting": "Welcome, {{name}}"
			}`),
		},
	}

	loc, err := i18n.New("tr")
	if err != nil {
		t.Fatalf("new i18n: %v", err)
	}

	if err := loc.LoadFromFS(fsys, "locales"); err != nil {
		t.Fatalf("load from fs: %v", err)
	}

	return loc
}

func TestT(t *testing.T) {
	loc := newTestI18n(t)

	tests := []struct {
		lang, key, want string
	}{
		{"tr", "nav.dashboard", "Gösterge Paneli"},
		{"en", "nav.dashboard", "Dashboard"},
		{"tr", "common.save", "Kaydet"},
		{"en", "common.save", "Save"},
	}

	for _, tt := range tests {
		got := loc.T(tt.lang, tt.key)
		if got != tt.want {
			t.Errorf("T(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.want)
		}
	}
}

func TestFallback(t *testing.T) {
	loc := newTestI18n(t)

	got := loc.T("fr", "nav.dashboard")
	if got != "Gösterge Paneli" {
		t.Errorf("fallback to tr expected 'Gösterge Paneli', got %q", got)
	}
}

func TestMissingKeyReturnsKey(t *testing.T) {
	loc := newTestI18n(t)

	got := loc.T("en", "nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("missing key should return key itself, got %q", got)
	}
}

func TestWithParams(t *testing.T) {
	loc := newTestI18n(t)

	got := loc.WithParams("tr", "greeting", map[string]string{"name": "Kerem"})
	if got != "Hoş geldin, Kerem" {
		t.Errorf("WithParams got %q, want 'Hoş geldin, Kerem'", got)
	}

	got = loc.WithParams("en", "greeting", map[string]string{"name": "Kerem"})
	if got != "Welcome, Kerem" {
		t.Errorf("WithParams got %q, want 'Welcome, Kerem'", got)
	}
}

func TestHasLocale(t *testing.T) {
	loc := newTestI18n(t)

	if !loc.HasLocale("tr") {
		t.Error("should have tr locale")
	}
	if !loc.HasLocale("en") {
		t.Error("should have en locale")
	}
	if loc.HasLocale("fr") {
		t.Error("should not have fr locale")
	}
}
