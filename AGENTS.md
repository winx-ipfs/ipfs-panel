# Working on this repository

**English** · [中文](AGENTS.zh-CN.md)

This file is for an AI agent or a human who has just been handed this repository. Read it before touching anything. It says what the product is, what is already decided, how to verify a claim, and how to leave the repository in a state the next person can pick up.

The rules here are enforced where they can be. A rule that only exists as prose is a rule that gets skipped — this repository has already seen that happen four times in one session.

## What this is

A panel for Kubo, the Go IPFS node. `apps/web` is the UI (Vue 3, Vite, TypeScript). `services/manager` is the API gateway (Go, standard library). The browser never talks to Kubo RPC; the manager does, on loopback, and it authenticates first.

`docs/VISION.md` says what this is and what it refuses to become. `docs/ROADMAP.md` numbers the slices; slices 1–4 are shipped as v0.1.0 and slice 5 is next.

## The three files that decide arguments

When two sources disagree, this is the order:

1. **`docs/design/*.md`** — the spec for the thing they describe. Header says it: if the code and the file disagree, the file wins until a pull request changes it. Changing behaviour means changing the spec in the same pull request.
2. **`docs/REFERENCE-ipfs-stack.md`** — where the truth about Kubo and boxo lives, with the exact upstream commits every claim was checked against.
3. **The upstream clones themselves** — `reference/`, gitignored, built by `.github/scripts/fetch-reference.sh`.

If you cannot point at a file and a line for a claim about Kubo, boxo, or IPFS, you do not have that claim. Write "unverified" instead. Three confident-and-wrong statements were already made and corrected in this repository; each would have been caught by one `grep`.

## The loop

1. **Read before writing.** `docs/HANDOFF.md` first, then the design doc for the area you touch, then `docs/ROADMAP.md` for where it sits.
2. **Decide whether it is a slice or a fix.** A slice changes what a person can do. A fix restores something the spec already promised. Both need a spec line, not only code.
3. **Work on a branch with a valid prefix** — `feature/`, `fix/`, `chore/`, `docs/`, `refactor/`, `test/`, `ci/`, `dependabot/`. Never commit to `develop` or `main`; both are protected and the push will be rejected.
4. **Verify by running it**, not by reading the diff. Details below.
5. **Open a pull request into `develop`** with a Conventional Commit title. Fill the Verification section with what you actually ran — CI rejects the pull request without it.
6. **Update `docs/HANDOFF.md`** before you stop, even if the work is unfinished.

## Verification, in the order that catches the most

All three are required, not a menu.

**1. The product runs.** Start the manager, exercise the changed path, paste the output. A green test suite is not this step. If the change touches the UI, open the page and look at it. If it touches a binary, run the binary.

**2. The suite is green.** The exact commands are in `README.md`. Note which ones need a C toolchain: `go test -race` needs cgo, so a machine without gcc cannot run it — see [Local environment](#local-environment).

**3. The printed claim matches the observed behaviour.** If the pull request says a stopped node returns `node_not_running`, run it while stopped and show the response. If it says a checksum matches, compute both.

Critical paths need a human decision before you proceed: anything that publishes to `main`, changes authentication, writes outside the data directory, or removes data. Ask; do not infer consent from a prior task.

## Where each kind of work is verified

| Work | Command |
| --- | --- |
| Manager, unit | `cd services/manager && go test ./...` |
| Manager, race (needs cgo) | `CGO_ENABLED=1 go test -race -count=1 ./...` |
| Manager, released-binary integration | see [docs/design/harness.md](docs/design/harness.md) — not wired into CI yet |
| Web | `cd apps/web && npm run lint && npm run typecheck && npm test && npm run build-only` |
| Docs | `.github/scripts/check-docs.sh` |
| Doc citations | `.github/scripts/check-doc-citations.sh` (needs `reference/`) |
| Pull request body | `.github/scripts/check-pr-body.sh` (CI job `pr-body`) |
| Everything CI runs | `.github/workflows/ci.yml`; `ci-ok` is the only required check |

CI runs the same commands on `ubuntu-latest`. If it passes locally and fails there, the difference is the platform or the toolchain, and that difference is the bug.

Two CI jobs exist only to make this section real: `docs` fails when a doc loses its `.zh-CN` pair or a relative link breaks, and `pr-body` fails when the pull request has no Verification evidence. Both are wired into `ci-ok`.

## Platforms: the thing that keeps biting

The manager ships linux, macOS, and Windows binaries. A change to any of these needs a Windows check, because all three fail only there:

- **Directory fsync.** `os.File.Sync` on a directory returns access-denied on Windows. `internal/atomicfile` splits this per platform; do not unify it.
- **Refused connections.** Windows reports `WSAECONNREFUSED` (10061), not POSIX `ECONNREFUSED`. `internal/kuborpc.IsRefused` compares both.
- **Permission bits.** Windows has no `0600`/`0700`. Tests that assert a mode live in `perm_unix_test.go` / `perm_windows_test.go` pairs.

Any test asserting a mode, a path separator, a signal, or an error string is a portability bug waiting to happen.

## Writing docs

Every doc ships as a pair: `name.md` and `name.zh-CN.md`, each with a switch line near the top. CI enforces the pair and fails on a broken relative link. `CODE_OF_CONDUCT.md` is the only exception.

UI copy is the same rule: a string lands in both `apps/web/src/locales/en.ts` and `zh.ts` in one change.

## Local environment

Observed on the machine this repository was developed on; check rather than assume, but expect these:

- **No gcc on Windows**, so `-race` cannot run there. WSL with Go and gcc can. If you cannot run `-race`, say so in the pull request instead of claiming race coverage.
- **`http_proxy` is set** and will break `curl` to localhost and to release downloads. Use `--noproxy '*'` for localhost, and use the GitHub API asset endpoint when the release redirect fails.
- **`/dev/null` may not resolve** in the shell used here. Redirect to a real file instead of discarding output.
- **Git Bash is the shell**, so `/c/...` paths and Windows `C:/...` paths behave differently depending on the tool.

## What not to do

- **Do not guess at Kubo behaviour.** See `docs/REFERENCE-ipfs-stack.md`. Every entry point and response shape is checkable in `reference/`.
- **Do not hand-write a stand-in for something you are trying to test.** `internal/node/fakeipfs` stands in for the `ipfs` CLI to test process lifecycle; it never speaks RPC, and making it do so would test our guesses rather than Kubo.
- **Do not add a second way to do something that already exists.** If the spec says how, follow it, or change the spec first.
- **Do not write a check that cannot fail.** Every script added here was tested by injecting the failure it is supposed to catch. A check that only prints "OK" is worse than none.
- **Do not commit `reference/`.** It is gitignored upstream source, never part of the product.

## Leaving the repository

Before you stop — finished or not:

1. `.github/scripts/check-docs.sh` and the test suite pass, or the pull request states which one does not and why.
2. `docs/HANDOFF.md` says what changed, what is decided, what is still open, and what the next step is.
3. No scratch files, no local clones inside tracked paths, no commented-out experiments.

A reader who arrives with no conversation history must be able to answer "what is true now" and "what should I do next" from the files alone. That is the test this file exists to pass.