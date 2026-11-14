package intent

import (
	"net/netip"
	"strings"

	hostresources "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
)

// ValidProtectionEndpoint checks typed fields without returning platform syntax
// or allocating a resource. It does not admit a management or firewall policy.
func ValidProtectionEndpoint(key hostresources.PublicEndpointKey, source string) bool {
	if !validEndpoint(key) {
		return false
	}
	if source == "" {
		return true
	}
	prefix, err := netip.ParsePrefix(source)
	return err == nil && prefix.Masked().String() == source && prefix.Addr().Is4() == (key.AddressFamily == hostresources.AddressFamilyIPv4)
}

func validEndpoint(key hostresources.PublicEndpointKey) bool {
	if key.Port == 0 || key.Network != hostresources.NetworkTCP && key.Network != hostresources.NetworkUDP ||
		key.AddressFamily != hostresources.AddressFamilyIPv4 && key.AddressFamily != hostresources.AddressFamilyIPv6 {
		return false
	}
	listen := hostresources.NormalizeListen(key.BindAddress)
	if listen.Wildcard() {
		return true
	}
	address, err := netip.ParseAddr(listen.Value)
	return err == nil && !strings.Contains(listen.Value, "%") && address.Is4() == (key.AddressFamily == hostresources.AddressFamilyIPv4)
}
