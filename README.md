# Solovey Host

`solovey-host` provides reusable Go libraries for host-local facts and component integration. Solovey Panel and independent consumers use the same versioned source while retaining their own observation, authorization and storage adapters.

## Packages

| Package | Responsibility |
| --- | --- |
| `hostref` | Explicit Local Host identity. |
| `networkidentity` | Address classification, evidence freshness, bounded snapshots and deterministic revisions. |

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

## License and provenance

The library retains the original [GNU GPLv3 license](LICENSE). Source attribution, original paths and commit history are recorded in `SOURCE_PROVENANCE.json`. See [NOTICE](NOTICE) for attribution.
