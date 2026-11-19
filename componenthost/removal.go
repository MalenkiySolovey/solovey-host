package componenthost

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/MalenkiySolovey/solovey-host/componenthost/manifest"
	"github.com/MalenkiySolovey/solovey-host/componenthost/resources"
)

const RemovalImpactV1 = "solovey.component-removal-impact/v1"

// RemovalImpact contains only facts required for metadata removal. The
// database, external authority and supervisor behind those facts stay with
// their existing owners.
type RemovalImpact struct {
	Schema            string
	OwnerID           string
	Installed         bool
	Available         bool
	Enabled           bool
	Blockers          []string
	Resources         []RemovalResource
	LeaseCount        int
	OperationCount    int
	ExternalAuthority string
}

type RemovalResource struct {
	Owner    string
	Terminal string
}

type RemovalAdmission struct {
	Preview   func(context.Context, string) (RemovalImpact, error)
	Resources func(context.Context, string) (RemovalImpact, error)
	Inventory func(context.Context) resources.ResourceSnapshot
	Serialize func(func() error) error
	Admit     func(context.Context, string, func() error) error
}

var errRemovalImpact = errors.New("component removal impact is unresolved")

// One read may outlive cancellation if an owner does not cooperate. Retain
// its slot until it returns; later requests cannot start unbounded work.
var removalReads = make(chan struct{}, 1)

// Commit checks the read-only footprint before entering the existing operation
// and lifecycle serializers, then rechecks it before calling the single
// installed-metadata writer. It never drops data or authorizes Panel requests.
// Callers must authorize the requested operation before calling Commit.
func (a RemovalAdmission) Commit(parent context.Context, item manifest.Manifest, requireDisabled bool, limit time.Duration, write func() error) error {
	if parent == nil || manifest.ValidateID(item.ID) != nil || a.Preview == nil || a.Resources == nil || a.Inventory == nil || a.Serialize == nil || a.Admit == nil || write == nil {
		return errRemovalImpact
	}
	timeout := 3 * time.Second
	if limit > 0 && limit < timeout {
		timeout = limit
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	id := item.ID
	if err := boundedRemovalRead(ctx, func() error {
		preview, err := a.Preview(ctx, id)
		if err != nil {
			return errRemovalImpact
		}
		if err := validateRemovalImpact(item, preview, requireDisabled, true); err != nil {
			return err
		}
		return validateRemovalInventory(id, a.Inventory(ctx))
	}); err != nil {
		return err
	}
	return a.Serialize(func() error {
		return a.Admit(ctx, id, func() error {
			preview, err := a.Resources(ctx, id)
			if err != nil {
				return errRemovalImpact
			}
			if err := validateRemovalImpact(item, preview, requireDisabled, false); err != nil {
				return err
			}
			if ctx.Err() != nil {
				return errRemovalImpact
			}
			return write()
		})
	})
}

func boundedRemovalRead(ctx context.Context, read func() error) error {
	if ctx.Err() != nil {
		return errRemovalImpact
	}
	select {
	case removalReads <- struct{}{}:
	default:
		return errors.New("component removal observation is busy")
	}
	result := make(chan error, 1)
	go func() {
		err := errRemovalImpact
		defer func() {
			if recover() != nil {
				err = errRemovalImpact
			}
			<-removalReads
			result <- err
		}()
		err = read()
	}()
	select {
	case err := <-result:
		if ctx.Err() != nil {
			return errRemovalImpact
		}
		return err
	case <-ctx.Done():
		return errRemovalImpact
	}
}

func validateRemovalImpact(item manifest.Manifest, preview RemovalImpact, disabled, external bool) error {
	if preview.Schema != RemovalImpactV1 || preview.OwnerID != item.ID || !preview.Installed || !preview.Available {
		return errRemovalImpact
	}
	if disabled && preview.Enabled {
		return errors.New("disable component before removal")
	}
	for _, blocker := range preview.Blockers {
		if blocker == "owner_is_enabled" && !disabled {
			continue
		}
		if blocker == "owner_drop_contract_unavailable" && !item.Database.Declared() && len(item.Privileged.Entries) == 0 {
			continue
		}
		return errRemovalImpact
	}
	if len(preview.Resources) > 416 || preview.LeaseCount != 0 || preview.OperationCount != 0 {
		return errRemovalImpact
	}
	for _, resource := range preview.Resources {
		if resource.Owner != item.ID || resource.Terminal != "ABSENT" {
			return errors.New("component retains owned data; use the separate owner Drop Data workflow")
		}
	}
	if external {
		critical := len(item.Privileged.Entries) != 0 || item.Database.PortabilityClass == manifest.PortabilityHostBound
		for _, file := range item.Database.Files {
			critical = critical || file.Portability == manifest.PortabilityHostBound
		}
		if preview.ExternalAuthority != "VERIFIED_SAFE" && (critical || preview.ExternalAuthority != "NOT_REQUIRED") {
			return errRemovalImpact
		}
	}
	return nil
}

func validateRemovalInventory(id string, snapshot resources.ResourceSnapshot) error {
	if len(snapshot.Errors) != 0 || len(snapshot.Warnings) != 0 || len(snapshot.Resources) > resources.MaxResourceFacts {
		return errRemovalImpact
	}
	for _, resource := range snapshot.Resources {
		if resource.Owner == "" || resource.ID == "" || resource.Source == "" {
			return errRemovalImpact
		}
		if resource.Owner == id || resource.ComponentID == id || strings.HasPrefix(resource.Capabilities.FallbackTargetID, id+":") {
			return errors.New("component retains a resource or external reference")
		}
	}
	return nil
}
