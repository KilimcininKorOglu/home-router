package handlers

import (
	"log"
	"net/http"

	"github.com/KilimcininKorOglu/home-router/internal/i18n"
	"github.com/KilimcininKorOglu/home-router/internal/services"
	"github.com/KilimcininKorOglu/home-router/internal/tmpl"
)

type DashboardHandler struct {
	renderer *tmpl.Renderer
	monitor  *services.MonitorService
	pppoe    *services.PPPoEService
	dhcp     *services.DHCPService
}

func NewDashboardHandler(
	renderer *tmpl.Renderer,
	monitor *services.MonitorService,
	pppoe *services.PPPoEService,
	dhcp *services.DHCPService,
) *DashboardHandler {
	return &DashboardHandler{
		renderer: renderer,
		monitor:  monitor,
		pppoe:    pppoe,
		dhcp:     dhcp,
	}
}

func (h *DashboardHandler) HandlePage(w http.ResponseWriter, r *http.Request) {
	lang := i18n.LangFromContext(r.Context())

	stats := h.monitor.GetCurrent()
	pppoeStatus, _ := h.pppoe.Status(r.Context())
	devices := h.dhcp.GetDeviceList()

	data := &tmpl.PageData{
		Lang: lang,
		Page: "dashboard",
		Data: map[string]any{
			"Stats":        stats,
			"PPPoE":        pppoeStatus,
			"DeviceCount":  len(devices),
			"History":      h.monitor.GetHistory(),
		},
	}

	if err := h.renderer.Render(w, "dashboard", "base", data); err != nil {
		log.Printf("render dashboard: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
