package networkidentity

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestSnapshotCanonicalFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshots.json")
	if err != nil {
		t.Fatal(err)
	}
	var bundle struct {
		Schema string `json:"schema"`
		Cases  []struct {
			Name     string          `json:"name"`
			Now      int64           `json:"now"`
			Input    Snapshot        `json:"input"`
			Overflow string          `json:"overflow"`
			Expected json.RawMessage `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Schema != "host-network-identity-fixtures/v1" || len(bundle.Cases) != 27 {
		t.Fatal("unexpected snapshot fixtures")
	}
	seen := map[string]bool{}
	for _, test := range bundle.Cases {
		t.Run(test.Name, func(t *testing.T) {
			if seen[test.Name] {
				t.Fatal("duplicate fixture")
			}
			seen[test.Name] = true
			value := test.Input
			switch test.Overflow {
			case "":
			case "interfaces":
				value.Interfaces = make([]Interface, MaxFacts+1)
			case "interface-addresses":
				value.Interfaces[0].Addresses = make([]Address, MaxFacts+1)
			case "listen":
				value.Listen.Values = make([]EndpointInput, MaxFacts+1)
			case "advertised":
				value.Advertised.Values = make([]EndpointInput, MaxFacts+1)
			case "private":
				value.PrivateAddresses.Values = make([]AddressRef, MaxFacts+1)
			case "lan":
				value.LAN.Values = make([]AddressRef, MaxFacts+1)
			case "wan":
				value.WAN.Values = make([]AddressRef, MaxFacts+1)
			case "routes":
				value.RouteSources.Values = make([]AddressRef, MaxFacts+1)
			case "egress":
				value.ExternalEgress.Values = make([]AddressRef, MaxFacts+1)
			default:
				t.Fatal("unknown fixture collection")
			}
			at := time.Unix(test.Now, 0)
			result := Finalize(value, at)
			got, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var expected bytes.Buffer
			if err := json.Compact(&expected, test.Expected); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, expected.Bytes()) {
				t.Fatalf("canonical snapshot differs from accepted behavior:\ngot %s\nwant %s", got, expected.Bytes())
			}
			if Finalize(result, at).Revision != result.Revision {
				t.Fatal("finalization changed the same observation revision")
			}
		})
	}
}
