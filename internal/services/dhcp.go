package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

type DHCPService struct {
	cfg *config.Config
}

func NewDHCPService(cfg *config.Config) *DHCPService {
	return &DHCPService{cfg: cfg}
}

type Lease struct {
	Expiry   time.Time
	MAC      string
	IP       string
	Hostname string
	Active   bool
}

func (s *DHCPService) GetLeases() ([]Lease, error) {
	return ParseLeaseFile("/var/lib/misc/dnsmasq.leases")
}

func ParseLeaseFile(path string) ([]Lease, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open lease file: %w", err)
	}
	defer f.Close()

	var leases []Lease
	now := time.Now()
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		expiry, _ := strconv.ParseInt(fields[0], 10, 64)
		lease := Lease{
			Expiry:   time.Unix(expiry, 0),
			MAC:      fields[1],
			IP:       fields[2],
			Hostname: fields[3],
			Active:   expiry == 0 || time.Unix(expiry, 0).After(now),
		}

		if lease.Hostname == "*" {
			lease.Hostname = ""
		}

		leases = append(leases, lease)
	}

	return leases, scanner.Err()
}

func ParseLeaseData(data string) []Lease {
	var leases []Lease
	now := time.Now()

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		expiry, _ := strconv.ParseInt(fields[0], 10, 64)
		lease := Lease{
			Expiry:   time.Unix(expiry, 0),
			MAC:      fields[1],
			IP:       fields[2],
			Hostname: fields[3],
			Active:   expiry == 0 || time.Unix(expiry, 0).After(now),
		}
		if lease.Hostname == "*" {
			lease.Hostname = ""
		}
		leases = append(leases, lease)
	}

	return leases
}

func (s *DHCPService) Reload(ctx context.Context) error {
	_, err := netutil.Run(ctx, "killall", "-HUP", "dnsmasq")
	if err != nil {
		return fmt.Errorf("reload dnsmasq: %w", err)
	}
	return nil
}

func (s *DHCPService) AddStaticLease(mac, ip, hostname string) {
	s.cfg.DHCP.StaticLeases = append(s.cfg.DHCP.StaticLeases, config.StaticLease{
		MAC:      mac,
		IP:       ip,
		Hostname: hostname,
	})
}

func (s *DHCPService) RemoveStaticLease(index int) error {
	if index < 0 || index >= len(s.cfg.DHCP.StaticLeases) {
		return fmt.Errorf("invalid static lease index: %d", index)
	}
	s.cfg.DHCP.StaticLeases = append(
		s.cfg.DHCP.StaticLeases[:index],
		s.cfg.DHCP.StaticLeases[index+1:]...,
	)
	return nil
}

func (s *DHCPService) GetDeviceList() []DeviceInfo {
	leases, _ := s.GetLeases()
	devices := make([]DeviceInfo, 0, len(leases))
	for _, l := range leases {
		if l.Active {
			devices = append(devices, DeviceInfo{
				MAC:      l.MAC,
				IP:       l.IP,
				Hostname: l.Hostname,
			})
		}
	}
	return devices
}

type DeviceInfo struct {
	MAC      string
	IP       string
	Hostname string
}
