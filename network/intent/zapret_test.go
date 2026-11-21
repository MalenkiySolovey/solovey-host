package intent

import (
	"strings"
	"testing"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

func zapretIntentFixture() ZapretPacketPathIntent {
	return ZapretPacketPathIntent{
		Schema:           ZapretPacketPathSchema,
		HostRef:          hostref.Local(),
		Owner:            ZapretOwner,
		ResourceRef:      "outbound:direct-a",
		OutboundRef:      "direct-a",
		StrategyRef:      "strategy-a",
		StrategyRevision: strings.Repeat("a", 64),
		DesiredRevision:  strings.Repeat("b", 64),
		InputRevision:    strings.Repeat("c", 64),
		Protocols:        []PacketProtocol{PacketProtocolUDP, PacketProtocolTCP, PacketProtocolTCP},
		Directions:       []PacketDirection{PacketDirectionInbound, PacketDirectionOutbound},
		NeedsReturn:      true,
		Purpose:          "direct-outbound-zapret",
	}
}

func TestZapretPacketPathIntentCanonicalizesAndValidates(t *testing.T) {
	value := zapretIntentFixture()
	canonical := CanonicalZapretPacketPathIntent(value)
	if err := canonical.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(canonical.Protocols) != 2 || canonical.Protocols[0] != PacketProtocolTCP || canonical.Protocols[1] != PacketProtocolUDP {
		t.Fatalf("protocols were not canonicalized: %#v", canonical.Protocols)
	}
	if len(canonical.Directions) != 2 || canonical.Directions[0] != PacketDirectionInbound || canonical.Directions[1] != PacketDirectionOutbound {
		t.Fatalf("directions were not canonicalized: %#v", canonical.Directions)
	}
	value.Protocols[0] = "foreign"
	if canonical.Protocols[0] != PacketProtocolTCP {
		t.Fatal("canonical intent retained caller-owned slice")
	}
}

func TestZapretPacketPathIntentRejectsMissingReturnAndForeignIdentity(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ZapretPacketPathIntent)
	}{
		{"foreign host", func(value *ZapretPacketPathIntent) { value.HostRef = hostref.Ref{Kind: "NODE", ID: "remote"} }},
		{"resource mismatch", func(value *ZapretPacketPathIntent) { value.ResourceRef = "outbound:direct-b" }},
		{"return direction without requirement", func(value *ZapretPacketPathIntent) { value.NeedsReturn = false }},
		{"missing strategy revision", func(value *ZapretPacketPathIntent) { value.StrategyRevision = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := zapretIntentFixture()
			test.edit(&value)
			if err := value.Validate(); err == nil {
				t.Fatal("invalid intent was accepted")
			}
		})
	}
}
