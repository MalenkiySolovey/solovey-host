# Solovey Host

`solovey-host` provides reusable Go libraries for host-local facts and component integration. Solovey Panel and independent consumers use the same versioned source while retaining their own observation, authorization and storage adapters.

## Packages

| Package | Responsibility |
| --- | --- |
| `hostref` | Explicit Local Host identity. |
| `networkidentity` | Address classification, evidence freshness, bounded snapshots and deterministic revisions. |
| `componenthost` and `componenthost/manifest` | Validated component descriptors, declared capabilities, neutral lifecycle contracts and the shared bounded supervisor loop. |
| `componenthost/durable` | Owner-scoped migration, restore, inspection and data-drop capabilities; no storage handles. |
| `componenthost/resources` | Local configured identity, listen normalization and deterministic revisions; observations remain unknown. |
| `componenthost/health` | Revision-bound evidence values and freshness validation; the caller retains its Health registry. |
| `componenthost/publicsurface` | Managed public request context and classified observations; the caller retains routing and delivery. |
| `coreruntime` | Bounded runtime group selection through an admitted owner lease, with observation and before-image recovery. |
| `scheduler` | Cancellation and drain behavior for existing scheduled jobs, using an injected operation lease. |
| `secretref` and `operationref` | Scoped owner-use references and masked operation correlation. |
| `auditrecord` and `observation` | Existing neutral audit events and sample/history projections. |
| `componentkit/telegram` | Stateless Bot API request, response and retry helpers for independent callers. |
| `redact` and `httpheader` | Shared secret redaction and bounded text sanitization. |

Network facts retain their source, authority, observation time and expiry. Unknown and stale evidence remains explicit. Address ranges and interface names do not establish LAN/WAN roles or public reachability. Snapshots describe the current process network namespace.

## Usage

```go
import (
    "time"

    "github.com/MalenkiySolovey/solovey-host/networkidentity"
)

snapshot := networkidentity.Empty(time.Now())
// The caller supplies observations from its existing trusted adapters.
snapshot = networkidentity.Finalize(snapshot, time.Now())
```

The `solovey-ui/host-network-identity/v1` JSON identity is preserved for compatibility. This library supplies no observation loop, network listener or remote management service. Panel authentication, databases, routing policy, component business logic and product releases remain with their respective owners.

## Development

Use Go 1.26.10 with `GOWORK=off`:

```text
go test -race -count=1 ./...
go vet ./...
```

Tests cover deterministic JSON, snapshot bounds, evidence expiry and dependency boundaries. Consumers should pin an immutable module version and retain normal Go checksums.

`componenthost.Reconcile(ctx, running, active, recorder)` uses the existing
supervisor algorithm: stop inactive instances in reverse order, start newly
admitted instances in supplied order, keep existing instances on repeated
reconcile, and retain failed Stops for retry. `Stop` drains all instances through
the same implementation. Each `Entry` contains its ComponentID, actual
`Lifecycle` and admitted Local Host `Ports`. The caller keeps its existing
serializer and uses the returned instance set; no second ComponentHost,
registry or scheduler is created. For example:

```go
running, err = componenthost.Reconcile(ctx, running, installedEnabled, recorder)
// During shutdown, keep any instances whose drain has not completed:
running, err = componenthost.Stop(ctx, running, recorder)
```

Composition owns installed/enabled/capability admission and supplies no entry
for an absent or disabled Component. A Component must undo partial startup if
Start fails, and Stop must drain owned activity before returning nil. The
recorder reports actual lifecycle calls; successful Start does not establish
Health/readiness. Failed Stop retains the instance even after disable/removal
intent. Ordinary disable calls neither DropData nor external-rule cleanup.
Owner-specific data removal and privileged enforcement remain separate.

Component composition retains installed/enabled admission, authorization and storage. The lifecycle contracts expose only capabilities already used by their consumers. The scheduled-job helper does not construct a second scheduler; its caller supplies the existing operation admission and retains registration/stop ownership. SecretRef provides masked metadata and owner-use validation without storing or revealing secrets.

Bot API callers supply their approved HTTP client and operation context. The helper creates no background client, bot registration or polling loop. Proxy selection and validation remain with caller adapters. The only external Go dependency is the existing `github.com/robfig/cron/v3` v3.0.1 contract.

## Runtime selection

`coreruntime.Select(ctx, intent, executor)` accepts an explicit Local Host, an
`outbound-group` resource owned by `core-runtime`, provider identity/revision,
expected runtime generation and member. The existing owner supplies
`SelectionExecutor.WithSelectionLease`; its callback receives a scoped
`SelectionLease` with current admission, a bounded selection reader and a
member selector. The owner retains authentication, authorization, mutual
exclusion, lifecycle fencing and its concrete provider API. An intent is not
an authorization grant. Adapters are trusted in-process code, not a sandbox.

For example, an already authorized owner constructs the intent from its local
resource and observed provider, then supplies its existing lease adapter:

```go
intent := coreruntime.SelectionIntent{
    Host: localHost, Resource: groupResource, Provider: selectedProvider,
    ExpectedRevision: runtimeGeneration, Member: requestedMember,
}
result, err := coreruntime.Select(ctx, intent, ownerExecutor)
```

The generation is the existing runtime revision; selection does not mutate
Desired configuration or create a new revision store. Only Official (`sing-box`)
and Shtorm Extended (`shtorm-extended`) identities are admitted. The provider
and AVAILABLE capability must match the owner lease and fresh observations.
Supporting an interface alone does not establish capability availability.

Success requires observation of the requested selection, including idempotent
retries. Failed dispatch, verification or cancellation after dispatch triggers
bounded recovery under the original lease. Recovery removes the caller's
cancellation deadline only for that bounded synchronous attempt, and must
observe the before-image. A changed provider, resource or generation prevents
recovery writes. Failed recovery returns AMBIGUOUS/UNKNOWN; verified recovery
still returns an error for the failed operation. Evidence proves runtime
selection only, not data-plane traffic or durable configuration. No generic
command, privileged executor, engine dependency or provider switch is exposed.

The API is additive to the v0.2 package contracts. Existing component modules
using v0.2 lifecycle ports remain source compatible. Consumers should qualify
and pin the same immutable v0.3 module for shared execution. Tests cover both
provider identities, foreign scope, stale generations, unavailable capabilities,
concurrency, cancellation, verification, ambiguous apply and failed recovery.

## License and provenance

The library retains the original [GNU GPLv3 license](LICENSE). Source attribution, original paths and commit history are recorded in `SOURCE_PROVENANCE.json`. See [NOTICE](NOTICE) for attribution.
