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
| Roadmap | slices 1–4 shipped in v0.1.0; slice 5 (scenes) not started; slice 6 (expert mode, multi-node) later |

Shipped in v0.1.0: the panel shell in English and 中文, first-run admin setup and session login, node start/stop/restart with a one-sentence status, files on the node's mutable file system, and pins. Binaries for linux, macOS, and Windows, plus a web bundle, published as a GitHub Release with checksums.

## What is decided

These are settled. Do not reopen them without a reason and a pull request that says why.

- **The manager is the only process that calls Kubo.** The browser never receives a Kubo RPC address. `docs/VISION.md` and `docs/design/auth.md`.
- **Kubo integration stays on the process boundary for now.** `docs/design/kubo-integration.md` compares that against a Kubo plugin and recommends the boundary. The plugin path is real but costs a Windows fork of the Kubo build and an fx graph Kubo does not keep stable.
- **The test baseline is a released Kubo binary, not a library import.** `docs/design/harness.md`. The in-process idea was considered and rejected; the file records why.
- **Dependencies are pinned, and the reference clones are pinned to exact commits.** `docs/REFERENCE-ipfs-stack.md` holds the table; `.github/scripts/fetch-reference.sh` reproduces it.

## What is open

Each of these needs a decision, not code.

1. **Does the panel need the node's bitswap for anything a gateway call cannot do?** This is the question that decides whether a Kubo plugin has a feature behind it. If the answer is no, the plugin model has nothing to carry and the boundary stands. Answer it before writing plugin code.
2. **Slice 5 (scenes) has no design yet.** `docs/ROADMAP.md` says what it is for: a named setup the panel can describe in one sentence and then write into Kubo's config. It needs a `docs/design/scenes.md` before implementation, including which config keys each scene writes, which changes need a restart, and what happens when the node is running.
3. **The integration harness is designed but not built.** `docs/design/harness.md` specifies a CI job that downloads a released Kubo binary and runs the existing `TestRealKuboOptional` against it. Today that test skips unless a developer sets `IPFS_PANEL_INTEGRATION=1` and has `ipfs` on `PATH`, so CI never exercises the RPC surface.
4. **Whether Dependabot actually retargets.** v0.1.1 put `target-branch: develop` on `main`, which is the copy GitHub reads. Dependabot's own runs now show `head_branch: develop`, so it took effect. The remaining check is the next pull request it opens: if that one still targets `main`, the setting is being ignored and the alternative is to make `develop` the default branch (`docs/ORG-SETUP.md` step 3).

## Known gaps worth naming

- **The RPC surface is untested in CI.** Three specific guesses are listed in `docs/design/harness.md`: the `pin/ls` rejection message shape, multipart without `Content-Length`, and the `object/stat` response shape. Each could be wrong today and no test would say so.
- **`docs/design/*.md` are the authority, and two of them describe work that does not exist yet.** `harness.md` is design only. `kubo-integration.md` is design only. Read their status lines.
- **Windows coverage depends on a developer running the tests there.** CI is ubuntu-only. The three Windows defects found in slice 1–4 were all found by a human running `go test` on Windows.
- **`ci-ok` requires eight jobs, and `web`/`manager` skip when their paths do not change.** That is intentional; `branch-policy` explains the ruleset. Do not add a required check without updating `.github/workflows/ci.yml` and `docs/ORG-SETUP.md` together.

## Next step, concretely

If you have one session and no other instruction:

1. Read `AGENTS.md`, then `docs/ROADMAP.md` slice 5.
2. Answer open question 1 above, out loud, in `docs/design/kubo-integration.md`. It is a sentence, and it unblocks the plugin question permanently.
3. If slice 5 is the goal instead, write `docs/design/scenes.md` first. The four scenes named in `docs/VISION.md` are a quiet node, sharing a folder, pinning a set of content, and a read-only public gateway. Each needs its Kubo config keys identified from `reference/kubo/docs/config.md`, with a citation.

Do not start with code. Every slice in this repository started with a design doc, and the two that were coded before their doc was settled are the two that needed rework.

## How this file stays true

Update it in the same pull request that changes what it says. A stale handoff is worse than none, because the next reader trusts it.

It is not enforced by CI. It is enforced by `AGENTS.md` step 6 and by review. If you notice it drifting, fix it before doing anything else.