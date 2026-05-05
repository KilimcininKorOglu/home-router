package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/KilimcininKorOglu/lankeeper/internal/config"
)

func runGenCert() error {
	fs := flag.NewFlagSet("gen-cert", flag.ExitOnError)
	configPath := fs.String("config", "/etc/lankeeper/router.yaml", "config file path")
	dataDir := fs.String("data-dir", "/var/lib/lankeeper", "data directory (cert is written under <data-dir>/tls)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	info, err := config.EnsureTLSCert(&cfg.System.TLS, *dataDir)
	if err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %w", err)
	}

	fmt.Printf("cert: %s\n", info.CertPath)
	fmt.Printf("key:  %s\n", info.KeyPath)
	return nil
}
