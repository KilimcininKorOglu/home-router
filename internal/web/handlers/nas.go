package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/i18n"
	"github.com/KilimcininKorOglu/home-router/internal/services"
	"github.com/KilimcininKorOglu/home-router/internal/tmpl"
)

type NASHandler struct {
	renderer *tmpl.Renderer
	nas      *services.NASService
}

func NewNASHandler(renderer *tmpl.Renderer, nas *services.NASService) *NASHandler {
	return &NASHandler{renderer: renderer, nas: nas}
}

func (h *NASHandler) HandlePage(w http.ResponseWriter, r *http.Request) {
	lang := i18n.LangFromContext(r.Context())

	data := &tmpl.PageData{
		Lang: lang,
		Page: "nas",
		Data: map[string]any{
			"Shares":    h.nas.GetShares(),
			"M3UStatus": h.nas.GetM3UStatus(),
		},
	}

	if err := h.renderer.Render(w, "nas", "base", data); err != nil {
		log.Printf("render nas: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *NASHandler) HandleAddShare(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	share := config.ShareConfig{
		Name:     r.FormValue("name"),
		Path:     r.FormValue("path"),
		GuestOK:  r.FormValue("guestOk") == "true" || r.FormValue("guestOk") == "on",
		ReadOnly: r.FormValue("readOnly") == "true" || r.FormValue("readOnly") == "on",
	}

	h.nas.AddShare(share)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/nas", http.StatusSeeOther)
}

func (h *NASHandler) HandleDeleteShare(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.nas.RemoveShare(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/nas", http.StatusSeeOther)
}

func (h *NASHandler) HandleSyncM3U(w http.ResponseWriter, r *http.Request) {
	go h.nas.SyncM3U(r.Context())

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "m3uSyncStarted")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/nas", http.StatusSeeOther)
}

func (h *NASHandler) HandleDiscoverGroups(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	groups, err := h.nas.DiscoverM3UGroups(r.Context(), url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, g := range groups {
		fmt.Fprintf(w, `<label style="display:flex;align-items:center;gap:var(--space-xs);cursor:pointer;padding:var(--space-xs) 0;"><input type="checkbox" name="includeGroups" value="%s" checked> %s</label>`, g, g)
	}
}
