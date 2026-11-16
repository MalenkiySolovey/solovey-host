// Package parseport defines the existing shared source parser projection.
// An implementation must retain current format detection and conversion rules;
// this port creates no parser registration, network worker or publisher.
package parseport

import (
	"context"
	"github.com/MalenkiySolovey/solovey-host/subscriptions/canonical"
	"github.com/MalenkiySolovey/solovey-host/subscriptions/conversion"
)

type Options struct {
	// Format is one existing canonical source representation.
	Format           string
	GroupAdaptation  string
	ConversionPolicy conversion.Policy
}

type Result struct {
	Outbounds []map[string]any
	Canonical canonical.Snapshot
}

// Parser is implemented by the existing shared subscription parsing owner.
// External source coordination supplies bounded content and an operation context.
type Parser interface {
	Parse(context.Context, []byte, Options) (Result, error)
}
