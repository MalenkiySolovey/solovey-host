# Solovey Host

`solovey-host` provides reusable Go libraries for host-local facts and component integration. Solovey Panel and independent consumers use the same versioned source while retaining their own observation, authorization and storage adapters.

## Packages

| Package | Responsibility |
| --- | --- |
| `hostref` | Explicit Local Host identity. |
| `networkidentity` | Address classification, evidence freshness, bounded snapshots and deterministic revisions. |
| `componenthost` and `componenthost/manifest` | Validated component descriptors, declared capabilities and neutral lifecycle contracts. |
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

Component composition retains installed/enabled admission, authorization and storage. The lifecycle contracts expose only capabilities already used by their consumers. The scheduled-job helper does not construct a second scheduler; its caller supplies the existing operation admission and retains registration/stop ownership. SecretRef provides masked metadata and owner-use validation without storing or revealing secrets.

Bot API callers supply their approved HTTP client and operation context. The helper creates no background client, bot registration or polling loop. Proxy selection and validation remain with caller adapters. The only external Go dependency is the existing `github.com/robfig/cron/v3` v3.0.1 contract.

## License and provenance

The library retains the original [GNU GPLv3 license](LICENSE). Source attribution, original paths and commit history are recorded in `SOURCE_PROVENANCE.json`. See [NOTICE](NOTICE) for attribution.
