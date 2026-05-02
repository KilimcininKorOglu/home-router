package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/KilimcininKorOglu/home-router/internal/config"
	"github.com/KilimcininKorOglu/home-router/internal/netutil"
)

type NetworkService struct {
	cfg *config.Config
}

func NewNetworkService(cfg *config.Config) *NetworkService {
	return &NetworkService{cfg: cfg}
}

func (s *NetworkService) DetectInterfaces() ([]netutil.InterfaceInfo, error) {
	ifaces, err := netutil.DetectInterfaces()
	if err != nil {
		return nil, err
	}

	var physical []netutil.InterfaceInfo
	for _, iface := range ifaces {
		if !iface.IsVirtual {
			physical = append(physical, iface)
		}
	}
	return physical, nil
}

func (s *NetworkService) GetInterfaceStatus(name string) (*InterfaceStatus, error) {
	state, err := netutil.GetInterfaceState(name)
	if err != nil {
		return nil, err
	}

	addrs, _ := netutil.GetInterfaceAddresses(name)
	rx, tx, _ := netutil.ReadInterfaceStats(name)

	var cfgIface *config.InterfaceConfig
	for i := range s.cfg.Interfaces {
		if s.cfg.Interfaces[i].Device == name || s.cfg.Interfaces[i].ID == name {
			cfgIface = &s.cfg.Interfaces[i]
			break
		}
	}

	status := &InterfaceStatus{
		Name:      name,
		State:     state,
		Addresses: addrs,
		RxBytes:   rx,
		TxBytes:   tx,
	}

	if cfgIface != nil {
		status.Label = cfgIface.Label
		status.Role = cfgIface.Role
		status.MTU = cfgIface.MTU
	}

	return status, nil
}

type InterfaceStatus struct {
	Name      string
	Label     string
	Role      string
	State     string
	MTU       int
	Addresses []string
	RxBytes   uint64
	TxBytes   uint64
}

func (s *NetworkService) CreateVLAN(ctx context.Context, parentDevice string, vid int, address string, mtu int) error {
	if err := netutil.ValidateVLANID(vid); err != nil {
		return err
	}

	vlanDev := fmt.Sprintf("%s.%d", parentDevice, vid)

	_, err := netutil.Run(ctx, "ip", "link", "add", "link", parentDevice,
		"name", vlanDev, "type", "vlan", "id", fmt.Sprintf("%d", vid))
	if err != nil {
		return fmt.Errorf("create VLAN %d: %w", vid, err)
	}

	if mtu > 0 {
		netutil.Run(ctx, "ip", "link", "set", vlanDev, "mtu", fmt.Sprintf("%d", mtu))
	}

	if address != "" {
		_, err = netutil.Run(ctx, "ip", "addr", "add", address, "dev", vlanDev)
		if err != nil {
			return fmt.Errorf("assign address to VLAN %d: %w", vid, err)
		}
	}

	_, err = netutil.Run(ctx, "ip", "link", "set", vlanDev, "up")
	if err != nil {
		return fmt.Errorf("bring up VLAN %d: %w", vid, err)
	}

	return nil
}

func (s *NetworkService) DeleteVLAN(ctx context.Context, parentDevice string, vid int) error {
	vlanDev := fmt.Sprintf("%s.%d", parentDevice, vid)
	_, err := netutil.Run(ctx, "ip", "link", "delete", vlanDev)
	if err != nil {
		return fmt.Errorf("delete VLAN %s: %w", vlanDev, err)
	}
	return nil
}

func (s *NetworkService) RestoreVLANs(ctx context.Context) error {
	var errs []string
	for _, vlan := range s.cfg.VLANs {
		var parentDev string
		for _, iface := range s.cfg.Interfaces {
			if iface.ID == vlan.Parent {
				parentDev = iface.Device
				break
			}
		}
		if parentDev == "" {
			errs = append(errs, fmt.Sprintf("parent %s not found for VLAN %d", vlan.Parent, vlan.VID))
			continue
		}

		if err := s.CreateVLAN(ctx, parentDev, vlan.VID, vlan.Address, vlan.MTU); err != nil {
			errs = append(errs, fmt.Sprintf("VLAN %d: %v", vlan.VID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("VLAN restore errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
