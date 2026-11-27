# Roadmap

**English** · [中文](ROADMAP.zh-CN.md)

The UI stack is Vue 3, Vite, TypeScript, Vue Router, Pinia, and vue-i18n, in `apps/web`. The API is Go in `services/manager`, standard library for now. Kubo is not vendored here. The manager will call a local Kubo later. It will not publish Kubo's RPC.

## How a slice works

Each slice is vertical. The screen, the manager route, and the Kubo call (when that slice has one) land together. A slice is done when a person can do that job in English and in 中文 without reading Kubo docs for it.

We do not build the whole backend, then the whole UI.

## Where this repo is

The tree you can clone today has finished slices 1 through 5. Slice 3's files page, slice 4's pins page, and slice 5's scenes page are in. The Explore and Peers pages are in, and so are the first two pieces of slice 6: the Expert raw config editor and the Logs page. The agreed upcoming sequence is [the plan](PLAN.md).

What is here:

- a panel shell with a Node card and a Files page
- English and 中文, with the language switch saved in the browser
- one admin, created on first run, with a session on the manager
- `GET /health` on the manager, loopback by default, and still open without a session
- start, stop, and restart for one local Kubo, with a one-sentence status, behind that session
- add, download, rename, move, copy, and delete files on that node, through its mutable file system
- see what the node is pinning, pin a content ID or a file, and unpin, without the command line
- pick one of four scenes, see what it will change, apply it, and revert it, without opening Kubo's JSON
- look inside a content ID: what the block is, and what it points at
- see who the node is connected to, dial one peer by an address, and keep chosen peers connected
- install the pinned Kubo when the machine has none, and upgrade it inside a supported range
- open the manager from another machine, with TLS or an explicit cleartext flag, while Kubo's API stays on loopback
- read and replace the Kubo config as JSON, from Settings, marked Expert
- read what the panel and the node are writing, follow it, and set one log level

What is not here, in this order. A release is next, and only when the user asks. The rest are later. The detail is [the plan](PLAN.md).

- v0.2.0
- the desktop app
- IPNS keys
- remote pinning
- diagnostics
- a second node

## 1. Shell, auth, and languages

The shell, the language switch, and login are in place. The spec is [Auth](design/auth.md).

An anonymous browser cannot call the manager's `/api` routes. The browser still never receives a Kubo RPC address to call itself.

Done when you can sign in, the shell renders in both languages, and a request without a session is rejected. That is the current tree.

## 2. Start, stop, and health

Show whether Kubo is up. Start it. Stop it. Restart it. The status is a sentence (up, down, starting, stopping, failed), not a dump of the RPC response.

The spec is [Node lifecycle](DESIGN-node-lifecycle.md). The manager and the Node card implement it. The routes require the session from slice 1.

Done when you can change whether the local node is running from the panel, and the panel tells the truth when it fails.

## 3. Files: add, pin, and MFS

Add a file, see the content ID, and find it again in the mutable file system. That part is in. The spec is [Files](design/files.md). Pinning is not part of the file browser. It is slice 4, and it is in.

Done when a beginner can put a file on the node and open it again from the panel.

## 4. Pins

A page for what the node is holding, and a way to unpin. Pin bookkeeping is not hidden inside the file browser. The spec is [Pins](design/pins.md).

Done when you can answer "what is pinned?" and remove one pin without the CLI. That is the current tree.

## 5. Scene settings

Pick a scene. The panel writes the Kubo config for that scene and says, in one sentence per language, what it changed. The spec is [Scenes](design/scenes.md).

Done when a common setup does not mean opening the JSON file. That is the current tree: a quiet node, sharing a folder, pinning a set of content, and a read-only public gateway.

## Explore

Look inside one content ID. The spec is [Explore](design/explore.md). It is webui parity, not a new numbered slice.

Done when a beginner can paste a content ID or an `/ipfs/` path, see the block, and open a link, in English and in 中文. That is the current tree.

## Peers

See who this node is connected to, and dial one address once. The spec is [Peers](design/peers.md). It is webui parity, not a new numbered slice.

Done when a beginner can open the list, in English and in 中文, and connect by an address the page allows, without a map and without seeing another node's address on that table. That is the current tree.

## Permanent peering

Keep chosen peers connected, including after the link drops and after a restart. The spec is [Permanent peering](design/peering.md). It lives on the Peers page.

Done when a beginner can add and remove always-connect peers without editing JSON, and the page says whether a restart or an apply is needed. That is the current tree.

## Install and upgrade Kubo

A machine with no Kubo can install the pinned release from the Node page. A newer release inside the supported range can be upgraded, and a failure before the repo version changes puts the previous binary back. A binary the operator pointed the manager at is left alone. The spec is [Install and upgrade Kubo](design/kubo-install.md).

Done when a beginner can leave the page with a working node, and can later see that a newer Kubo exists and upgrade it, in English and in 中文. That is the current tree.

## Remote control

Open the same panel from a browser on another machine. The manager is what listens. Kubo's API does not. The spec is [Remote control](design/remote.md).

Done when the owner of a home server or a VPS can sign in from another browser, the listen stays loopback unless they opt in, and a remote listen without TLS does not start unless they pass `-insecure`. That is the current tree.

## 6. Expert mode

The raw config editor is in. It is Settings, Advanced, marked Expert. The spec is [Config editor](design/config-editor.md). Logs are in. The spec is [Logs](design/logs.md).

Slice 6's expert half has landed, permanent peering has landed, installing Kubo has landed, and remote control of the manager has landed. The sequence after that lives in [the plan](PLAN.md). v0.2.0 waits for an explicit release request. The next feature after the release is the desktop app.

## Not in the early slices

- Host-panel work: sites, databases, containers, OS admin
- A public Kubo RPC, or a tunnel to it
- Rewriting Kubo
- A hosted public gateway as a product
- Multi-node before one node works
