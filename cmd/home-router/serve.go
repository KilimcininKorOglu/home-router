package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/KilimcininKorOglu/home-router/internal/agent"
	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/i18n"
)

func runServe() error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "/etc/home-router/router.yaml", "config file path")
	socketPath := fs.String("socket", "/run/home-router/agent.sock", "agent UDS path")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	loc, err := i18n.New(cfg.System.Language)
	if err != nil {
		return fmt.Errorf("failed to init i18n: %w", err)
	}

	agentClient := agent.NewClient(*socketPath)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("home-router serve starting (bind=%s:%d, lang=%s)",
		cfg.System.WebBind, cfg.System.WebPort, loc.Fallback())

	_ = agentClient
	_ = ctx

	<-ctx.Done()
	log.Println("serve shutting down...")
	return nil
}
