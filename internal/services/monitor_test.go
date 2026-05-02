package services_test

import (
	"testing"
	"time"

	"github.com/KilimcininKorOglu/home-router/internal/services"
)

func TestNewMonitorService(t *testing.T) {
	svc := services.NewMonitorService()
	if svc == nil {
		t.Fatal("service should not be nil")
	}
}

func TestMonitorCollect(t *testing.T) {
	svc := services.NewMonitorService()

	stop := make(chan struct{})
	go svc.Start(stop, []string{"lo"})

	time.Sleep(1500 * time.Millisecond)
	close(stop)

	stats := svc.GetCurrent()
	if stats.Timestamp.IsZero() {
		t.Error("timestamp should not be zero after collection")
	}

	history := svc.GetHistory()
	if len(history) == 0 {
		t.Error("history should not be empty after 1.5s")
	}
}

func TestMonitorGetCurrentEmpty(t *testing.T) {
	svc := services.NewMonitorService()
	stats := svc.GetCurrent()
	if !stats.Timestamp.IsZero() {
		t.Error("timestamp should be zero before any collection")
	}
}
