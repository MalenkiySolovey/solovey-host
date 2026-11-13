package durable

import (
	"github.com/MalenkiySolovey/solovey-host/hostref"
	"testing"
)

func TestScopeRequiresExplicitOwnerStorageAndVersions(t *testing.T) {
	good := Scope{Host: hostref.Local(), OwnerID: "fallback-html", Storage: Candidate, SchemaVersion: "1", MigrationVersion: "1"}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []Scope{good, good, good, good, good, good}
	cases[0].Host = hostref.Ref{}
	cases[1].Host = hostref.Ref{Kind: "NODE", ID: "local"}
	cases[2].OwnerID = ""
	cases[3].Storage = ""
	cases[4].SchemaVersion = ""
	cases[5].MigrationVersion = ""
	for _, s := range cases {
		if s.Validate() == nil {
			t.Fatalf("scope accepted: %+v", s)
		}
	}
	good.Storage = Live
	if good.Validate() != nil {
		t.Fatal("live owner scope rejected")
	}
}
