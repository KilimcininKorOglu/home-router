package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/i18n"
	"github.com/KilimcininKorOglu/home-router/internal/netutil"
	"github.com/KilimcininKorOglu/home-router/internal/services"
	"github.com/KilimcininKorOglu/home-router/internal/tmpl"
)

type FirewallHandler struct {
	renderer *tmpl.Renderer
	firewall *services.FirewallService
	cfg      *config.Config
}

func NewFirewallHandler(renderer *tmpl.Renderer, firewall *services.FirewallService, cfg *config.Config) *FirewallHandler {
	return &FirewallHandler{
		renderer: renderer,
		firewall: firewall,
		cfg:      cfg,
	}
}

func (h *FirewallHandler) HandlePage(w http.ResponseWriter, r *http.Request) {
	lang := i18n.LangFromContext(r.Context())

	data := &tmpl.PageData{
		Lang: lang,
		Page: "firewall",
		Data: map[string]any{
			"OpenPorts":     h.firewall.GetOpenPorts(),
			"PortForwards":  h.cfg.Firewall.PortForwards,
			"Rules":         h.firewall.GetCustomRules(),
			"TTLFixEnabled": h.cfg.Firewall.TTLFix.Enabled,
			"TTLFixValue":   h.cfg.Firewall.TTLFix.Value,
			"PendingChange": h.firewall.HasPendingChange(),
		},
	}

	if err := h.renderer.Render(w, "firewall", "base", data); err != nil {
		log.Printf("render firewall: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *FirewallHandler) HandleApply(w http.ResponseWriter, r *http.Request) {
	if err := h.firewall.Apply(r.Context()); err != nil {
		log.Printf("apply firewall: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "firewallApplied")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleConfirm(w http.ResponseWriter, r *http.Request) {
	h.firewall.Confirm()

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "firewallConfirmed")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleRollback(w http.ResponseWriter, r *http.Request) {
	if err := h.firewall.Rollback(r.Context()); err != nil {
		log.Printf("rollback firewall: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "firewallRolledBack")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleAddPortForward(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	extPort, err := strconv.Atoi(r.FormValue("extPort"))
	if err != nil || netutil.ValidatePort(extPort) != nil {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	intPort, err := strconv.Atoi(r.FormValue("intPort"))
	if err != nil || netutil.ValidatePort(intPort) != nil {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}

	pf := config.PortForward{
		Name:     r.FormValue("name"),
		Protocol: r.FormValue("protocol"),
		ExtPort:  extPort,
		IntIP:    r.FormValue("intIP"),
		IntPort:  intPort,
		Enabled:  true,
	}

	if err := h.firewall.AddPortForward(pf); err != nil {
		log.Printf("add port forward: %v", err)
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "portForwardAdded")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleDeletePortForward(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	if err := h.firewall.RemovePortForward(idx); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "portForwardDeleted")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleAddRule(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	port, err := strconv.Atoi(r.FormValue("port"))
	if err != nil || netutil.ValidatePort(port) != nil {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}

	rule := config.FirewallRule{
		Name:      r.FormValue("name"),
		Chain:     r.FormValue("chain"),
		Action:    r.FormValue("action"),
		SrcIP:     r.FormValue("srcIP"),
		DstIP:     r.FormValue("dstIP"),
		Protocol:  r.FormValue("protocol"),
		Port:      port,
		Interface: r.FormValue("interface"),
		Direction: r.FormValue("direction"),
		Enabled:   true,
	}

	if err := h.firewall.AddRule(rule); err != nil {
		log.Printf("add rule: %v", err)
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleDeleteRule(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	if err := h.firewall.RemoveRule(idx); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleToggleRule(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	enabled := r.FormValue("enabled") == "true"

	if err := h.firewall.ToggleRule(idx, enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleAddOpenPort(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	port, err := strconv.Atoi(r.FormValue("port"))
	if err != nil || netutil.ValidatePort(port) != nil {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}

	op := config.OpenPort{
		Name:     r.FormValue("name"),
		Protocol: r.FormValue("protocol"),
		Port:     port,
		Source:   r.FormValue("source"),
		Enabled:  true,
	}

	if err := h.firewall.AddOpenPort(op); err != nil {
		log.Printf("add open port: %v", err)
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleDeleteOpenPort(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	if err := h.firewall.RemoveOpenPort(idx); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}

func (h *FirewallHandler) HandleToggleOpenPort(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	enabled := r.FormValue("enabled") == "true"

	if err := h.firewall.ToggleOpenPort(idx, enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/firewall", http.StatusSeeOther)
}
