# Feature list and development plan

**English** · [中文](PLAN.zh-CN.md)

This is the agreed list of what the panel already does, what comes next, and what it will not do. A later agent should read this before starting a slice. Inventory date: 2026-11-25 (Asia/Shanghai).

`docs/HANDOFF.md` is still the answer to "what is true right now". This file is the plan. If the order or the scope changes, update this file and the "Next step" in the handoff in the same pull request.

## What this is

ipfs-panel is a local, open-source ops panel for a real Kubo node. It is not a SaaS product, and it is not the official IPFS marketing console. The aim is that a beginner can start and stop the node, add files, pin content, see who is connected, read logs, and change common settings, without the command line or raw JSON. Experts get an entrance, but it does not block the beginner path. The same web UI will later be reused by a desktop app and by remote control of the manager. The job now is to make one local node solid.

- Repository: https://github.com/winx-ipfs/ipfs-panel
- Brand: 星萤无垠 (winx.run visual language)
- Default branch: `main` (what ships). Integration branch: `develop`
- Latest release: v0.1.1. `main` is still there. `develop` is ahead of that release.

## How to use this document

Before starting work, read in this order. Do not open a new visual restyle from memory. One product slice per pull request. A small refactor that the slice needs can sit beside it.

1. **Read the repository docs.** `AGENTS.md`, `docs/HANDOFF.md` (and the 中文 pair), this file, `docs/DESIGN-SYSTEM.md`, and the `docs/design/*` spec for the slice. Design docs outrank code.
2. **Do the current slice.** That is the first item in [What comes next](#what-comes-next) that is not done, skipping a release unless the user explicitly asked for one. Write the design before the code. A refactor that this slice needs, or that removes an obvious obstacle for a later slice in this plan, can land in the same pull request when the pull request says why. Do not start the next feature. Do not reopen a settled decision without a reason. Do not build a Kubo plugin. Do not restyle the design system.
3. **Land one vertical slice, and open the pull request into `develop` only.** The UI, the manager route, the Kubo call, English and 中文 copy, tests, and screenshots land together. Squash, with a Conventional Commit title. `ci-ok` must be green. Do not merge to `main`, do not tag, and do not release unless the user explicitly asked.
4. **Finish before you hand it over.** Review the states you changed at desktop width, and fix them before you say the slice is done. Check the other theme, the other language, and a narrow width when the layout or the copy can break there. A full matrix is not a gate for a change that cannot affect it. Do not stop after each small defect and wait for confirmation.

## What has landed

Status as of 2026-11-25. "Shipped" means it is on `main` / v0.1.1. "On `develop`" means the code is merged but not in a release yet.

| Feature | Status | Notes |
| --- | --- | --- |
| Panel shell and English / 中文 switch | Shipped | The language switch is saved in the browser. New strings must land in both languages in the same pull request. |
| First-run admin and session login | Shipped | argon2id, sessions, CSRF. No `/api` call without a session. |
| Start, stop, and restart one local Kubo | Shipped | One-sentence status. API and gateway default to loopback only. |
| Files (mutable file system) | Shipped | Add, download, rename, move, copy, and delete. |
| Pins and unpin | Shipped | A page of its own, not hidden in the file browser. |
| Four scene settings | On `develop` | Quiet, share, pinset, and read-only gateway. Preview, apply, and revert. |
| Explore one block of content | On `develop` | Paste a content ID or an `/ipfs/` path; see the type and the links. |
| Peers list and a one-time dial | On `develop` | No map. The one-time dial does not write `Peering.Peers`. |
| Permanent peering | On `develop` | Keep connected on the peers page. Live `swarm peering` plus `Peering.Peers`, with a backup. |
| Install and upgrade Kubo | On `develop` | Download and verify a pinned release. Upgrade inside `>=0.43.1 <0.45.0`. A binary the operator supplied is not replaced. |
| Released-Kubo integration tests | On `develop` | CI pins Kubo v0.43.1. Pull requests into `develop` run them. |
| winx visual language | On `develop` | shadcn-vue + Tailwind + Motion. Do not restyle; fix real defects only. |
| Expert raw config editor | On `develop` | Settings → Advanced. Strip `PrivKey`. Lock `Addresses.API`. Diff, then save. A backup can be restored. |
| Logs page | On `develop` | Manager logs and node stdout/stderr. Secrets are redacted. |

## What comes next

The table order is the delivery order. Do not start the next slice before the previous one has merged into `develop`. A release happens only when the user explicitly says to release.

| Order | Feature | Why now | Done when |
| --- | --- | --- | --- |
| 1 | Release v0.2.0 | `develop` already has a large set of capabilities that `main` (v0.1.1) does not ship, including Kubo install and permanent peering. | A pull request from `develop` into `main`, a tag, and linux / macOS / Windows binaries with checksums. |
| 2 | Remote control of the manager | The manager listens on loopback by default, so a browser cannot manage a node installed on a server. | The manager can bind to a LAN or public address, still using the existing login and CSRF. Kubo RPC stays off the public interface. |
| 3 | Desktop app | The same UI, packaged as an installable local app. Choose the shell in that slice; do not pick it early. | A person can open the local panel like an ordinary app. It still talks to the local manager. |
| 4 | IPNS keys | Expert webui parity, after install and remote control. | Generate, list, publish, and resolve names. Private keys never reach the browser. |
| 5 | Remote pinning | Hand content to a remote pinning service. Not the beginner path. | Configure a service and a secret, submit a pin, and see its status. Secrets do not land in frontend code. |
| 6 | Diagnostics | Retrieval check and DHT provide, so an operator can tell whether content can be found. | A beginner can understand one check. An expert can see enough detail. |
| 7 | A second node | After one node can be installed and reached remotely. This is not a cluster console. | The same panel can manage two Kubo nodes. Existing APIs are not redesigned from scratch. |

Permanent peering is done. The spec is [Permanent peering](design/peering.md). Installing and upgrading Kubo is done. The spec is [Install and upgrade Kubo](design/kubo-install.md).

### 1. Release v0.2.0

This is a release, not a feature slice. Do not start it unless the user has explicitly said to release. The release includes what is already on `develop` (scenes, explore, peers, permanent peering, Kubo install and upgrade, the winx visual language, the config editor, logs). Follow the existing release recipe: a pull request from `develop` into `main`, a tag, binaries, and checksums. The release pull request does not carry a new feature.

### 2. Remote control of the manager

What is exposed remotely is the manager, not Kubo's port 5001. The existing login has to be solid enough before the listen address moves off loopback.

- Write the design first: bind address, TLS, reverse proxies, firewalls, and a warning on the first bind that is not loopback.
- Kubo RPC stays loopback-only. Enforce that in the program, not only in the docs.
- The same web UI. Do not invent a second remote skin.

### 3 through 7. Desktop, IPNS, remote pinning, diagnostics, and a second node

Each of these is its own design and its own pull request. Compare desktop shells (for example Tauri and Electron) when that slice starts, not before. IPNS and remote-pinning keys never reach the browser. Diagnostics are operator checks, not an analytics product. A second node extends "this node"; do not throw away the existing API to build a cluster console.

## What we will not do, or will defer

| Item | How to treat it | Why |
| --- | --- | --- |
| Another visual restyle | Stop | The language is locked. Fix real defects found in use. |
| A Kubo plugin | Will not do | Maintenance cost is high, and it conflicts with users installing their own Kubo. |
| An analytics dashboard | Defer | Not a requirement for an ops panel. |
| Restarting winx.run | Later | Not a slice in this repository. |
| Sites, databases, Docker, OS admin | Will not do | This panel manages an IPFS node, nothing else. |
| Exposing Kubo RPC | Will not do | The browser never calls Kubo itself. |
| A hosted gateway product | Will not do | That is not this product. |
| Switching to React, HeroUI, or DaisyUI | Will not do | The stack is shadcn-vue + Tailwind + Motion. |
| Multi-site | Later | After one local node is solid. |

## How a slice is done

1. Start only after this plan, or the user, names the slice. Branch from current `develop`.
2. In `docs/design/`, write what the person can do, the manager routes, the Kubo calls, verification, how errors are explained, how the slice coexists with what is already there, and the English / 中文 copy.
3. Then write the manager and its tests, and add coverage against real Kubo v0.43.1 in the integration harness.
4. Then write the UI: the work surface fills the window, the toolbar sits on the list, row height is 44, insets match the other pages, and the page uses real data only.
5. Lint, typecheck, Go and web tests (including `-race` when the machine can run it), doc checks, and a screenshot review of the states you changed. Update the handoff, the roadmap, and any parity doc the slice touches.
6. Open the pull request into `develop`. Wait for `ci-ok` to go green. Do not merge it automatically.

The block below can be pasted into a cloud agent's task, then filled in with the scope of the current slice.

```
You are continuing winx-ipfs/ipfs-panel.
Read AGENTS.md, docs/HANDOFF.md (+ zh-CN), docs/PLAN.md (+ zh-CN), docs/DESIGN-SYSTEM.md, and the design doc for this slice first. Design docs outrank code.

Do the named slice. Write the design in docs/design/ before code. One vertical slice: UI + manager + Kubo call + en/zh + tests + screenshots. A small refactor that the slice needs, or that makes the next named slice easier to extend, can land in the same pull request when the pull request says why. Do not start the next feature.
PR into develop only (squash, Conventional Commit). Do not merge to main, do not tag, do not release unless the user explicitly asked.
Do not reopen a settled decision without a reason in the pull request. No Kubo plugin. No new UI libraries. Do not start a visual restyle. A new dependency needs a reason; do not add one by habit.
Browser never talks to Kubo RPC. Never send Identity.PrivKey to the browser. Addresses.API stays locked.
Visual tokens stay locked: #F9FAFB background, white cards, #E5E5E5 lines, #0A0A0A text, #171717 primary (near-white in dark), #20FFA9 only for logo / selection bar / small tags, radii 6/8/10/12/16, Inter + system CJK, 44px rows, motion 150-250ms ease-out. shadcn-vue + Tailwind + Motion only.
Review the states you changed at desktop width. Check the other theme, language, and a narrow width when the layout or the copy can break there. Wait for ci-ok. Do not auto-merge.
```

## Settled rules

Do not reopen these without a reason, and a pull request that says why.

- Stack: Vue 3 + Vite + TypeScript. The manager is the Go standard library (`net/http`), not GoFrame and not chi.
- Architecture: browser → manager → Kubo. The browser never receives a Kubo RPC address.
- Kubo stays on the process boundary. No plugin. The test baseline is a released Kubo binary (currently v0.43.1).
- Components: shadcn-vue + Tailwind + Motion. Do not switch stacks, and do not mix in DaisyUI or HeroUI.
- Colour follows winx.run: paper-gray background, white cards, black primary buttons. Firefly green `#20FFA9` is only for the mark, the selected bar, and small tags.
- Concentric radii: 6 for tags, 8 for buttons and nav, 10 for inputs, 12 for cards, 16 for dialogs.
- Features merge into `develop`, then a pull request from `develop` into `main` ships a release. Squash. `ci-ok` must be green. Do not merge that pull request yourself.
- Do not merge to `main` or cut a release before the user says to release. Model choice and cost are not product rules.
