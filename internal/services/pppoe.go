package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

type PPPoEService struct {
	cfg       *config.Config
	mu        sync.RWMutex
	connected bool
}

func NewPPPoEService(cfg *config.Config) *PPPoEService {
	return &PPPoEService{cfg: cfg}
}

type PPPoEStatus struct {
	Connected bool
	Interface string
	LocalIP   string
	RemoteIP  string
	LocalIPv6 string
	Uptime    string
	PID       int
}

func (s *PPPoEService) Status(ctx context.Context) (*PPPoEStatus, error) {
	pid, err := s.readPID()
	if err != nil || pid == 0 {
		return &PPPoEStatus{Connected: false}, nil
	}

	if !processExists(pid) {
		return &PPPoEStatus{Connected: false}, nil
	}

	status := &PPPoEStatus{
		Connected: true,
		Interface: "ppp0",
		PID:       pid,
	}

	addrs, err := netutil.GetInterfaceAddresses("ppp0")
	if err == nil {
		for _, addr := range addrs {
			if strings.Contains(addr, ".") && status.LocalIP == "" {
				status.LocalIP = strings.SplitN(addr, "/", 2)[0]
			} else if strings.Contains(addr, ":") && !strings.HasPrefix(addr, "fe80") && status.LocalIPv6 == "" {
				status.LocalIPv6 = strings.SplitN(addr, "/", 2)[0]
			}
		}
	}

	s.mu.Lock()
	s.connected = true
	s.mu.Unlock()

	return status, nil
}

func (s *PPPoEService) Connect(ctx context.Context) error {
	if err := s.renderConfig(); err != nil {
		return fmt.Errorf("render pppoe config: %w", err)
	}

	_, err := netutil.Run(ctx, "pppd", "call", "wan")
	if err != nil {
		return fmt.Errorf("start pppd: %w", err)
	}

	s.mu.Lock()
	s.connected = true
	s.mu.Unlock()

	return nil
}

func (s *PPPoEService) Disconnect(ctx context.Context) error {
	pid, err := s.readPID()
	if err != nil {
		return fmt.Errorf("read pppd pid: %w", err)
	}

	if pid > 0 {
		_, err := netutil.Run(ctx, "kill", fmt.Sprintf("%d", pid))
		if err != nil {
			return fmt.Errorf("kill pppd: %w", err)
		}
	}

	s.mu.Lock()
	s.connected = false
	s.mu.Unlock()

	return nil
}

func (s *PPPoEService) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

type peerTemplateData struct {
	WANDevice       string
	Username        string
	MTU             int
	MRU             int
	LCPEchoInterval int
	LCPEchoFailure  int
	Holdoff         int
	IPv6CP          bool
}

func (s *PPPoEService) renderConfig() error {
	var wanDevice string
	for _, iface := range s.cfg.Interfaces {
		if iface.Role == "wan" {
			wanDevice = iface.Device
			break
		}
	}
	if wanDevice == "" {
		return fmt.Errorf("no WAN interface configured")
	}

	data := peerTemplateData{
		WANDevice:       wanDevice,
		Username:        s.cfg.PPPoE.Username,
		MTU:             s.cfg.PPPoE.MTU,
		MRU:             s.cfg.PPPoE.MRU,
		LCPEchoInterval: s.cfg.PPPoE.LCPEchoInterval,
		LCPEchoFailure:  s.cfg.PPPoE.LCPEchoFailure,
		Holdoff:         s.cfg.PPPoE.Holdoff,
		IPv6CP:          s.cfg.PPPoE.IPv6CP,
	}

	if data.MTU == 0 {
		data.MTU = 1492
	}
	if data.MRU == 0 {
		data.MRU = 1492
	}
	if data.LCPEchoInterval == 0 {
		data.LCPEchoInterval = 10
	}
	if data.LCPEchoFailure == 0 {
		data.LCPEchoFailure = 3
	}
	if data.Holdoff == 0 {
		data.Holdoff = 5
	}

	peerDir := "/etc/ppp/peers"
	os.MkdirAll(peerDir, 0o755)

	tmpl, err := template.ParseFiles("configs/sysconf/pppoe-peer.tmpl")
	if err != nil {
		return fmt.Errorf("parse peer template: %w", err)
	}

	f, err := os.Create(filepath.Join(peerDir, "wan"))
	if err != nil {
		return fmt.Errorf("create peer file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("execute peer template: %w", err)
	}

	if s.cfg.PPPoE.Password != "" {
		secretsLine := fmt.Sprintf("%q * %q\n", s.cfg.PPPoE.Username, s.cfg.PPPoE.Password)
		if err := appendToFile("/etc/ppp/chap-secrets", secretsLine); err != nil {
			return fmt.Errorf("write chap-secrets: %w", err)
		}
		if err := appendToFile("/etc/ppp/pap-secrets", secretsLine); err != nil {
			return fmt.Errorf("write pap-secrets: %w", err)
		}
	}

	return nil
}

func (s *PPPoEService) readPID() (int, error) {
	data, err := os.ReadFile("/var/run/ppp0.pid")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("empty pid file")
	}

	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}

	return pid, nil
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

func appendToFile(path, line string) error {
	existing, _ := os.ReadFile(path)
	if strings.Contains(string(existing), strings.TrimSpace(line)) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
