# RedisSleuth Engineering Constraints

These instructions apply to the entire repository.

## Product and compatibility

- RedisSleuth is a read-only, low-intrusion diagnostic CLI for Redis Cluster.
- Keep the module compatible with Go 1.23 and later.
- Keep Redis command and protocol behavior compatible with Redis 5.0.x. Use RESP2
  when compatibility is relevant, and do not require ACL, RESP3, or Redis 6/7-only
  commands or INFO fields.
- Support Linux and Windows builds. Avoid Unix-only behavior unless it has a
  portable implementation or build-tagged alternative.
- This MVP supports Redis Cluster only. Do not silently treat standalone or
  Sentinel deployments as supported.

## Redis safety

- Redis access must remain read-only. Never add commands that mutate application
  data, configuration, topology, persistence state, or replication state.
- Never use `KEYS` for discovery. Key analysis must use bounded `SCAN` sampling.
- Do not enable or execute `MONITOR` by default. Adding a MONITOR path requires an
  explicit product decision and prominent operational warnings.
- Every Redis request must have a context deadline or client timeout.
- Keep sampling bounded by count, rate, duration, and sample-size controls.
- Prefer scanning a healthy replica. Fall back to its master only when necessary.
- Do not claim that a client IP accessed a specific Key unless direct evidence
  exists. CLIENT LIST, SLOWLOG, and temporal correlation alone are not proof.
- Key output is masked by default. Full Key output must remain opt-in.
- Authentication secrets must come from environment variables and must never be
  logged, committed, included in findings, or passed as ordinary CLI values.

## Code structure and behavior

- Keep external Redis interactions behind interfaces so collectors and analyzers
  can be tested without a live cluster.
- Use `log/slog` for logs and Cobra for CLI commands.
- Preserve the stable JSON envelope: `metadata`, `cluster`, `nodes`, `findings`,
  and `recommendations`.
- Missing Redis 5 INFO fields mean “unknown”; do not infer a healthy or unhealthy
  state merely from a missing newer-version field.
- CPU percentage must be calculated from cumulative CPU-time deltas over the
  actual sample interval. Label it as Redis process single-core CPU percentage.
- Slot calculation must follow Redis CRC16/XMODEM and exact Hash Tag semantics.

## Validation and security

- Format changed Go files with `gofmt`.
- Before publishing changes, run:

  ```text
  go test ./...
  go vet ./...
  go build ./...
  ```

- Changes to parsing, slot calculation, masking, sampling math, or rules require
  focused unit tests.
- Keep `.github/workflows/security.yml` enabled. Security changes must not weaken
  its permissions, remove `govulncheck`, `gosec`, or CodeQL, or replace pinned
  third-party Action SHAs with floating branches.
- Use least-privilege GitHub Actions permissions. Never expose Redis credentials
  to pull-request workflows.
- Do not suppress a security finding without documenting why it is a false
  positive and narrowly scoping the suppression.

## Documentation

- Keep README usage, Redis 5.0.x compatibility, safety guarantees, output
  semantics, and MVP limitations synchronized with implementation changes.
- Examples must use placeholder addresses and environment-variable secret names,
  never real credentials or production identifiers.
