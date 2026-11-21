package outbounds

import (
	"errors"
	"regexp"
)

var (
	ErrZapretDirectRequired = errors.New("Zapret requires a Direct outbound without a detour")
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
	if !zapretOutboundID.MatchString(outboundRef) || binding.OutboundRef != outboundRef || kind != "direct" || binding.Enabled && detour != "" {
		return ErrZapretDirectRequired
	}
	if (binding.Enabled || binding.StrategyRef != "") && !ValidZapretStrategyRef(binding.StrategyRef) {
		return ErrZapretStrategyRef
	}
	return nil
}
