package componenthost

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/MalenkiySolovey/solovey-host/componenthost/manifest"
)

var componentMutationMu sync.Mutex

// Manager runs the existing local lifecycle commands against trusted ports.
// The caller authorizes each command; installed metadata, enabled settings
// and the supervisor remain with their existing writers.
type Manager struct {
	Snapshot     func() (CatalogSnapshot, error)
	SetInstalled func(id string, installed bool) error
	Removal      RemovalAdmission
	Reconcile    func(context.Context) error
	Migrate      func(context.Context) error
	Timeout      time.Duration
}

func (m Manager) Inventory() (Inventory, error) {
	if m.Snapshot == nil {
		return Inventory{}, errors.New("component catalog authority is unavailable")
	}
	snapshot, err := m.Snapshot()
	if err != nil {
		return Inventory{}, err
	}
	return snapshot.Inventory()
}

func (m Manager) Install(ctx context.Context, id string) (ComponentStatus, error) {
	if !componentMutationMu.TryLock() {
		return ComponentStatus{}, errors.New("component mutation is busy")
	}
	defer componentMutationMu.Unlock()
	item, status, err := m.entry(ctx, id)
	if err != nil || status.Installed {
		return status, err
	}
	if !status.Installable {
		if status.UnavailableReason != "" {
			return ComponentStatus{}, fmt.Errorf("component %q cannot be installed: %s", id, status.UnavailableReason)
		}
		return ComponentStatus{}, fmt.Errorf("component %q cannot be installed in this runtime profile", id)
	}
	if m.SetInstalled == nil || m.Migrate == nil || m.Reconcile == nil {
		return ComponentStatus{}, errors.New("component install authority is unavailable")
	}
	if err := m.SetInstalled(id, true); err != nil {
		return ComponentStatus{}, err
	}
	if err := m.run(m.Migrate); err != nil {
		rollbackErr := m.removeMetadata(ctx, item, false)
		return ComponentStatus{}, errors.Join(err, wrapCompensationError("restore component install state", rollbackErr))
	}
	if err := m.run(m.Reconcile); err != nil {
		if restoreErr := m.removeMetadata(ctx, item, false); restoreErr != nil {
			return ComponentStatus{}, errors.Join(err, wrapCompensationError("retain component ownership after unsafe compensation", restoreErr))
		}
		return ComponentStatus{}, errors.Join(err, wrapCompensationError("reconcile restored component state", m.run(m.Reconcile)))
	}
	return m.status(id)
}

func (m Manager) Remove(ctx context.Context, id string) (ComponentStatus, error) {
	if !componentMutationMu.TryLock() {
		return ComponentStatus{}, errors.New("component mutation is busy")
	}
	defer componentMutationMu.Unlock()
	item, status, err := m.entry(ctx, id)
	if err != nil || !status.Installed {
		return status, err
	}
	if !status.Removable {
		if status.Enabled && !status.Locked {
			return ComponentStatus{}, errors.New("disable component before removal")
		}
		if status.LockedReason != "" {
			return ComponentStatus{}, fmt.Errorf("component %q cannot be removed: %s", id, status.LockedReason)
		}
		return ComponentStatus{}, fmt.Errorf("component %q cannot be removed", id)
	}
	if m.SetInstalled == nil || m.Reconcile == nil {
		return ComponentStatus{}, errors.New("component removal authority is unavailable")
	}
	if err := m.removeMetadata(ctx, item, true); err != nil {
		return ComponentStatus{}, err
	}
	if err := m.run(m.Reconcile); err != nil {
		if restoreErr := m.SetInstalled(id, true); restoreErr != nil {
			return ComponentStatus{}, errors.Join(err, wrapCompensationError("restore component install state", restoreErr))
		}
		return ComponentStatus{}, errors.Join(err, wrapCompensationError("reconcile restored component state", m.run(m.Reconcile)))
	}
	return m.status(id)
}

func (m Manager) SetEnabled(ctx context.Context, id string, enabled bool, write func(bool) error) (ComponentStatus, error) {
	if !componentMutationMu.TryLock() {
		return ComponentStatus{}, errors.New("component mutation is busy")
	}
	defer componentMutationMu.Unlock()
	_, status, err := m.entry(ctx, id)
	if err != nil {
		return ComponentStatus{}, err
	}
	if !status.AvailableInBinary || !status.Installed {
		return ComponentStatus{}, fmt.Errorf("component is not installed in this binary: %s", id)
	}
	if status.Locked && !enabled {
		return ComponentStatus{}, fmt.Errorf("component %q cannot be disabled: %s", id, status.LockedReason)
	}
	if write == nil || m.Reconcile == nil {
		return ComponentStatus{}, errors.New("component settings authority is unavailable")
	}
	if err := write(enabled); err != nil {
		return ComponentStatus{}, err
	}
	if err := m.run(m.Reconcile); err != nil {
		writeErr := write(status.Enabled)
		reconcileErr := m.run(m.Reconcile)
		return ComponentStatus{}, errors.Join(err, wrapCompensationError("restore component enabled state", writeErr), wrapCompensationError("reconcile restored component state", reconcileErr))
	}
	return m.status(id)
}

func (m Manager) removeMetadata(ctx context.Context, item manifest.Manifest, requireDisabled bool) error {
	return m.Removal.Commit(ctx, item, requireDisabled, m.Timeout, func() error {
		return m.SetInstalled(item.ID, false)
	})
}

func (m Manager) entry(ctx context.Context, id string) (manifest.Manifest, ComponentStatus, error) {
	if err := manifest.ValidateID(id); err != nil {
		return manifest.Manifest{}, ComponentStatus{}, err
	}
	if ctx == nil {
		return manifest.Manifest{}, ComponentStatus{}, errors.New("component operation context is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return manifest.Manifest{}, ComponentStatus{}, err
	}
	if m.Snapshot == nil {
		return manifest.Manifest{}, ComponentStatus{}, errors.New("component catalog authority is unavailable")
	}
	snapshot, err := m.Snapshot()
	if err != nil {
		return manifest.Manifest{}, ComponentStatus{}, err
	}
	return snapshot.entry(id)
}

func (m Manager) status(id string) (ComponentStatus, error) {
	_, status, err := m.entry(context.Background(), id)
	return status, err
}

func (m Manager) run(fn func(context.Context) error) error {
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	// An admitted migration/reconcile must finish or compensate even if the
	// authenticated caller disconnects after metadata has been committed.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return fn(ctx)
}

func wrapCompensationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
