package outbounds

import (
	"errors"
	"testing"
)

func TestZapretBindingRequiresAnAdmissibleSocketOwner(t *testing.T) {
	for _, test := range []struct {
		binding            ZapretBinding
		outbound, typ, via string
		want               error
	}{
		{ZapretBinding{"a", false, "saved"}, "a", "direct", "proxy", ErrZapretDirectRequired},
		{ZapretBinding{"a", true, "saved"}, "a", "direct", "", nil},
		{ZapretBinding{"a", true, "saved"}, "a", "http", "", nil},
		{ZapretBinding{"a", false, "saved"}, "a", "hysteria2", "", nil},
		{ZapretBinding{"a", true, "saved"}, "a", "hysteria2", "", nil},
		{ZapretBinding{"a", true, ""}, "a", "direct", "", ErrZapretStrategyRef},
		{ZapretBinding{"a", true, "saved"}, "b", "direct", "", ErrZapretDirectRequired},
		{ZapretBinding{"a", true, "saved"}, "a", "direct", "proxy", ErrZapretDirectRequired},
		{ZapretBinding{"a", true, "saved"}, "a", "selector", "", ErrZapretDirectRequired},
		{ZapretBinding{"a", true, "saved"}, "a", "http", "direct", ErrZapretSocketRequired},
		{ZapretBinding{"a", false, "saved"}, "a", "http", "direct", nil},
		{ZapretBinding{"a", true, "saved"}, "a", "hysteria2", "group", ErrZapretSocketRequired},
		{ZapretBinding{"a", true, "saved"}, "a", "wireguard", "", ErrZapretSocketRequired},
	} {
		if err := test.binding.Validate(test.outbound, test.typ, test.via); !errors.Is(err, test.want) {
			t.Errorf("binding=%+v outbound=%s type=%s detour=%s: %v", test.binding, test.outbound, test.typ, test.via, err)
		}
	}
}
