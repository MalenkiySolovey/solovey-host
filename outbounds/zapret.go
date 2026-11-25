package outbounds

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrZapretSocketRequired = errors.New("Zapret requires an outbound that opens a qualified egress socket")
	// ErrZapretDirectRequired is retained for existing clients of Validate.
	ErrZapretDirectRequired = ErrZapretSocketRequired
	ErrZapretStrategyRef    = errors.New("Zapret strategy reference is missing or invalid")
	zapretStrategyID        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	zapretOutboundID        = regexp.MustCompile(`^[A-Za-z0-9._:/@+\-]{1,192}$`)
)

// ZapretBinding is the Outbounds owner's desired attachment, not an engine
// configuration or a permission to use the network. The Strategy lives in
// the optional component and its absence must not erase this value.
type ZapretBinding struct {
	OutboundRef string `json:"outboundRef"`
	Enabled     bool   `json:"enabled"`
	StrategyRef string `json:"strategyRef,omitempty"`
}

func ValidZapretStrategyRef(ref string) bool { return zapretStrategyID.MatchString(ref) }

func (binding ZapretBinding) Validate(outboundRef, kind, detour string) error {
	if !zapretOutboundID.MatchString(outboundRef) || binding.OutboundRef != outboundRef {
		return fmt.Errorf("%w: binding identity does not match the outbound", ErrZapretSocketRequired)
	}
	if detour != "" && (binding.Enabled || kind == "direct") {
		return fmt.Errorf("%w: the detour %q owns egress selection for this hop", ErrZapretSocketRequired, detour)
	}
	// These kinds can delegate to a shared dialer. The selected provider and
	// installed broker still have to qualify the concrete socket path.
	switch kind {
	case "direct", "http", "hysteria2", "tuic", "vless", "vmess", "trojan", "anytls", "shadowtls", "ssh", "socks", "shadowsocks", "snell":
	default:
		return fmt.Errorf("%w: no shared dial path admitted for outbound type %q", ErrZapretSocketRequired, kind)
	}
	if (binding.Enabled || binding.StrategyRef != "") && !ValidZapretStrategyRef(binding.StrategyRef) {
		return ErrZapretStrategyRef
	}
	return nil
}
