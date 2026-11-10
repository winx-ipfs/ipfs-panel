# ipfs-webui parity

**English** · [中文](webui-parity.zh-CN.md)

**Status: checklist.** This is not a spec for copying the webui. It records what [ipfs-webui](https://github.com/ipfs/ipfs-webui) v4.13.0 actually ships, and where each capability sits in this panel. The commit is `c51d980920d60e9d41f53f461886d6e1ea046298`, pinned in [REFERENCE-ipfs-stack](../REFERENCE-ipfs-stack.md).

The route table is `ipfs-webui/src/bundles/routes.js:13-28`. The nav is `ipfs-webui/src/navigation/NavBar.js:81-86`: Status, Files, Explore, Peers, Settings, Diagnostics. Pins and Welcome are routes that are not in that nav.

The panel keeps its own beginner tone ([Vision](../VISION.md)) and its own design system. Nothing below is a request to restyle this product as the webui.

## Where each capability sits

| Webui surface | What it does | This panel |
| --- | --- | --- |
| Status `/` and `/status*` | Connected sentence, peer id, agent version, repo size, peer count, bandwidth chart and rates | **Shipped**, plus a small addition in this slice. The Node page already has the sentence, version, and peer id ([Node lifecycle](../DESIGN-node-lifecycle.md)). This slice adds the peer count and the in/out rates, and hides both while the node is stopped. Repo size, the chart, and the analytics banner stay deferred. |
| Files `/files*`, `/ipfs*`, `/ipns*` | MFS browser: upload, folders, download, rename, delete, share link, pin, inspect, CAR import, add by path | **Shipped** for the beginner path ([Files](files.md)): add, download, rename, move, copy, delete, local link, and pin from a row. Inspect, CAR import, add-by-path, and browsing `/ipfs` or `/ipns` paths are deferred. |
| Explore `/explore*` | IPLD explorer over the gateway | **Shipped** for the beginner path ([Explore](explore.md)): paste a content ID or an `/ipfs/` path, see the block, and open a link. The manager calls `dag/get` on loopback. The graph, the hex digest, featured public datasets, CAR import, and IPNS stay out. |
| Peers `/peers` | World map, peer table, manual connect | **Shipped** for the beginner path ([Peers](peers.md)): a table of who is connected, and a one-time dial. No map. Connected peer addresses stay off the browser. Permanent peering stays deferred with the raw editor. |
| Pins `/pins*` | In-progress pin jobs. Not in the nav | **Shipped**, and broader than this route. [Pins](pins.md) lists what the node keeps, adds a pin, and cancels a job. |
| Settings `/settings*` | Kubo RPC address, local gateway URL, share-link shape, IPNS keys, remote pinning services, retrieval check URL, language, analytics, CLI tutor, raw JSON editor | **Partly shipped.** Language and theme are in Settings. This slice's scenes replace the need to edit JSON for the four setups in the vision. A raw config editor, IPNS keys, remote pinning services, and an analytics toggle are deferred. The panel does not add a form for the Kubo RPC address. |
| Settings analytics `/settings/analytics` | Analytics opt-in | **Deferred.** |
| Diagnostics `/diagnostics*` | Logs, retrieval check, DHT provide | **Deferred.** Expert mode, slice 6. |
| Welcome `/welcome` | What IPFS and the webui are | **Shipped** in a smaller form. The Node page explains what a node is. There is no second product tour. |
| Blank `/blank` | Empty route used by the shell | **Not applicable.** |

## What this slice takes from Status

Status shows a peer count from `swarm peers` (`ipfs-webui/src/status/StatusConnected.js:6`, counted in `ipfs-webui/src/bundles/peers.js:27-32`) and bandwidth from `stats bw` (`ipfs-webui/src/bundles/node-bandwidth.js:10`). Both are real Kubo RPC calls. The chart, per-peer bandwidth, and country chart are not copied.

The manager calls the same two RPC commands and keeps only a count and two rates. See [Node lifecycle](../DESIGN-node-lifecycle.md) for the response shape. Addresses from `swarm peers` are discarded before the browser sees the body (`core/commands/swarm.go:433` is the address field that is not forwarded).

## What Explore keeps

The webui fetches raw blocks from the local gateway and decodes them in the browser (`ipfs-webui/src/bundles/gateway.js:77-86`). Codec conversion on the gateway is off by default, so this panel does not ask the gateway for dag-json. The manager calls `dag/get` instead. The page, the path, and the response are in [Explore](explore.md).

## What Peers keeps

The webui page is a map, a table, and a connect dialog (`ipfs-webui/src/peers/PeersPage.js:39-46`). The map plots public IPs from peer multiaddrs (`ipfs-webui/src/bundles/peer-locations.js:335-337`). Those addresses are the field this panel already discards. This slice keeps the table and a one-time `swarm/connect`. It does not keep the map, the multiaddr, or the permanent `Peering.Peers` write (`ipfs-webui/src/bundles/peers.js:40-61`). The page, the path, and the response are in [Peers](peers.md).

## What stays out on purpose

- A raw config editor. It is the remaining webui gap. It still waits.
- Any screen that gives the browser a Kubo RPC address. The webui settings page has one (`ipfs-webui/src/settings/SettingsPage.js:60`). This panel does not.
