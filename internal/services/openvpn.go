package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

type OpenVPNService struct {
	cfg *config.Config
	mu  sync.RWMutex
}

func NewOpenVPNService(cfg *config.Config) *OpenVPNService {
	return &OpenVPNService{cfg: cfg}
}

type OVPNServerStatus struct {
	Enabled      bool
	Active       bool
	PKIReady     bool
	Port         int
	Protocol     string
	ClientCount  int
}

func (s *OpenVPNService) ServerStatus(ctx context.Context) (*OVPNServerStatus, error) {
	status := &OVPNServerStatus{
		Enabled:  s.cfg.OpenVPN.Server.Enabled,
		Port:     s.cfg.OpenVPN.Server.Port,
		Protocol: s.cfg.OpenVPN.Server.Protocol,
	}

	if _, err := os.Stat("/etc/openvpn/pki/ca.crt"); err == nil {
		status.PKIReady = true
	}

	_, err := netutil.Run(ctx, "pgrep", "-x", "openvpn")
	status.Active = err == nil

	status.ClientCount = len(s.cfg.OpenVPN.Server.Clients)

	return status, nil
}

func (s *OpenVPNService) InitPKI(ctx context.Context) error {
	pkiDir := "/etc/openvpn/pki"
	easyrsaDir := "/usr/share/easy-rsa"

	os.MkdirAll(pkiDir, 0o700)
	os.Setenv("EASYRSA_PKI", pkiDir)

	easyrsa := easyrsaDir + "/easyrsa"

	if _, err := netutil.Run(ctx, easyrsa, "init-pki"); err != nil {
		return fmt.Errorf("init-pki: %w", err)
	}

	if _, err := netutil.Run(ctx, easyrsa, "build-ca", "nopass"); err != nil {
		return fmt.Errorf("build-ca: %w", err)
	}

	if _, err := netutil.Run(ctx, easyrsa, "gen-req", "server", "nopass"); err != nil {
		return fmt.Errorf("gen-req server: %w", err)
	}

	if _, err := netutil.Run(ctx, easyrsa, "sign-req", "server", "server"); err != nil {
		return fmt.Errorf("sign-req server: %w", err)
	}

	if _, err := netutil.Run(ctx, easyrsa, "gen-dh"); err != nil {
		return fmt.Errorf("gen-dh: %w", err)
	}

	if _, err := netutil.Run(ctx, "openvpn", "--genkey", "secret", pkiDir+"/ta.key"); err != nil {
		return fmt.Errorf("gen tls-auth key: %w", err)
	}

	if _, err := netutil.Run(ctx, easyrsa, "gen-crl"); err != nil {
		return fmt.Errorf("gen-crl: %w", err)
	}

	log.Println("OpenVPN PKI initialized")
	return nil
}

func (s *OpenVPNService) AddClient(ctx context.Context, name string) error {
	easyrsa := "/usr/share/easy-rsa/easyrsa"
	os.Setenv("EASYRSA_PKI", "/etc/openvpn/pki")

	if _, err := netutil.Run(ctx, easyrsa, "gen-req", name, "nopass"); err != nil {
		return fmt.Errorf("gen-req %s: %w", name, err)
	}

	if _, err := netutil.Run(ctx, easyrsa, "sign-req", "client", name); err != nil {
		return fmt.Errorf("sign-req %s: %w", name, err)
	}

	s.mu.Lock()
	s.cfg.OpenVPN.Server.Clients = append(s.cfg.OpenVPN.Server.Clients, config.OVPNClientEntry{
		Name:       name,
		CommonName: name,
		Enabled:    true,
	})
	s.mu.Unlock()

	log.Printf("OpenVPN client %q added", name)
	return nil
}

func (s *OpenVPNService) RevokeClient(ctx context.Context, name string) error {
	easyrsa := "/usr/share/easy-rsa/easyrsa"
	os.Setenv("EASYRSA_PKI", "/etc/openvpn/pki")

	if _, err := netutil.Run(ctx, easyrsa, "revoke", name); err != nil {
		return fmt.Errorf("revoke %s: %w", name, err)
	}

	if _, err := netutil.Run(ctx, easyrsa, "gen-crl"); err != nil {
		return fmt.Errorf("gen-crl: %w", err)
	}

	s.mu.Lock()
	for i := range s.cfg.OpenVPN.Server.Clients {
		if s.cfg.OpenVPN.Server.Clients[i].Name == name {
			s.cfg.OpenVPN.Server.Clients[i].Enabled = false
			break
		}
	}
	s.mu.Unlock()

	log.Printf("OpenVPN client %q revoked", name)
	return nil
}

func (s *OpenVPNService) GenerateClientOVPN(name string) (string, error) {
	pkiDir := "/etc/openvpn/pki"

	ca, err := os.ReadFile(pkiDir + "/ca.crt")
	if err != nil {
		return "", fmt.Errorf("read CA: %w", err)
	}

	cert, err := os.ReadFile(fmt.Sprintf("%s/issued/%s.crt", pkiDir, name))
	if err != nil {
		return "", fmt.Errorf("read cert: %w", err)
	}

	key, err := os.ReadFile(fmt.Sprintf("%s/private/%s.key", pkiDir, name))
	if err != nil {
		return "", fmt.Errorf("read key: %w", err)
	}

	ta, err := os.ReadFile(pkiDir + "/ta.key")
	if err != nil {
		return "", fmt.Errorf("read ta.key: %w", err)
	}

	srv := s.cfg.OpenVPN.Server
	var sb strings.Builder
	fmt.Fprintf(&sb, "client\n")
	fmt.Fprintf(&sb, "dev tun\n")
	fmt.Fprintf(&sb, "proto %s\n", srv.Protocol)
	fmt.Fprintf(&sb, "remote <YOUR_PUBLIC_IP> %d\n", srv.Port)
	fmt.Fprintf(&sb, "resolv-retry infinite\n")
	fmt.Fprintf(&sb, "nobind\n")
	fmt.Fprintf(&sb, "persist-key\n")
	fmt.Fprintf(&sb, "persist-tun\n")
	fmt.Fprintf(&sb, "cipher %s\n", srv.Cipher)
	fmt.Fprintf(&sb, "auth %s\n", srv.Auth)
	fmt.Fprintf(&sb, "key-direction 1\n")
	fmt.Fprintf(&sb, "verb 3\n\n")
	fmt.Fprintf(&sb, "<ca>\n%s</ca>\n\n", ca)
	fmt.Fprintf(&sb, "<cert>\n%s</cert>\n\n", cert)
	fmt.Fprintf(&sb, "<key>\n%s</key>\n\n", key)
	fmt.Fprintf(&sb, "<tls-auth>\n%s</tls-auth>\n", ta)

	return sb.String(), nil
}

func (s *OpenVPNService) ServerStart(ctx context.Context) error {
	_, err := netutil.Run(ctx, "systemctl", "start", "openvpn@server")
	return err
}

func (s *OpenVPNService) ServerStop(ctx context.Context) error {
	_, err := netutil.Run(ctx, "systemctl", "stop", "openvpn@server")
	return err
}

func (s *OpenVPNService) ImportClientConfig(name, ovpnContent string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.OpenVPN.Clients = append(s.cfg.OpenVPN.Clients, config.OVPNClientConfig{
		Name:       name,
		ConfigFile: ovpnContent,
	})
}

func (s *OpenVPNService) ConnectClient(ctx context.Context, name string) error {
	for _, c := range s.cfg.OpenVPN.Clients {
		if c.Name == name {
			confPath := fmt.Sprintf("/etc/openvpn/client/%s.conf", name)
			os.MkdirAll("/etc/openvpn/client", 0o700)
			os.WriteFile(confPath, []byte(c.ConfigFile), 0o600)

			_, err := netutil.Run(ctx, "openvpn", "--config", confPath, "--daemon",
				"--writepid", fmt.Sprintf("/var/run/openvpn-%s.pid", name))
			return err
		}
	}
	return fmt.Errorf("client %q not found", name)
}
