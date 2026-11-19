package componenthost

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MalenkiySolovey/solovey-host/componenthost/manifest"
)

type Group string

const (
	GroupInstalled   Group = "installed"
	GroupAvailable   Group = "available"
	GroupUnavailable Group = "unavailable"
)

// LifecycleObservation is the result of a supervisor call, not readiness or
// proof that its external resources have been released.
type LifecycleObservation struct {
	Phase             string     `json:"phase"`
	InstanceRetention string     `json:"instanceRetention"`
	Source            string     `json:"source"`
	ObservedAt        *time.Time `json:"observedAt,omitempty"`
	ReasonCode        string     `json:"reasonCode"`
}

type Inventory struct {
	BinaryProfile string
	Components    []ComponentStatus
	Installed     []ComponentStatus
	Available     []ComponentStatus
	Unavailable   []ComponentStatus
}

type ComponentStatus struct {
	ID, Name, Version, LatestVersion, Since, RequiredPanel string
	Delivery                                               manifest.Delivery
	DefaultEnabled                                         bool
	TokenScopes                                            []string
	AvailableInBinary, Compatible, Locked                  bool
	LockedReason                                           string
	Installable, Removable, Installed, Enabled, Active     bool
	ObservedLifecycle                                      LifecycleObservation
	Group                                                  Group
	UnavailableReason                                      string
}

type InstalledOwner struct {
	ID       string
	Delivery manifest.Delivery
	Enabled  bool
}

// CatalogSnapshot comes from the existing registry and metadata reader. The
// caller supplies version and packaging policy; neither grants write access.
type CatalogSnapshot struct {
	BinaryProfile     string
	Manifests         []manifest.Manifest
	Installed         []InstalledOwner
	Compatible        func(required string) bool
	Observe           func(id string, installed, availableInBinary bool) LifecycleObservation
	PackageLockReason string
	LockedID          string
	LockedReason      string
}

// Inventory reports what is in this binary, including missing installed
// owners. It does not discover releases or qualify artifact installs.
func (s CatalogSnapshot) Inventory() (Inventory, error) {
	if s.Compatible == nil || s.Observe == nil {
		return Inventory{}, errors.New("component catalog authority is unavailable")
	}
	result := Inventory{BinaryProfile: s.BinaryProfile, Unavailable: []ComponentStatus{}}
	installed := make(map[string]InstalledOwner, len(s.Installed))
	for _, item := range s.Installed {
		installed[item.ID] = item
	}
	for _, item := range s.Manifests {
		owner, hasOwner := installed[item.ID]
		if hasOwner && owner.Delivery != "" && owner.Delivery != item.Delivery {
			return Inventory{}, fmt.Errorf("installed component %q delivery does not match this binary", item.ID)
		}
		delete(installed, item.ID)
		status := s.status(item, hasOwner, hasOwner && owner.Enabled)
		result.Components = append(result.Components, status)
		if hasOwner {
			result.Installed = append(result.Installed, status)
		} else {
			result.Available = append(result.Available, status)
		}
	}
	for _, owner := range installed {
		status := ComponentStatus{ID: owner.ID, Name: owner.ID, Delivery: owner.Delivery, Installed: true,
			Group: GroupUnavailable, UnavailableReason: "installed owner is absent from this binary",
			ObservedLifecycle: s.Observe(owner.ID, true, false)}
		result.Components = append(result.Components, status)
		result.Unavailable = append(result.Unavailable, status)
	}
	for _, group := range [][]ComponentStatus{result.Components, result.Installed, result.Available, result.Unavailable} {
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
	}
	return result, nil
}

func (s CatalogSnapshot) status(item manifest.Manifest, installed, enabled bool) ComponentStatus {
	required := strings.TrimSpace(item.Since)
	compatible := required == "" || s.Compatible(required)
	locked := item.ID == s.LockedID
	lockedReason, unavailableReason := "", ""
	if !compatible {
		unavailableReason = fmt.Sprintf("requires panel %s or newer", required)
	}
	if locked {
		lockedReason = s.LockedReason
	} else if s.PackageLockReason != "" {
		lockedReason = s.PackageLockReason
		if !installed {
			unavailableReason = lockedReason
		}
	}
	group := GroupAvailable
	if installed {
		group = GroupInstalled
	}
	mutable := !locked && s.PackageLockReason == ""
	return ComponentStatus{
		ID: item.ID, Name: item.Name, Version: item.Version, LatestVersion: item.Version, Since: item.Since,
		RequiredPanel: required, Delivery: item.Delivery, DefaultEnabled: item.DefaultEnabled,
		TokenScopes: append([]string(nil), item.TokenScopes...), AvailableInBinary: true, Compatible: compatible,
		Locked: locked, LockedReason: lockedReason, Installable: !installed && compatible && mutable,
		Removable: installed && !enabled && mutable, Installed: installed, Enabled: enabled, Active: installed && enabled,
		ObservedLifecycle: s.Observe(item.ID, installed, true), Group: group, UnavailableReason: unavailableReason,
	}
}

func (s CatalogSnapshot) entry(id string) (manifest.Manifest, ComponentStatus, error) {
	inventory, err := s.Inventory()
	if err != nil {
		return manifest.Manifest{}, ComponentStatus{}, err
	}
	for _, status := range inventory.Components {
		if status.ID != id {
			continue
		}
		for _, item := range s.Manifests {
			if item.ID == id {
				return item, status, nil
			}
		}
		return manifest.Manifest{}, status, nil
	}
	return manifest.Manifest{}, ComponentStatus{}, fmt.Errorf("component is not available in this binary: %s", id)
}
