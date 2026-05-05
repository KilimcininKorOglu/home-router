package netutil_test

import (
	"testing"

	"github.com/KilimcininKorOglu/lankeeper/internal/netutil"
)

func TestValidateIP(t *testing.T) {
	valid := []string{"10.10.10.1", "192.168.1.1", "8.8.8.8", "::1", "fe80::1"}
	for _, ip := range valid {
		if err := netutil.ValidateIP(ip); err != nil {
			t.Errorf("ValidateIP(%q) should be valid: %v", ip, err)
		}
	}

	invalid := []string{"", "999.999.999.999", "abc", "10.10.10.1/24"}
	for _, ip := range invalid {
		if err := netutil.ValidateIP(ip); err == nil {
			t.Errorf("ValidateIP(%q) should be invalid", ip)
		}
	}
}

func TestValidateCIDR(t *testing.T) {
	valid := []string{"10.10.10.0/24", "192.168.1.0/16", "fd00::/48", "::1/128"}
	for _, cidr := range valid {
		if err := netutil.ValidateCIDR(cidr); err != nil {
			t.Errorf("ValidateCIDR(%q) should be valid: %v", cidr, err)
		}
	}

	invalid := []string{"", "10.10.10.1", "10.10.10.0/33", "abc/24"}
	for _, cidr := range invalid {
		if err := netutil.ValidateCIDR(cidr); err == nil {
			t.Errorf("ValidateCIDR(%q) should be invalid", cidr)
		}
	}
}

func TestValidateMAC(t *testing.T) {
	valid := []string{"aa:bb:cc:dd:ee:ff", "00:11:22:33:44:55", "AA:BB:CC:DD:EE:FF"}
	for _, mac := range valid {
		if err := netutil.ValidateMAC(mac); err != nil {
			t.Errorf("ValidateMAC(%q) should be valid: %v", mac, err)
		}
	}

	invalid := []string{"", "aa:bb:cc:dd:ee", "zz:bb:cc:dd:ee:ff", "aabbccddeeff"}
	for _, mac := range invalid {
		if err := netutil.ValidateMAC(mac); err == nil {
			t.Errorf("ValidateMAC(%q) should be invalid", mac)
		}
	}
}

func TestValidatePort(t *testing.T) {
	if err := netutil.ValidatePort(80); err != nil {
		t.Errorf("port 80 should be valid: %v", err)
	}
	if err := netutil.ValidatePort(0); err == nil {
		t.Error("port 0 should be invalid")
	}
	if err := netutil.ValidatePort(65536); err == nil {
		t.Error("port 65536 should be invalid")
	}
}

func TestValidateVLANID(t *testing.T) {
	if err := netutil.ValidateVLANID(100); err != nil {
		t.Errorf("VLAN 100 should be valid: %v", err)
	}
	if err := netutil.ValidateVLANID(0); err == nil {
		t.Error("VLAN 0 should be invalid")
	}
	if err := netutil.ValidateVLANID(4095); err == nil {
		t.Error("VLAN 4095 should be invalid")
	}
}

func TestValidateMTU(t *testing.T) {
	if err := netutil.ValidateMTU(1500); err != nil {
		t.Errorf("MTU 1500 should be valid: %v", err)
	}
	if err := netutil.ValidateMTU(67); err == nil {
		t.Error("MTU 67 should be invalid")
	}
	if err := netutil.ValidateMTU(9001); err == nil {
		t.Error("MTU 9001 should be invalid")
	}
}

func TestParseCIDRAddress(t *testing.T) {
	ip, prefix, err := netutil.ParseCIDRAddress("10.10.10.1/24")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ip != "10.10.10.1" || prefix != 24 {
		t.Errorf("got ip=%q prefix=%d, want 10.10.10.1/24", ip, prefix)
	}

	_, _, err = netutil.ParseCIDRAddress("invalid")
	if err == nil {
		t.Error("should fail for invalid CIDR")
	}
}
