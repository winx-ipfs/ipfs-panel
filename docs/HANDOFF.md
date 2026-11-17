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
| Open pull requests | two Dependabot updates: #37 (manager group) and #38 (web group), both into `develop` |
| Roadmap | slices 1–4 shipped in v0.1.0; slice 5 (scenes) and the Explore and Peers pages are on `develop`; the released-Kubo harness is on `develop`; the first half of slice 6 — the Expert raw config editor and the Logs page — is on `develop`; the second node is not |

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
- **Peers is a table and a one-time dial.** The manager calls `swarm/peers` and `swarm/connect` on loopback. Connected peer addresses stay off the browser. There is no map, and the page does not write `Peering.Peers`. The spec is `docs/design/peers.md`.
- **The panel uses the winx.run visual language, at ops-panel density.** `docs/DESIGN-SYSTEM.md`. Paper gray, white cards, black primary buttons (near-white in the dark theme, including when disabled), firefly green (`#20FFA9`) only for the mark, the selected bar, and small tags. The light sidebar is white; the dark selected row is a neutral lift plus that bar, not a green block. The main column fills the window: a toolbar and a list or property surface that starts under the title and runs to the bottom. Ops pages are a flex column so a closed disclosure cannot shove the surface down. The node surface adds the current pins and a few connected peers from the existing APIs, and the card ends after the last row. Table cells, including the first column, use the toolbar's 12px inset. Peers connect sits on that list, on the same baseline as the address input, with search on the other side. shadcn-vue (Nova, themed) plus Motion. Not a charcoal 1Panel clone, not IPFS webui teal, and not a mint wash. Do not restyle it back.
- **The raw config editor is expert mode.** It is the Advanced tab on Settings, not a nav item. The manager reads with Kubo `config/show` and writes with `config/replace`. `Identity.PrivKey` never reaches the browser. `Addresses.API` cannot be changed. A backup can be restored. A restart is offered. Scenes still apply and undo. The spec is `docs/design/config-editor.md`.
- **Logs are one page and two sources.** The manager's own operational and audit log, and the stdout and stderr of the daemon this manager spawned. The browser reads them through the manager, with the existing session. `log/tail` is not called. An expert can set one Kubo subsystem level through the manager. Clear view does not delete the files. Secrets are redacted before a line is stored. The spec is `docs/design/logs.md`.

## What is open

Each of these needs a decision, not code.

1. **Whether Dependabot actually retargets.** v0.1.1 put `target-branch: develop` on `main`, which is the copy GitHub reads. Dependabot's own runs now show `head_branch: develop`, so it took effect. The remaining check is the next pull request it opens: if that one still targets `main`, the setting is being ignored and the alternative is to make `develop` the default branch (`docs/ORG-SETUP.md` step 3).
2. **Webui parity.** The raw config editor is in, on Settings, marked Expert. The spec is `docs/design/config-editor.md`. The logs page is in. The spec is `docs/design/logs.md`. IPNS keys, remote pinning, analytics, the retrieval check, and DHT provide stay deferred. Do not fold those into a logs fix.

## Known gaps worth naming

- **Pull requests into `develop` run the RPC surface against Kubo v0.43.1.** The pin and the checksum are in `.github/scripts/fetch-kubo.sh`. On that release, `object/stat` is a removed command, so pin sizes stay unset. The other checks in `docs/design/harness.md` pass against the binary. A local run still skips without `IPFS_PANEL_INTEGRATION=1`.
- **`docs/design/*.md` are the authority.** `harness.md` is implemented. `kubo-integration.md` is a decision, not a build. Read their status lines.
- **Windows coverage depends on a developer running the tests there.** The harness runs on ubuntu only. The three Windows defects found in slice 1–4 were all found by a human running `go test` on Windows.
- **`ci-ok` requires nine jobs.** `web` and `manager` skip when their paths do not change. `integration` runs on pull requests into `develop` and skips otherwise. That is intentional; `docs/ORG-SETUP.md` explains the ruleset. Do not add a required check without updating `.github/workflows/ci.yml` and `docs/ORG-SETUP.md` together.
- **The browser finds what the unit tests cannot.** Three defects shipped past a green `web` job in slice 6: a row menu clipped to nothing by its cell's `overflow: hidden`, a `flex-basis` that became a 280px height once the toolbar stacked, and a test that asserted the start request was the *last* fetch when companion loads can follow it. `npm run check-menus` now drives the real page in the `web` job for the first, and the third is pinned by its own regression test. Anything that depends on computed layout or clipping still needs a browser, not a headless DOM.

## Next step, concretely

If you have one session and no other instruction:

1. Read `AGENTS.md`, then `docs/design/harness.md`, so you do not reopen the harness, Peers, Explore, the four scenes, or the winx visual language in `docs/DESIGN-SYSTEM.md`. Then open the panel in a browser: `npm run check-menus` covers the row menus, and both list pages are worth one look at 390px, where the layout changes shape.
2. Logs are in. Permanent peering and a second node are still not. Do not start those in the same change as a logs fix.

Scenes are in. Explore is in. Peers are in. The raw config editor is in. Logs are in. The released-Kubo harness is in. The bitswap question is answered. Do not start plugin work.

## How this file stays true

Update it in the same pull request that changes what it says. A stale handoff is worse than none, because the next reader trusts it.

It is not enforced by CI. It is enforced by `AGENTS.md` step 6 and by review. If you notice it drifting, fix it before doing anything else.