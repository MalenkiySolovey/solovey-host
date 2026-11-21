package intent

import (
	"fmt"
	"slices"
	"strings"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

const (
	ZapretPacketPathSchema = "solovey-ui/zapret-packet-path/v1"
	ZapretOwner            = "zapret"
	ZapretResourcePrefix   = "outbound:"
	ZapretPurpose          = "direct-outbound-zapret"
)

// PacketProtocol is the transport family that the selected Strategy can
// process. It is deliberately narrower than a CoreRuntime protocol catalog.
type PacketProtocol string

const (
	PacketProtocolTCP PacketProtocol = "tcp"
	PacketProtocolUDP PacketProtocol = "udp"
)

// PacketDirection describes which side of a selected flow is required by the
// native engine. Outbound is mandatory; inbound is admitted only when a
// Strategy explicitly needs bounded return traffic.
type PacketDirection string

const (
	PacketDirectionOutbound PacketDirection = "outbound"
	PacketDirectionInbound  PacketDirection = "inbound"
)

// ZapretPacketPathIntent is the only packet-handoff input accepted from the
// Zapret semantic owner. It contains no queue, mark, nft, fw4 or shell data.
// Those values are allocated by the existing Network Planner.
type ZapretPacketPathIntent struct {
	Schema           string            `json:"schema"`
	HostRef          hostref.Ref       `json:"hostRef"`
	Owner            string            `json:"owner"`
	ResourceRef      string            `json:"resourceRef"`
	OutboundRef      string            `json:"outboundRef"`
	StrategyRef      string            `json:"strategyRef"`
	StrategyRevision string            `json:"strategyRevision"`
	DesiredRevision  string            `json:"desiredRevision"`
	InputRevision    string            `json:"inputRevision"`
	Protocols        []PacketProtocol  `json:"protocols"`
	TCPPorts         []uint16          `json:"tcpPorts,omitempty"`
	UDPPorts         []uint16          `json:"udpPorts,omitempty"`
	Directions       []PacketDirection `json:"directions"`
	NeedsReturn      bool              `json:"needsReturn"`
	Purpose          string            `json:"purpose"`
}

// CanonicalZapretPacketPathIntent returns a copy with ordered lists.
// It never invents an identity or changes the selected Outbound/Strategy.
func CanonicalZapretPacketPathIntent(value ZapretPacketPathIntent) ZapretPacketPathIntent {
	value.Protocols = slices.Clone(value.Protocols)
	value.TCPPorts = slices.Clone(value.TCPPorts)
	value.UDPPorts = slices.Clone(value.UDPPorts)
	value.Directions = slices.Clone(value.Directions)
	slices.Sort(value.Protocols)
	slices.Sort(value.TCPPorts)
	slices.Sort(value.UDPPorts)
	slices.Sort(value.Directions)
	return value
}

func (value ZapretPacketPathIntent) Validate() error {
	if value.Schema != ZapretPacketPathSchema || !value.HostRef.IsLocal() || value.Owner != ZapretOwner ||
		!token(value.OutboundRef, 192) || !token(value.StrategyRef, 192) ||
		!digest(value.StrategyRevision) || !digest(value.DesiredRevision) || !digest(value.InputRevision) ||
		value.Purpose != ZapretPurpose || len(value.Protocols) == 0 || len(value.Protocols) > 2 ||
		len(value.Directions) == 0 || len(value.Directions) > 2 {
		return fmt.Errorf("zapret packet-path intent is malformed")
	}
	if value.ResourceRef != ZapretResourcePrefix+value.OutboundRef {
		return fmt.Errorf("zapret packet-path resource does not identify its outbound")
	}
	for index, protocol := range value.Protocols {
		if protocol != PacketProtocolTCP && protocol != PacketProtocolUDP || index > 0 && protocol == value.Protocols[index-1] {
			return fmt.Errorf("zapret packet-path protocols are invalid")
		}
	}
	if !validZapretPorts(value.TCPPorts, slices.Contains(value.Protocols, PacketProtocolTCP)) ||
		!validZapretPorts(value.UDPPorts, slices.Contains(value.Protocols, PacketProtocolUDP)) {
		return fmt.Errorf("zapret packet-path requires explicit, bounded ports for each protocol")
	}
	hasOutbound := false
	hasInbound := false
	for _, direction := range value.Directions {
		switch direction {
		case PacketDirectionOutbound:
			if hasOutbound {
				return fmt.Errorf("zapret packet-path direction is duplicated")
			}
			hasOutbound = true
		case PacketDirectionInbound:
			if hasInbound {
				return fmt.Errorf("zapret packet-path direction is duplicated")
			}
			hasInbound = true
		default:
			return fmt.Errorf("zapret packet-path direction %q is unsupported", direction)
		}
	}
	if !hasOutbound || value.NeedsReturn != hasInbound {
		return fmt.Errorf("zapret packet-path directions do not match return requirement")
	}
	return nil
}

func validZapretPorts(ports []uint16, required bool) bool {
	if (len(ports) > 0) != required || len(ports) > 64 {
		return false
	}
	seen := make(map[uint16]bool, len(ports))
	for _, port := range ports {
		if port == 0 || seen[port] {
			return false
		}
		seen[port] = true
	}
	return true
}

func token(value string, limit int) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("._:/@+-", char) {
			continue
		}
		return false
	}
	return true
}

func digest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') {
			continue
		}
		return false
	}
	return true
}
