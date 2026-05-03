package services

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

type BackupService struct {
	configDir string
}

func NewBackupService(configDir string) *BackupService {
	return &BackupService{configDir: configDir}
}

func (s *BackupService) Export(ctx context.Context, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/home-router-backup-%s.tar.gz",
			time.Now().Format("20060102-150405"))
	}

	_, err := netutil.Run(ctx, "tar", "czf", outputPath,
		"-C", filepath.Dir(s.configDir), filepath.Base(s.configDir),
		"-C", "/etc", "unbound",
		"-C", "/etc", "dnsmasq.d",
	)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	return nil
}

func (s *BackupService) Import(ctx context.Context, archivePath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	destRoot := filepath.Dir(s.configDir)
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		clean := filepath.Clean(hdr.Name)
		if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("unsafe tar member rejected: %s", hdr.Name)
		}

		target := filepath.Join(destRoot, clean)
		if !strings.HasPrefix(target, destRoot+string(os.PathSeparator)) && target != destRoot {
			return fmt.Errorf("tar member escapes destination: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			netutil.MkdirAll(target, os.FileMode(hdr.Mode)|0o755)
		case tar.TypeReg:
			data, err := io.ReadAll(io.LimitReader(tr, 10<<20))
			if err != nil {
				return fmt.Errorf("read tar member %s: %w", hdr.Name, err)
			}
			if err := netutil.WriteFile(target, data, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("write tar member %s: %w", hdr.Name, err)
			}
		default:
			return fmt.Errorf("unsupported tar member type %d: %s", hdr.Typeflag, hdr.Name)
		}
	}

	return nil
}

func (s *BackupService) FactoryReset(ctx context.Context) error {
	defaultsDir := filepath.Join(filepath.Dir(s.configDir), "configs", "defaults")

	entries, err := os.ReadDir(defaultsDir)
	if err != nil {
		return fmt.Errorf("read defaults: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(defaultsDir, entry.Name())
		dst := filepath.Join(s.configDir, entry.Name())

		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		netutil.WriteFile(dst, data, 0o644)
	}

	return nil
}
