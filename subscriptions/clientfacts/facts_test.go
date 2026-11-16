package clientfacts

import (
	"regexp"
	"testing"
)

func TestCredentialFieldsRemainFixedAndCallerCannotMutateSchema(t *testing.T) {
	fieldPattern := regexp.MustCompile(`^[a-z0-9_]+$`)
	for _, fact := range Facts() {
		if field, ok := UserField(fact.Type); !ok || field != fact.UserField || !fieldPattern.MatchString(field) {
			t.Fatalf("unsafe credential field for %q", fact.Type)
		}
	}
	fields := Facts()
	fields[0].UserField = "vmess') FROM clients; --"
	if field, _ := UserField(fields[0].Type); field == fields[0].UserField {
		t.Fatal("caller changed authoritative schema")
	}
	if _, ok := UserField("vmess'); DROP TABLE clients; --"); ok {
		t.Fatal("untrusted type became a credential field")
	}
	if field, ok := UserFieldForMethod("shadowsocks", "2022-blake3-aes-128-gcm"); !ok || field != "shadowsocks16" {
		t.Fatal("legacy 16-byte credential schema changed")
	}
}

func TestUDPProjectionRetainsNetworkAndSOCKSVersionRestrictions(t *testing.T) {
	for _, options := range []map[string]any{{"version": "4"}, {"version": "4a"}, {"network": "tcp"}, {"network": []any{"tcp"}}, {"network": true}} {
		if SupportsUDP("socks", options) {
			t.Fatal("unsupported UDP delivery inferred")
		}
	}
	if SupportsUDP("http", nil) || SupportsUDP("unknown", nil) {
		t.Fatal("UDP inferred for unknown or TCP schema")
	}
	for _, options := range []map[string]any{nil, {"network": "udp"}, {"network": []string{"tcp", "udp"}}, {"network": []any{"tcp", "udp"}}} {
		if !SupportsUDP("socks", options) {
			t.Fatal("accepted UDP capability lost")
		}
	}
}
