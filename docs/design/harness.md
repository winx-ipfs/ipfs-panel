# Test harness: a released Kubo binary, in CI

**English** · [中文](harness.zh-CN.md)

**Status: design only.** No code yet. This file is the spec for how the manager's tests reach Kubo. If the code and this file disagree, this file wins until a pull request changes it.

Every claim below carries its evidence, because the point of this file is that a later reader does not have to take anything on trust. Sources were cloned to `reference/` at the commits named in [REFERENCE-ipfs-stack](../REFERENCE-ipfs-stack.md).

## The problem this fixes

The manager reaches Kubo over HTTP RPC on loopback. `internal/files`, `internal/pins`, and the endpoint probe in `internal/node` issue about forty calls: `files/ls`, `files/stat`, `files/write` (multipart), `files/read`, `files/mv`, `files/cp`, `files/rm`, `files/mkdir`, `object/stat`, `pin/add`, `pin/ls`, `pin/rm`.

Today they are proven against hand-written `httptest` stubs, and against `TestRealKuboOptional` — which skips unless `IPFS_PANEL_INTEGRATION=1` and `ipfs` is on `PATH`. No CI job sets either. So the RPC surface is never exercised by CI, and the stubs encode our own guesses about it. When a guess is wrong the stub agrees with the client and both are wrong together.

The three guesses most likely to be wrong, named so a future reader can check them first:

1. `pins/client.go` decides whether to retry `pin/ls` without `names=` by matching words in a 400 message body. That message shape is invented.
2. `files/client.go` sends `files/write` as multipart with `Content-Length` left unset. Whether Kubo's reader accepts that is a property of Kubo.
3. `object/stat` and `files/stat` response shapes were copied by hand. A field rename passes every test we have.

`internal/node/fakeipfs` does not cover this and is not meant to. It is a stand-in for the **`ipfs` CLI** — `init`, `daemon`, `id`, `shutdown`, `config` (`internal/node/fakeipfs/main.go:24-35`). Its `daemon` holds the repo lock, writes a pidfile, and waits for a signal; it never opens an HTTP listener. It tests process lifecycle, correctly. It is not an RPC server.

## The decision

CI downloads a **released Kubo binary** and runs the manager's clients against it as a subprocess. Not kubo-as-a-library, not an in-process node.

The reason is the contract being tested. The `/api/v0` surface is Kubo's own, and its compatibility rule is that endpoints are not removed and response shapes are not changed (kubo `AGENTS.md`, "Never break the `/api/v0/` RPC API"). That promise is made about *released binaries that users install*. A library import pinned at a commit tests a moving target and cannot see a release regression.

Costs, stated plainly:

- CI downloads roughly 30 MB per run, per platform that runs the test.
- The test is slower than a pure unit test: process start plus daemon readiness.
- The pinned version must be bumped deliberately, and a bump is a visible change in a pull request.

The user's instruction for this design was that CI time is acceptable and the test must be complete. That settles the tradeoff.

## Shape

```text
.github/workflows/integration.yml
  |
  +-- download ipfs vX.Y.Z for the runner's OS/arch from the kubo release
  +-- verify the published checksum
  +-- IPFS_PANEL_INTEGRATION=1
  +-- PATH includes the unpacked binary
  +-- go test -run TestRealKubo ./services/manager/internal/node/
```

The existing test is already the right shape and needs no rewrite: it looks up `ipfs` on `PATH`, skips on its own if the binary or the env var is missing, and refuses to run against `fakeipfs` (`internal/node/integration_test.go:18-27`). What is missing is the workflow that supplies the binary.

The clients need no change at all. They already refuse a non-loopback base URL and read the API address out of the repo config.

## What the integration test must cover

The existing `TestRealKuboOptional` exercises the MFS round trip and the pin list. That is a start, not a contract. Each of these is a guess that only a real binary can settle:

| Check | Why it cannot be a stub |
| --- | --- |
| `files/write` multipart with no `Content-Length` | Kubo's multipart reader decides, not ours |
| upload over `MaxBytes`, partial write cleanup | Depends on when Kubo aborts the stream |
| `pin/add` with `progress=true` shape | Progress framing is Kubo's |
| `pin/ls` with `names=true`, and the 400 fallback | The fallback fires on a message we guessed |
| `pin/add` with `name=`, and its fallback | Same shape, different argument |
| `object/stat` on a CID | Field names and wrapper |
| `files/ls` `long=true` entry fields | `Type` encoding, `Mtime`, `Hash` |
| `files/stat` on a directory versus a file | `Type` differs |
| `pin/rm` on an indirect pin | Error text and status |
| stopped node | Connection refused, not a timeout |

The last one is already covered for Windows by `internal/kuborpc` and is not repeated here; it is listed because running it against a real stopped daemon is cheap once the binary is present.

## Version pinning

Pin one released version, and change it only in a pull request that says why. The API version string Kubo sends is `/kubo/<version>/` (`version.go:33`), so a mismatch is visible in a request rather than silent.

As of this writing the latest release is `v0.43.1` (2026-09-15; `git ls-remote --tags` on the clone). `master` reports `CurrentVersionNumber = "0.44.0-dev"` (`version.go:31`), so the clone is ahead of the release and must not be used as the test baseline.

Two options for later, not now:

- A second matrix entry against the previous minor release, to catch a break in both directions.
- A nightly against `master`, report-only, never blocking a pull request.

Neither is worth building until the first version runs green on every platform.

## Not in this design

- Replacing `fakeipfs`. It tests a different layer and does it correctly.
- An in-process `github.com/ipfs/kubo` import. Rejected above; it would put a large dependency in `go.mod` to test something other than what users install.
- Testing the gateway. The manager does not serve or configure it in slices 1–4.
- Multi-node tests. Roadmap slice 6.

## Open questions

- **Which platforms run it.** Windows, Linux, macOS coverage is the ideal. The binary is available for all three; start with the runner CI already uses, add a platform when a platform-specific bug appears.
- **Whether the test starts its own daemon or expects one.** `TestRealKuboOptional` uses the supervisor to start it, which also exercises the lifecycle against the real binary. Keep that. A test that assumes a running daemon would not test our start path.
- **Repo directory per test.** The existing test uses `t.TempDir()`. Kubo init costs a key generation; that is acceptable at this size.