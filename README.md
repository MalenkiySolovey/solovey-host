# Solovey Host

`solovey-host` provides reusable Go libraries for host-local facts and component integration. Solovey Panel and independent consumers use the same versioned source while retaining their own observation, authorization and storage adapters.

## Packages

| Package | Responsibility |
| --- | --- |
| `hostref` | Explicit Local Host identity. |
| `networkidentity` | Address classification, evidence freshness, bounded snapshots and deterministic revisions. |
| `componenthost` and `componenthost/manifest` | Validated component descriptors, declared capabilities, neutral lifecycle contracts and the shared bounded supervisor loop. |
| `componenthost/durable` | Owner-scoped migration, restore, inspection and data-drop capabilities; no storage handles. |
| `componenthost/resources` | Protected resource and endpoint values, exact Fronting/local-proxy/interception references, transport facts and bounded lease/CAS validation; registries and provider execution stay with their owners. |
| `componenthost/hostsurface` | Socket, process, service and listener-owner evidence values with exact identity and freshness validation. |
| `componenthost/deploymentidentity` | Supervisor-neutral expected application-owner value; installed proof loading stays with deployment adapters. |
| `componenthost/fallbacktargets` | Bounded target facts, exact references, provider requests and reservation CAS validation; registry and provider storage stay with their owners. |
| `network/intent` | Shared value-only network intent vocabulary; allocation, accepted plans and execution remain with Network Planner. |
| `network/nftcontract` | Existing bounded NFT adapter request/result values; path admission, managed-table validation, execution, observation and rollback remain with the platform owner. |
| `network/planning` | Existing coordinator input/error/ownership views and read interfaces for private immutable plan and candidate receipts; allocation, acceptance, comparison and execution remain with the coordinator. |
| `componenthost/health` | Revision-bound evidence and typed protocol/local-proxy/interception probe values with exact identity and freshness validation; the caller retains registration, replay/generation state and probe execution. |
| `componenthost/publicsurface` | Managed public request context and classified observations; the caller retains routing and delivery. |
| `outbounds` | Source binding preview, revision/capability admission and verified write receipts through the existing Outbounds owner. |
| `subscriptions/canonical` and `subscriptions/conversion` | Existing format-neutral snapshots, source observations, identity-preserving merge and checked client projections shared by source import and Publishing. |
| `subscriptions/clientfacts` and `subscriptions/protocoldefaults` | Portable protocol credential/delivery schema and existing transport defaults; runtime admission remains with the caller. |
| `subscriptions/parseport` | Finite format parser port implemented by the existing Backend parser; no parser, fetcher or publisher is registered by importing this module. |
| `coreruntime` | Bounded runtime group selection through an admitted owner lease, with observation and before-image recovery. |
| `coreruntime/inboundcontrol` | Typed inbound fallback snapshots and exact checkpoint operations; capability resolution, engine pins, database hooks and runtime execution stay with the Core owner. |
| `scheduler` | Cancellation and drain behavior for existing scheduled jobs, using an injected operation lease. |
| `secretref` and `operationref` | Scoped owner-use references, masked operation correlation and finite owner-controlled read views. |
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

Protected resources expose their configured tuple through
`resource.ConfiguredResource()`. The existing
`LocalConfiguredIdentity(ConfiguredResource)` and `identity.ValidFor(ConfiguredResource)`
APIs keep the same signatures. A configured identity establishes no observed
socket ownership. `ConfiguredListenIntents`, `PreservationEndpointKeys` and
management/recovery validators preserve conservative coverage and reject
incomplete or stale evidence. Callers retain observation, registration and
authorization. A sealed listener-owner value is evidence to validate against
the trusted producer and current revisions, never an execution grant.

`network/intent` contains the existing typed fields and JSON representations.
Feature policy builds desired values; the existing Planner checks current
Host, resource, owner, revision and generation before admitting a plan. The
package supplies no Planner, accepted receipt, executor or mutable registry.

`network/nftcontract` preserves the existing wire fields and JSON identities so
feature declarations and the authorized platform adapter can share the same
types. Constructing a request grants no authority. The adapter still validates
the exact managed root, table, current revision, lock and rollback evidence
before executing an admitted operation.

Resource lease requests carry exact references and revisions. Pure transition
validation admits no owner or endpoint by itself: the provider retains its
reservation store and applies the same owner checks. Probe values bind evidence
to the target, provider instance, operation, mutation marker, challenge and
generation. Their validators perform no network activity. The registered owner
retains replay prevention, bounded invocation and actual observation.

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

ComponentHost owns catalog projection and local lifecycle and metadata-removal rules. Composition supplies installed/enabled state, existing admission, the metadata writer, and the supervisor. Panel request authorization and storage stay outside Host. The scheduled-job helper does not construct a second scheduler; its caller supplies the existing operation admission and retains registration/stop ownership. SecretRef provides masked metadata and owner-use validation without storing or revealing secrets.

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

## Source bindings

`outbounds.PreviewBinding` and `ApplyBinding` use an explicit Local Host, source and endpoint identity, provider, source revision and Outbound before-image revision. The trusted Backend adapter retains its transaction and canonical Outbounds writer. Preview supplies no authority; application repeats admission under the owner fence. An accepted source snapshot never establishes an applied binding. A caller-owned uncommitted transaction returns PENDING; APPLIED requires a read-back of the exact owner write and confirmed transaction finalization. Cancellation or uncertain commit remains UNKNOWN without replay. ObserveBinding reads the explicit target and cannot fall back from an unavailable Host to Local. These receipts establish durable Outbound configuration only; Core activation and traffic health remain separate observations.

## Shared subscription projections

The subscription packages preserve the existing native, Xray, Mihomo/Clash and URI observation model. Source import and Users Publishing consume these same types and algorithms. Unmapped critical semantics remain inspectable and cannot silently become permissive runtime defaults. Snapshot acceptance grants no Outbound write or publication authority.

The parser port takes one bounded format representation, its existing conversion policy and an operation context. Backend supplies its existing parser implementation. The external source owner retains representation selection and legacy multi-representation merge policy. This module does not fetch sources, load files, own durable data, start workers or render public user subscriptions.
