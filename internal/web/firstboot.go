package web

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

const firstBootFlag = "/var/lib/home-router/.first-boot"

func IsFirstBoot() bool {
	_, err := os.Stat(firstBootFlag)
	return err == nil
}

func CompleteFirstBoot() error {
	return os.Remove(firstBootFlag)
}

type firstBootNIC struct {
	Device string
	IP     string
	CIDR   string
}

func SetupFirstBootNetworking(ctx context.Context) ([]firstBootNIC, error) {
	ifaces, err := netutil.DetectInterfaces()
	if err != nil {
		return nil, fmt.Errorf("detect interfaces: %w", err)
	}

	var nics []firstBootNIC
	subnets := []struct{ ip, cidr string }{
		{"10.10.10.1", "10.10.10.1/24"},
		{"10.10.20.1", "10.10.20.1/24"},
		{"10.10.30.1", "10.10.30.1/24"},
		{"10.10.40.1", "10.10.40.1/24"},
	}

	idx := 0
	for _, iface := range ifaces {
		if iface.IsVirtual || iface.Name == "lo" {
			continue
		}
		if idx >= len(subnets) {
			break
		}

		netutil.Run(ctx, "ip", "link", "set", iface.Name, "up")
		netutil.Run(ctx, "ip", "addr", "flush", "dev", iface.Name)
		_, err := netutil.Run(ctx, "ip", "addr", "add", subnets[idx].cidr, "dev", iface.Name)
		if err != nil {
			log.Printf("first-boot: failed to assign IP to %s: %v", iface.Name, err)
			continue
		}

		nics = append(nics, firstBootNIC{
			Device: iface.Name,
			IP:     subnets[idx].ip,
			CIDR:   subnets[idx].cidr,
		})

		log.Printf("first-boot: %s → %s", iface.Name, subnets[idx].cidr)
		idx++
	}

	return nics, nil
}

func TeardownFirstBootNetworking(ctx context.Context, nics []firstBootNIC, keepDevice string) {
	for _, nic := range nics {
		if nic.Device == keepDevice {
			continue
		}
		netutil.Run(ctx, "ip", "addr", "flush", "dev", nic.Device)
		log.Printf("first-boot: removed temporary IP from %s", nic.Device)
	}
}
