package intent

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

func TestZapretPacketPathIntentCarriesCompactPortRanges(t *testing.T) {
	value := zapretIntentFixture()
	value.TCPPorts = nil
	value.TCPPortRanges = []PortInterval{{80, 80}, {1000, 65000}}
	canonical := CanonicalZapretPacketPathIntent(value)
	if err := canonical.Validate(); err != nil {
		t.Fatal(err)
	}
	value.TCPPortRanges[1].Last = 1001
	if canonical.TCPPortRanges[1].Last != 65000 {
		t.Fatal("canonical intent retained caller-owned ranges")
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ZapretPacketPathIntent
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.Validate() != nil ||
		len(decoded.TCPPorts) != 0 || !slices.Equal(decoded.TCPPortRanges, canonical.TCPPortRanges) {
		t.Fatalf("compact scope lost during transfer: %+v, %v", decoded, err)
	}
	for _, edit := range []func(*ZapretPacketPathIntent){
		func(value *ZapretPacketPathIntent) { value.TCPPorts = []uint16{443} },
		func(value *ZapretPacketPathIntent) { value.TCPPortRanges = []PortInterval{{80, 0}} },
		func(value *ZapretPacketPathIntent) { value.TCPPortRanges = []PortInterval{{0, 500}} },
		func(value *ZapretPacketPathIntent) { value.TCPPortRanges = []PortInterval{{80, 500}, {499, 700}} },
		func(value *ZapretPacketPathIntent) { value.TCPPortRanges = []PortInterval{{80, 500}, {501, 700}} },
		func(value *ZapretPacketPathIntent) { value.TCPPortRanges = []PortInterval{{90, 90}, {80, 80}} },
	} {
		candidate := canonical
		edit(&candidate)
		if candidate.Validate() == nil {
			t.Fatalf("invalid range scope accepted: %+v", candidate)
		}
	}
}

func TestCanonicalPortIntervalsPreservesUnionAndEndOfDomain(t *testing.T) {
	input := []PortInterval{{64000, 65535}, {1000, 1099}, {1018, 1042}, {200, 203}}
	got, err := CanonicalPortIntervals([]uint16{65535, 80, 1100, 199}, input...)
	want := []PortInterval{{80, 80}, {199, 203}, {1000, 1100}, {64000, 65535}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("interval union = %+v, %v; want %+v", got, err, want)
	}
	if input[0] != (PortInterval{64000, 65535}) {
		t.Fatal("interval canonicalization changed the caller's input")
	}
}

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
		Protocols:        []PacketProtocol{PacketProtocolUDP, PacketProtocolTCP},
		TCPPorts:         []uint16{443, 80},
		UDPPorts:         []uint16{443},
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
		{"duplicate protocol", func(value *ZapretPacketPathIntent) { value.Protocols[0] = value.Protocols[1] }},
		{"unbounded TCP scope", func(value *ZapretPacketPathIntent) { value.TCPPorts = nil }},
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
