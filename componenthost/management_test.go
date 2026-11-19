package componenthost

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/MalenkiySolovey/solovey-host/componenthost/manifest"
	"github.com/MalenkiySolovey/solovey-host/componenthost/resources"
)

const managedID = "test-component"

func managementFixture(t *testing.T, installed, enabled bool) (Manager, *bool, *bool, *[]string) {
	t.Helper()
	var events []string
	snapshot := CatalogSnapshot{
		BinaryProfile: "full",
		Manifests:     []manifest.Manifest{{ID: managedID, Version: "1", Delivery: manifest.DeliveryInProcess}},
		Compatible:    func(string) bool { return true },
		Observe:       func(string, bool, bool) LifecycleObservation { return LifecycleObservation{Phase: "NOT_OBSERVED"} },
	}
	read := func(_ context.Context, id string) (RemovalImpact, error) {
		return RemovalImpact{Schema: RemovalImpactV1, OwnerID: id, Installed: installed,
			Available: true, Enabled: enabled, ExternalAuthority: "NOT_REQUIRED"}, nil
	}
	return Manager{
		Snapshot: func() (CatalogSnapshot, error) {
			s := snapshot
			if installed {
				s.Installed = []InstalledOwner{{ID: managedID, Enabled: enabled}}
			}
			return s, nil
		},
		SetInstalled: func(id string, value bool) error {
			if id != managedID {
				t.Fatal("metadata write changed the target", id)
			}
			event := "remove"
			if value {
				event = "install"
			}
			events = append(events, event)
			installed = value
			return nil
		},
		Reconcile: func(context.Context) error { events = append(events, "reconcile"); return nil },
		Migrate:   func(context.Context) error { events = append(events, "migrate"); return nil },
		Removal: RemovalAdmission{
			Preview: read, Resources: read,
			Inventory: func(context.Context) resources.ResourceSnapshot { return resources.ResourceSnapshot{} },
			Serialize: func(commit func() error) error { events = append(events, "serialize"); return commit() },
			Admit: func(_ context.Context, _ string, commit func() error) error {
				events = append(events, "admit")
				return commit()
			},
		},
	}, &installed, &enabled, &events
}

func TestManagementPreservesMetadataOnRemovalDenial(t *testing.T) {
	for _, variant := range []string{"enabled", "missing port", "authority unknown", "inventory unavailable", "external reference", "footprint changed", "panic", "canceled"} {
		t.Run(variant, func(t *testing.T) {
			m, installed, enabled, events := managementFixture(t, true, false)
			read := m.Removal.Preview
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch variant {
			case "enabled":
				*enabled = true
			case "missing port":
				m.Removal.Admit = nil
			case "authority unknown":
				m.Removal.Preview = func(ctx context.Context, id string) (RemovalImpact, error) {
					p, err := read(ctx, id)
					p.ExternalAuthority = "UNKNOWN"
					return p, err
				}
			case "inventory unavailable":
				m.Removal.Inventory = func(context.Context) resources.ResourceSnapshot {
					return resources.ResourceSnapshot{Errors: []resources.ResourceError{{Owner: "foreign", Message: "private"}}}
				}
			case "external reference":
				m.Removal.Inventory = func(context.Context) resources.ResourceSnapshot {
					return resources.ResourceSnapshot{Resources: []resources.ProtectableResource{{ID: "foreign:socket", Owner: "foreign", Source: "settings", Capabilities: resources.ProtectableResourceCapabilities{FallbackTargetID: managedID + ":target"}}}}
				}
			case "footprint changed":
				m.Removal.Resources = func(ctx context.Context, id string) (RemovalImpact, error) {
					p, err := read(ctx, id)
					p.Resources = []RemovalResource{{Owner: id, Terminal: "PRESENT"}}
					return p, err
				}
			case "panic":
				m.Removal.Preview = func(context.Context, string) (RemovalImpact, error) { panic("private-owner-secret") }
			case "canceled":
				cancel()
			}
			if _, err := m.Remove(ctx, managedID); err == nil || err.Error() == "private-owner-secret" || !*installed {
				t.Fatalf("removal was not fail-closed: installed=%v err=%v", *installed, err)
			}
			for _, effect := range *events {
				if effect != "serialize" && effect != "admit" {
					t.Fatal("unexpected mutation after denial", *events)
				}
			}
		})
	}
}

func TestManagementUsesOneWriterAndCompensatesReconcileFailure(t *testing.T) {
	for _, initial := range []bool{false, true} {
		m, installed, _, events := managementFixture(t, initial, false)
		cause := errors.New("injected lifecycle failure")
		reconciles := 0
		m.Reconcile = func(context.Context) error {
			*events = append(*events, "reconcile")
			reconciles++
			if reconciles == 1 {
				return cause
			}
			return nil
		}
		var err error
		want := []string{"install", "migrate", "reconcile", "serialize", "admit", "remove", "reconcile"}
		if initial {
			_, err = m.Remove(t.Context(), managedID)
			want = []string{"serialize", "admit", "remove", "reconcile", "install", "reconcile"}
		} else {
			_, err = m.Install(t.Context(), managedID)
		}
		if !errors.Is(err, cause) || *installed != initial || !reflect.DeepEqual(*events, want) {
			t.Fatalf("metadata compensation drift: installed=%v, error=%v, effects=%v", *installed, err, *events)
		}
	}
}

func TestManagementCannotAdmitLateOrUnboundedRemoval(t *testing.T) {
	m, installed, _, _ := managementFixture(t, true, false)
	read := m.Removal.Preview
	started, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	m.Removal.Preview = func(ctx context.Context, id string) (RemovalImpact, error) {
		close(started)
		<-release
		defer close(returned)
		return read(ctx, id)
	}
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan error, 1)
	go func() { _, err := m.Remove(ctx, managedID); finished <- err }()
	<-started
	cancel()
	err := <-finished
	if _, secondErr := m.Remove(t.Context(), managedID); secondErr == nil || secondErr.Error() != "component removal observation is busy" {
		close(release)
		t.Fatalf("repeated removal did not respect read bound: %v", secondErr)
	}
	close(release)
	<-returned
	for deadline := time.Now().Add(time.Second); len(removalReads) > 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if err == nil || !*installed || len(removalReads) != 0 {
		t.Fatalf("late or unbounded removal: installed=%v error=%v", *installed, err)
	}
}

func TestCatalogKeepsMissingOwnersNonActionable(t *testing.T) {
	s := CatalogSnapshot{BinaryProfile: "core", Compatible: func(string) bool { return true },
		Manifests: []manifest.Manifest{{ID: managedID, Version: "1", Delivery: manifest.DeliveryInProcess}},
		Installed: []InstalledOwner{{ID: "missing-owner", Delivery: manifest.DeliveryInProcess, Enabled: true}},
		Observe: func(_ string, _, available bool) LifecycleObservation {
			if !available {
				return LifecycleObservation{Phase: "BINARY_ABSENT"}
			}
			return LifecycleObservation{Phase: "STOP_FAILED", InstanceRetention: "RETAINED"}
		}}
	inventory, err := s.Inventory()
	if err != nil || len(inventory.Unavailable) != 1 || len(inventory.Available) != 1 {
		t.Fatalf("unexpected catalog: %+v %v", inventory, err)
	}
	missing, available := inventory.Unavailable[0], inventory.Available[0]
	if !missing.Installed || missing.AvailableInBinary || missing.Compatible || missing.Installable || missing.Removable || missing.Active || missing.Enabled || missing.Version != "" || missing.ObservedLifecycle.Phase != "BINARY_ABSENT" {
		t.Fatalf("absent owner acquired authority: %+v", missing)
	}
	if available.Installed || !available.Installable || available.ObservedLifecycle.InstanceRetention != "RETAINED" {
		t.Fatalf("catalog hid the running supervisor's retained instance: %+v", available)
	}
}
