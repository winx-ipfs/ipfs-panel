# Handoff

**English** · [中文](HANDOFF.zh-CN.md)

Read this first, then `AGENTS.md`. This file is the state of the work: what shipped, what is decided, what is open, and what to do next. It is not a tutorial and not a plan; it is the answer to "what is true right now".

No commit hashes in this file. They go stale the moment the pull request that updates this file is merged, so a hash here would be wrong by construction. Read the branches instead:

```bash
git log --oneline -1 origin/main origin/develop && git tag -l
```

## Where the repository stands

| | |
| --- | --- |
| `main` | the branch that ships. Every commit on it is a release; the latest is v0.1.1 |
| `develop` | the integration branch. Feature work lands here first |
| Latest tag | v0.1.1 |
| Open pull requests | none |
| Roadmap | slices 1–4 shipped in v0.1.0; slice 5 (scenes) is on `develop`; Explore is on `develop` after this work; slice 6 (expert mode, multi-node) later |

Shipped in v0.1.0: the panel shell in English and 中文, first-run admin setup and session login, node start/stop/restart with a one-sentence status, files on the node's mutable file system, and pins. Binaries for linux, macOS, and Windows, plus a web bundle, published as a GitHub Release with checksums.

## What is decided

These are settled. Do not reopen them without a reason and a pull request that says why.

- **The manager is the only process that calls Kubo.** The browser never receives a Kubo RPC address. `docs/VISION.md` and `docs/design/auth.md`.
- **Kubo integration stays on the process boundary for now.** `docs/design/kubo-integration.md` compares that against a Kubo plugin and recommends the boundary. The plugin path is real but costs a Windows fork of the Kubo build and an fx graph Kubo does not keep stable.
- **The test baseline is a released Kubo binary, not a library import.** `docs/design/harness.md`. The in-process idea was considered and rejected; the file records why.
- **Dependencies are pinned, and the reference clones are pinned to exact commits.** `docs/REFERENCE-ipfs-stack.md` holds the table; `.github/scripts/fetch-reference.sh` reproduces it. `ipfs-webui` v4.13.0 is pinned there for the parity checklist.
- **The panel does not need the node's bitswap.** A gateway read already uses it, and nothing the panel does needs the ledger or in-process code. `docs/design/kubo-integration.md`. No plugin work.
- **Scenes are four named setups.** `quiet`, `share`, `pinset`, and `gateway`. The keys, the restart rule, and the read-only public gateway exception are in `docs/design/scenes.md`. The control API stays loopback.
- **Explore looks inside one content ID.** The manager calls `dag/resolve` and `dag/get` on loopback. The browser does not fetch the gateway and does not receive the RPC address. The spec is `docs/design/explore.md`.

## What is open

Each of these needs a decision, not code.

1. **The integration harness is designed but not built.** `docs/design/harness.md` specifies a CI job that downloads a released Kubo binary and runs the existing `TestRealKuboOptional` against it. Today that test skips unless a developer sets `IPFS_PANEL_INTEGRATION=1` and has `ipfs` on `PATH`, so CI never exercises the RPC surface.
2. **Whether Dependabot actually retargets.** v0.1.1 put `target-branch: develop` on `main`, which is the copy GitHub reads. Dependabot's own runs now show `head_branch: develop`, so it took effect. The remaining check is the next pull request it opens: if that one still targets `main`, the setting is being ignored and the alternative is to make `develop` the default branch (`docs/ORG-SETUP.md` step 3).
3. **Webui parity still open, in this order.** A peers page (map or table, not only the count), and a raw config editor. The checklist is `docs/design/webui-parity.md`. The raw editor is expert mode and still waits. Do not start it in the same change as the peers page.

## Known gaps worth naming

- **The RPC surface is untested in CI.** Three specific guesses are listed in `docs/design/harness.md`: the `pin/ls` rejection message shape, multipart without `Content-Length`, and the `object/stat` response shape. Each could be wrong today and no test would say so.
- **`docs/design/*.md` are the authority, and two of them describe work that does not exist yet.** `harness.md` is design only. `kubo-integration.md` is design only. Read their status lines.
- **Windows coverage depends on a developer running the tests there.** CI is ubuntu-only. The three Windows defects found in slice 1–4 were all found by a human running `go test` on Windows.
- **`ci-ok` requires eight jobs, and `web`/`manager` skip when their paths do not change.** That is intentional; `branch-policy` explains the ruleset. Do not add a required check without updating `.github/workflows/ci.yml` and `docs/ORG-SETUP.md` together.

## Next step, concretely

If you have one session and no other instruction:

1. Read `AGENTS.md`, then `docs/design/explore.md`, so you do not reopen Explore or the four scenes.
2. The next build is the integration harness in `docs/design/harness.md`, or, if the goal is webui parity, a peers page. Do not add a raw config editor in the same change. The raw editor still waits.

Scenes are in. Explore is in. The bitswap question is answered. Do not start plugin work.

## How this file stays true

Update it in the same pull request that changes what it says. A stale handoff is worse than none, because the next reader trusts it.

It is not enforced by CI. It is enforced by `AGENTS.md` step 6 and by review. If you notice it drifting, fix it before doing anything else.