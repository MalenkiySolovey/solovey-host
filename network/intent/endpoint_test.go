package intent

import (
	"testing"

	hostresources "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
)

func TestEndpointFieldsRejectForeignFamilyAndUnboundedSyntax(t *testing.T) {
	key := hostresources.PublicEndpointKey{Network: hostresources.NetworkTCP, AddressFamily: hostresources.AddressFamilyIPv4, BindAddress: "127.0.0.1", Port: 443}
	if !ValidProtectionEndpoint(key, "") || !ValidProtectionEndpoint(key, "192.0.2.0/24") {
		t.Fatal("valid exact endpoint rejected")
	}
	for _, source := range []string{"192.0.2.1/24", "2001:db8::/32", "192.0.2.0/24;accept", "192.0.2.0/999"} {
		if ValidProtectionEndpoint(key, source) {
			t.Fatalf("invalid source accepted: %q", source)
		}
	}
	for _, change := range []func(*hostresources.PublicEndpointKey){
		func(k *hostresources.PublicEndpointKey) { k.Port = 0 },
		func(k *hostresources.PublicEndpointKey) { k.Network = "unknown" },
		func(k *hostresources.PublicEndpointKey) { k.BindAddress = "::1" },
		func(k *hostresources.PublicEndpointKey) {
			k.BindAddress = "fe80::1%eth0"
			k.AddressFamily = hostresources.AddressFamilyIPv6
		},
	} {
		current := key
		change(&current)
		if ValidProtectionEndpoint(current, "") {
			t.Fatalf("invalid endpoint accepted: %#v", current)
		}
	}
}
