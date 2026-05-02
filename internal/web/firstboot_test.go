package web_test

import (
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/web"
)

func TestIsFirstBootFalse(t *testing.T) {
	if web.IsFirstBoot() {
		t.Error("should return false when flag file doesn't exist")
	}
}

func TestCompleteFirstBootNoFlag(t *testing.T) {
	err := web.CompleteFirstBoot()
	if err == nil {
		t.Error("should error when flag file doesn't exist")
	}
}
