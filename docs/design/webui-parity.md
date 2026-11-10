# ipfs-webui parity

**English** · [中文](webui-parity.zh-CN.md)

**Status: checklist for slice 5.** This is not a spec for copying the webui. It records what [ipfs-webui](https://github.com/ipfs/ipfs-webui) v4.13.0 actually ships, and where each capability sits in this panel. The commit is `c51d980920d60e9d41f53f461886d6e1ea046298`, pinned in [REFERENCE-ipfs-stack](../REFERENCE-ipfs-stack.md).

The route table is `ipfs-webui/src/bundles/routes.js:13-28`. The nav is `ipfs-webui/src/navigation/NavBar.js:81-86`: Status, Files, Explore, Peers, Settings, Diagnostics. Pins and Welcome are routes that are not in that nav.

The panel keeps its own beginner tone ([Vision](../VISION.md)) and its own design system. Nothing below is a request to restyle this product as the webui.

## Where each capability sits

| Webui surface | What it does | This panel |
| --- | --- | --- |
| Status `/` and `/status*` | Connected sentence, peer id, agent version, repo size, peer count, bandwidth chart and rates | **Shipped**, plus a small addition in this slice. The Node page already has the sentence, version, and peer id ([Node lifecycle](../DESIGN-node-lifecycle.md)). This slice adds the peer count and the in/out rates, and hides both while the node is stopped. Repo size, the chart, and the analytics banner stay deferred. |
| Files `/files*`, `/ipfs*`, `/ipns*` | MFS browser: upload, folders, download, rename, delete, share link, pin, inspect, CAR import, add by path | **Shipped** for the beginner path ([Files](files.md)): add, download, rename, move, copy, delete, local link, and pin from a row. Inspect, CAR import, add-by-path, and browsing `/ipfs` or `/ipns` paths are deferred. |
| Explore `/explore*` | IPLD explorer over the gateway | **Deferred.** Not in this pull request. Recorded as the next webui gap in [Handoff](../HANDOFF.md). |
| Peers `/peers` | World map, peer table, manual connect | **Deferred.** A peer map and a connect box are not in this pull request. The Node page shows only the count. |
| Pins `/pins*` | In-progress pin jobs. Not in the nav | **Shipped**, and broader than this route. [Pins](pins.md) lists what the node keeps, adds a pin, and cancels a job. |
| Settings `/settings*` | Kubo RPC address, local gateway URL, share-link shape, IPNS keys, remote pinning services, retrieval check URL, language, analytics, CLI tutor, raw JSON editor | **Partly shipped.** Language and theme are in Settings. This slice's scenes replace the need to edit JSON for the four setups in the vision. A raw config editor, IPNS keys, remote pinning services, and an analytics toggle are deferred. The panel does not add a form for the Kubo RPC address. |
| Settings analytics `/settings/analytics` | Analytics opt-in | **Deferred.** |
| Diagnostics `/diagnostics*` | Logs, retrieval check, DHT provide | **Deferred.** Expert mode, slice 6. |
| Welcome `/welcome` | What IPFS and the webui are | **Shipped** in a smaller form. The Node page explains what a node is. There is no second product tour. |
| Blank `/blank` | Empty route used by the shell | **Not applicable.** |

## What this slice takes from Status

Status shows a peer count from `swarm peers` (`ipfs-webui/src/status/StatusConnected.js:6`, counted in `ipfs-webui/src/bundles/peers.js:27-32`) and bandwidth from `stats bw` (`ipfs-webui/src/bundles/node-bandwidth.js:10`). Both are real Kubo RPC calls. The chart, per-peer bandwidth, and country chart are not copied.

The manager calls the same two RPC commands and keeps only a count and two rates. See [Node lifecycle](../DESIGN-node-lifecycle.md) for the response shape. Addresses from `swarm peers` are discarded before the browser sees the body (`core/commands/swarm.go:433` is the address field that is not forwarded).

## What stays out on purpose

- Explore, the peers map, and a raw config editor. They are the next webui gaps, not this slice.
- Any screen that gives the browser a Kubo RPC address. The webui settings page has one (`ipfs-webui/src/settings/SettingsPage.js:60`). This panel does not.
