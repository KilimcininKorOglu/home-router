package handlers

import (
	"log"
	"net/http"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/i18n"
	"github.com/KilimcininKorOglu/home-router/internal/tmpl"
	"golang.org/x/crypto/bcrypt"
)

type SystemHandler struct {
	renderer *tmpl.Renderer
	cfg      *config.Config
}

func NewSystemHandler(renderer *tmpl.Renderer, cfg *config.Config) *SystemHandler {
	return &SystemHandler{renderer: renderer, cfg: cfg}
}

func (h *SystemHandler) HandleSettingsPage(w http.ResponseWriter, r *http.Request) {
	lang := i18n.LangFromContext(r.Context())

	data := &tmpl.PageData{
		Lang: lang,
		Page: "settings",
		Data: map[string]any{
			"Hostname": h.cfg.System.Hostname,
			"Timezone": h.cfg.System.Timezone,
			"Language": h.cfg.System.Language,
			"TLSMode":  h.cfg.System.TLS.Mode,
		},
	}

	if err := h.renderer.Render(w, "settings", "base", data); err != nil {
		log.Printf("render settings: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *SystemHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	lang := i18n.LangFromContext(r.Context())
	r.ParseForm()

	newPassword := r.FormValue("newPassword")
	confirmPassword := r.FormValue("confirmPassword")

	if newPassword != confirmPassword || len(newPassword) < 8 {
		data := &tmpl.PageData{
			Lang:  lang,
			Page:  "settings",
			Error: "Password mismatch or too short (min 8 chars)",
		}
		w.WriteHeader(http.StatusBadRequest)
		h.renderer.Render(w, "settings", "base", data)
		return
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	hash := string(hashBytes)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.cfg.System.AdminPasswordHash = hash

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "passwordChanged")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
