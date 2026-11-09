# Network identity

This package finalizes bounded Local Host observations. Callers retain their own interface/resource providers and call `Finalize` when reading stored snapshots so evidence ages correctly.

`AddressScope` reports conservative address-range facts. Private addresses do not establish LAN roles; global unicast addresses do not prove public reachability. `Empty` represents unprovided facts explicitly as UNKNOWN. Observations belong to the current process network namespace, including inside containers.

Evidence includes source, authority, observation time, expiry and revision. Available evidence requires a recognized source/authority, a 32-byte hexadecimal revision, no future observation and a TTL of at most 300 seconds. Invalid or stale evidence becomes UNKNOWN. Collection bounds are retained at 4096 facts. Finalization copies nested collections and computes a deterministic JSON-based revision without promoting missing information.

The JSON schema identity remains `solovey-ui/host-network-identity/v1`. The package does not collect network observations, infer remote facts, select advertised endpoints or mutate routing and DNS policy.
