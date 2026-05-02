package netutil

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

var macRegex = regexp.MustCompile(`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)

func ValidateIP(s string) error {
	if net.ParseIP(s) == nil {
		return fmt.Errorf("invalid IP address: %s", s)
	}
	return nil
}

func ValidateCIDR(s string) error {
	_, _, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %s", s)
	}
	return nil
}

func ValidateMAC(s string) error {
	if !macRegex.MatchString(s) {
		return fmt.Errorf("invalid MAC address: %s", s)
	}
	return nil
}

func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", port)
	}
	return nil
}

func ValidateVLANID(vid int) error {
	if vid < 1 || vid > 4094 {
		return fmt.Errorf("invalid VLAN ID: %d (must be 1-4094)", vid)
	}
	return nil
}

func ValidateMTU(mtu int) error {
	if mtu < 68 || mtu > 9000 {
		return fmt.Errorf("invalid MTU: %d (must be 68-9000)", mtu)
	}
	return nil
}

func ParseCIDRAddress(cidr string) (ip string, prefix int, err error) {
	parts := strings.SplitN(cidr, "/", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid CIDR: %s", cidr)
	}
	if net.ParseIP(parts[0]) == nil {
		return "", 0, fmt.Errorf("invalid IP in CIDR: %s", parts[0])
	}
	p, err := strconv.Atoi(parts[1])
	if err != nil || p < 0 || p > 128 {
		return "", 0, fmt.Errorf("invalid prefix length in CIDR: %s", parts[1])
	}
	return parts[0], p, nil
}
