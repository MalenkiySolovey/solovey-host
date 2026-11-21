package intent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

const (
	ZapretPacketPathSchema = "solovey-ui/zapret-packet-path/v1"
	ZapretOwner            = "zapret"
	ZapretResourcePrefix   = "outbound:"
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
	Directions       []PacketDirection `json:"directions"`
	NeedsReturn      bool              `json:"needsReturn"`
	Purpose          string            `json:"purpose"`
}

// CanonicalZapretPacketPathIntent returns a copy with bounded ordered lists.
// It never invents an identity or changes the selected Outbound/Strategy.
func CanonicalZapretPacketPathIntent(value ZapretPacketPathIntent) ZapretPacketPathIntent {
	value.Protocols = append([]PacketProtocol(nil), value.Protocols...)
	value.Directions = append([]PacketDirection(nil), value.Directions...)
	sort.Slice(value.Protocols, func(i, j int) bool { return value.Protocols[i] < value.Protocols[j] })
	sort.Slice(value.Directions, func(i, j int) bool { return value.Directions[i] < value.Directions[j] })
	value.Protocols = uniqueProtocols(value.Protocols)
	value.Directions = uniqueDirections(value.Directions)
	return value
}

func (value ZapretPacketPathIntent) Validate() error {
	value = CanonicalZapretPacketPathIntent(value)
	if value.Schema != ZapretPacketPathSchema || !value.HostRef.IsLocal() || value.Owner != ZapretOwner ||
		!token(value.ResourceRef, 256) || !strings.HasPrefix(value.ResourceRef, ZapretResourcePrefix) ||
		!token(strings.TrimPrefix(value.ResourceRef, ZapretResourcePrefix), 192) ||
		!token(value.OutboundRef, 192) || !token(value.StrategyRef, 192) ||
		!digest(value.StrategyRevision) || !digest(value.DesiredRevision) || !digest(value.InputRevision) ||
		!token(value.Purpose, 96) || len(value.Protocols) == 0 || len(value.Protocols) > 2 ||
		len(value.Directions) == 0 || len(value.Directions) > 2 {
		return fmt.Errorf("zapret packet-path intent is malformed")
	}
	if value.ResourceRef != ZapretResourcePrefix+value.OutboundRef {
		return fmt.Errorf("zapret packet-path resource does not identify its outbound")
	}
	for _, protocol := range value.Protocols {
		if protocol != PacketProtocolTCP && protocol != PacketProtocolUDP {
			return fmt.Errorf("zapret packet-path protocol %q is unsupported", protocol)
		}
	}
	hasOutbound := false
	hasInbound := false
	for _, direction := range value.Directions {
		switch direction {
		case PacketDirectionOutbound:
			hasOutbound = true
		case PacketDirectionInbound:
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

func uniqueProtocols(values []PacketProtocol) []PacketProtocol {
	result := make([]PacketProtocol, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func uniqueDirections(values []PacketDirection) []PacketDirection {
	result := make([]PacketDirection, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
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
