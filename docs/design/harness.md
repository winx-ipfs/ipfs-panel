# Test harness: a released Kubo binary, in CI

**English** · [中文](harness.zh-CN.md)

**Status: implemented.** Pull requests into `develop` download one pinned Kubo release and run `TestRealKuboOptional` against it. The test still skips on a machine that does not set `IPFS_PANEL_INTEGRATION=1` or does not have `ipfs` on `PATH`. If the code and this file disagree, this file wins until a pull request changes it.

Every claim below carries its evidence, because the point of this file is that a later reader does not have to take anything on trust. Sources were cloned to `reference/` at the commits named in [REFERENCE-ipfs-stack](../REFERENCE-ipfs-stack.md).

## The problem this fixes

The manager reaches Kubo over HTTP RPC on loopback. `internal/files`, `internal/pins`, and the endpoint probe in `internal/node` issue about forty calls: `files/ls`, `files/stat`, `files/write` (multipart), `files/read`, `files/mv`, `files/cp`, `files/rm`, `files/mkdir`, `object/stat`, `pin/add`, `pin/ls`, `pin/rm`.

Unit tests prove them against hand-written `httptest` stubs. `TestRealKuboOptional` runs only when `IPFS_PANEL_INTEGRATION=1` and `ipfs` is on `PATH`. A pull request into `develop` sets both and puts the pinned binary on `PATH`. A local run without them still skips. The stubs encode our own guesses. When a guess is wrong the stub agrees with the client and both are wrong together, which is why the released binary has to run in CI.

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
.github/workflows/ci.yml
  integration                 pull requests whose base is develop
    uses .github/workflows/integration.yml
      |
      +-- .github/scripts/fetch-kubo.sh
      |     download kubo_v0.43.1_linux-amd64.tar.gz from the GitHub release
      |     the published sha512 must equal the pin in that script
      |     then the archive must match that sha512
      +-- IPFS_PANEL_INTEGRATION=1
      +-- IPFS_PANEL_KUBO_VERSION from the script (0.43.1)
      +-- PATH includes the unpacked binary
      +-- go test -count=1 -run TestRealKubo ./services/manager/internal/node/
      +-- a skip is a failure
  ci-ok needs integration
```

`integration.yml` is a `workflow_call` workflow, not a separate required check. The ruleset requires only `ci-ok`. A job in another workflow cannot be a `needs:` of `ci-ok`, and adding `integration` itself as a required check would block merges: the job skips on push and on a pull request into `main`, and a skipped required check blocks the merge. `ci-ok` already treats skip as success for the jobs that are not on every event. A failure of this job fails `ci-ok` the same way `manager` does.

`-count=1` is there so a cached skip cannot pass. The test looks up `ipfs` on `PATH`, skips on its own if the binary or the env var is missing, and refuses to run against `fakeipfs` (`internal/node/integration_test.go:21-30`). The workflow fails if that skip happens, because a skip is a passing `go test`.

The RPC clients need no change. They already refuse a non-loopback base URL. The test reads the API origin from the repo with `LoopbackEndpoints`. The supervisor's readiness probe waits for the repo's `api` file before `ipfs id`. Without that wait, `ipfs id` opens the repo, takes `repo.lock`, and the real daemon exits. That race is invisible against `fakeipfs`. It is recorded in [Node lifecycle](../DESIGN-node-lifecycle.md).

## What the integration test must cover

`TestRealKuboOptional` keeps the MFS round trip and adds the checks below. Each one is a guess that only a real binary can settle:

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

The last one is already covered for Windows by `internal/kuborpc`. The integration test also runs it against the real daemon after the supervisor stops it, because that is cheap once the binary is present. The assertion is `node_not_running`, which is connection refused, not a timeout.

### What v0.43.1 settled

Checked by running the released `v0.43.1` binary, not by reading the reference clone. The clone is ahead of this release.

- `files/write` as multipart with no `Content-Length` is accepted. The body round-trips.
- An upload over `MaxBytes` returns `too_large`, and the path is gone afterwards.
- `pin/add` with `progress=true` emits a JSON object with `Progress` (a node count) and then one with `Pins`. A local pin of a file already in the repo emitted `Progress` 2.
- `pin/ls` with `names=true` and `pin/add` with `name=` are accepted. `NamesFrom` is `kubo`, and the name comes back on the pin. The 400 fallback does not run against this release. The test fails if this release starts rejecting those arguments with a message the client does not recognize, and it also fails if the fallback runs, because that would mean this release no longer accepts them.
- `POST /api/v0/object/stat` returns HTTP 500 `{"Message":"removed, use 'ipfs dag' or 'ipfs files' instead","Code":0,"Type":"error"}`. `FillSizes` leaves `Size` unset. The test fails if a size appears. `dag/stat` walks the DAG, which [Pins](pins.md) does not do, so the client stays on `object/stat` and a miss stays a miss.
- `files/ls` with `long=true` returns `Name`, `Type` (an integer: 0 file, 1 directory), `Size`, and `Hash`. It does not return `Mtime`. `files/stat` returns `Type` as the string `file` or `directory`. After `ipfs files touch --mtime`, `files/stat` returns that `Mtime`, and the list entry's modified time comes from that stat.
- `pin/rm` of a CID that is only pinned indirectly returns HTTP 500 `{"Message":"not pinned or pinned indirectly","Code":0,"Type":"error"}`. The client maps that to `indirect`.
- After the supervisor stops the daemon, a files stat returns `node_not_running`.

`IPFS_PANEL_KUBO_VERSION=0.43.1` locks those results. Any other non-empty value fails the test until the pin pull request writes the new expectations. An empty value, which is a local run, still requires the calls to succeed and the two name outcomes to be coherent (`kubo` with the name, or `panel` after the fallback).

## Version pinning

Pin one released version, and change it only in a pull request that says why. The API version string Kubo sends is `/kubo/<version>/` (`version.go:33`), so a mismatch is visible in a request rather than silent.

The pin is `v0.43.1`, in `.github/scripts/fetch-kubo.sh`, with the published sha512 of `kubo_v0.43.1_linux-amd64.tar.gz`. Checked again on 2026-11-10 against `git ls-remote --tags https://github.com/ipfs/kubo` and the GitHub releases API: `v0.43.1` (2026-09-15) is still the latest stable release. Tags named `v0.5.x` are older than `v0.43.1`. The reference clone still reports `CurrentVersionNumber = "0.44.0-dev"` (`version.go:31`), so the clone is ahead of the release and must not be used as the test baseline. The pin stays `v0.43.1` because it is still the latest release, which is the binary users install.

Two options for later, not now:

- A second matrix entry against the previous minor release, to catch a break in both directions.
- A nightly against `master`, report-only, never blocking a pull request.

Neither is worth building until the first version runs green on every platform.

## History: what this replaced

An earlier draft of this file recommended the opposite: run Kubo as a library in the test process, under `internal/kubotest`, with the released-binary test left opt-in. It was replaced because the thing being tested is Kubo's own `/api/v0` surface, and that compatibility promise is made about *released binaries users install*. A library import pinned at a commit tests a moving target and cannot see a release regression. The requirement for this change was that CI time is acceptable and the test must be complete, which is what settled it.

If a future reader finds the in-process idea in a branch, an old review, or a draft, this section is why it is not the current design. Do not add it back as a second path: two harnesses would mean two answers to "does the manager work against Kubo".

## Not in this design

- Replacing `fakeipfs`. It tests a different layer and does it correctly.
- An in-process `github.com/ipfs/kubo` import. Rejected in [History](#history-what-this-replaced) above; it would put a large dependency in `go.mod` to test something other than what users install.
- Testing the gateway. The manager does not serve or configure it in slices 1–4.
- Multi-node tests. Roadmap slice 6.

## Decided with the job

- **Which platforms run it.** Ubuntu, the runner CI already uses. linux/amd64 is the only pinned checksum. The script rejects any other OS or arch. Add a platform when a platform-specific bug appears, with that platform's published checksum in the same pull request. There is no macOS or Windows matrix.
- **The test starts its own daemon.** `TestRealKuboOptional` uses the supervisor, which also exercises the lifecycle against the real binary. A test that assumes a running daemon would not test our start path.
- **Repo directory.** `t.TempDir()`. Kubo init costs a key generation; that is acceptable at this size.