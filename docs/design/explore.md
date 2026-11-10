# Explore

**English** · [中文](explore.zh-CN.md)

**Status: implemented.** This file is the spec for the Explore page. If the code and this file disagree, this file wins until a pull request changes it. It is the next webui parity gap after scenes, not a new roadmap slice and not expert mode.

## What this is for

A beginner can paste a content ID, or a path that starts with `/ipfs/`, and see the one block it names: what kind of thing it is, and what it points at. Opening a row looks inside that next block. The page says "explore". It does not say "Merkle forest", and it does not draw a graph.

The browser still never talks to Kubo. It talks to the manager. The manager calls Kubo on loopback, and only the calls this page needs. The page does not receive a Kubo RPC address, and it does not fetch the gateway itself.

## What ipfs-webui actually ships

Checked against ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298` and `ipld-explorer-components` 8.1.3, the version in `ipfs-webui/package-lock.json:38474`.

The routes are `/explore` and `/explore*` (`ipfs-webui/src/bundles/routes.js:14-15`). `/explore` is the start page. Any longer path is the explorer (`ipfs-webui/src/explore/explore-page-renderer.jsx:25-29`).

The start page offers a pasted CID or a featured public dataset. The explorer then shows four things: a breadcrumb of the path, the node (kind, CID, size, data, and a table of links you can open), a decomposed CID, and a graph of the node and its links. The node can offer "View on Local Gateway" and "View on Public Gateway" (`ipfs-webui/src/explore/ExploreContainer.js:31-36`).

The blocks are fetched by a Helia node in the browser from the local gateway, as raw blocks, not as dag-json (`ipfs-webui/src/bundles/gateway.js:77-86`). Remote gateways are off unless the person turns them on (`ipfs-webui/src/bundles/ipfs-provider.js:328-332`). The header inspect box resolves an `/ipfs/` or `/ipns/` path with Kubo `resolve` and then opens the explorer on the resulting CID (`ipfs-webui/src/files/explore-form/files-explore-form.tsx:63-82`).

This page keeps the paste box, the breadcrumb, the kind of block, and the links. It does not keep the graph, the coloured hash lesson, the featured public datasets, the CAR upload, IPNS, or a public gateway link.

## Decisions

1. **One block at a time, through `dag/get`.** The manager calls `dag/resolve` when the target has a path, then `dag/get` with `output-codec=dag-json`. That codec is the command default (`core/commands/dag/dag.go:125`). `dag/get` resolves the path and encodes the node (`core/commands/dag/get.go:39-66`). `dag/resolve` returns the CID of the block the path lands in, and any remainder inside that block (`core/commands/dag/resolve.go:23-30`, and the text form in `test/sharness/t0053-dag.sh:378-391`).

   The webui does not call `dag/get`. It fetches raw blocks from the gateway because the browser decodes them. This panel does not put that decoder in the browser. It also does not turn on `Gateway.AllowCodecConversion`. That flag defaults to false, and a codec mismatch is `406` (`docs/config.md:1326-1347`). Asking the gateway for `?format=dag-json` would fail on a stock node. `dag/get` is the decoded dag-json of the same block. The changelog's directory example is that shape (`docs/changelogs/v0.18.md:221-229`).

2. **The target is a content ID or an `/ipfs/` path.** The same rules as a pin target. Surrounding whitespace is ignored. `/ipns/`, `ipfs://`, a URL, `..`, and an empty segment are rejected. A bare content ID is fetched as that block. A path is resolved first. The response `target` is the normalised input. `cid` is the block the path lands on. `focus` is the remainder inside that block, omitted when there is none. A UnixFS name usually has no remainder, because the name is a link to another block.

3. **A stopped node is 503.** When the cached node state is not `up`, the route returns **503** `{"error":"node_not_running","state":"<state>"}`. A dial that fails after the cache said `up` is the same 503 with `state` `down`. The deadline is 60 seconds. It is not a flag. A deadline is **504** `timeout`. The handler uses the request context, so a disconnected browser cancels the Kubo call.

4. **The RPC dial is loopback only.** The call reads `<repo>/config` and uses the API address only when it is an `http` loopback origin with a port. The client refuses any other dial, ignores `HTTP_PROXY`, and does not follow redirects. The response has no API URL, no port `5001`, and no multiaddr.

5. **The local link is copy-only.** When the gateway address is a loopback HTTP origin, the body includes `localGateway`, the same field Files uses. The page may copy `localGateway` plus the `/ipfs/` path of the block on screen. The toast says the link only works on this computer. The page does not navigate there, and it does not show the origin as a field. When the gateway is not loopback, `localGateway` is omitted. Explore still works, because the bytes come from the API.

6. **The kind is a short word.** `folder` is a UnixFS directory or HAMT shard. `file` is a UnixFS file. `symlink` is a UnixFS symlink. `bytes` is codec `raw`. `data` is dag-cbor, dag-json, json, or cbor. `block` is anything else, including a dag-pb node that is not UnixFS. The UnixFS type is the first protobuf field in the `Data` bytes. A directory in the changelog is the bytes `CAE` (`docs/changelogs/v0.18.md:221-224`), which is field 1 set to 1. Type 2 is a file, 4 is a symlink, 5 is a HAMT shard and is shown as a folder.

7. **Links are the block's own links, capped.** A dag-pb node uses `Links`: `Name`, `Hash`, `Tsize`. Other nodes contribute CID links of the form `{"/":"<cid>"}`. A bytes object `{"/":{"bytes":"..."}}` is not a link. At most 200 links are returned. `linksTruncated` is true when there are more. `links` is `[]` when there are none, never `null`. An empty name stays `""`. The page says "Unnamed part".

8. **A preview is small, or it is absent.** File, raw, and symlink bytes are `text` only when they are valid UTF-8, contain no NUL, and are at most 4096 bytes. Structured data is included as JSON only when that JSON is at most 8 KiB. A larger value sets `dataOmitted` and omits `data`. The dag-json body itself is refused over 4 MiB with **413** `too_large`. Binary is not sent as base64.

9. **The CID line is the part a beginner can use.** `version` is 0 or 1. `codec` is `raw`, `dag-pb`, `dag-cbor`, `dag-json`, `json`, `cbor`, or `unknown`. `hash` is `sha2-256` when the multihash code is `0x12` (the same code a version 0 CID must use, `internal/pins/cid.go:135`) and `identity` when the code is `0x00` (the inlined multihash in `ipfs-webui/test/e2e/explore.test.js:86-87`). Other hash codes omit `hash`. The page does not print the hex digest.

10. **Errors are short codes.** The body is `{"error":"<code>"}`. `node_not_running` also has `state`. Kubo's own message stays in the manager log.

11. **Session matches the other pages.** `GET /api/explore` needs a session. It does not need the CSRF header. `GET /health` stays open.

## API

| Method and path | Query | Success |
| --- | --- | --- |
| `GET /api/explore?target=` | content ID or `/ipfs/` path | **200** one block |

```json
{
  "target": "/ipfs/bafy.../notes",
  "cid": "bafkrei...",
  "version": 1,
  "codec": "dag-pb",
  "hash": "sha2-256",
  "kind": "file",
  "size": 5,
  "text": "hello",
  "links": [],
  "localGateway": "http://127.0.0.1:8081"
}
```

`size` is the UnixFS file size when that field is present. It is omitted for a folder whose size is 0, and whenever the node has no size. For raw bytes it is the length of those bytes. `focus`, `text`, `data`, `hash`, and `localGateway` are omitted when they do not apply. `data` is a JSON value, not a string. `links[].size` is the link's `Tsize` when Kubo sent one, including 0.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `invalid_target` | Empty, or not a content ID or `/ipfs/` path |
| 401 | `unauthorized` | No session |
| 404 | `not_found` | Kubo cannot resolve the path or the block |
| 413 | `too_large` | The dag-json is over 4 MiB |
| 502 | `kubo_error` | Kubo refused for another reason |
| 503 | `node_not_running` | Node state is not `up`, or the dial failed |
| 503 | `unavailable` | Config is not a loopback RPC, or the manager failed |
| 504 | `timeout` | The 60 second deadline elapsed |

## UI

Explore sits in the nav after Files: Node, Files, Explore, Peers, Pins, Scenes, Settings.

The page has a title, one sentence, and a closed disclosure that says a content ID names one block and that opening it does not change the node. Then a card: one field, and Explore as the only primary button. The hint says to paste a content ID or a path that starts with `/ipfs/`. An empty field does not call the manager. A value the page can already reject does not either.

Before a lookup, one quiet sentence says nothing is open yet. After a lookup, a second card shows the path as a breadcrumb, the kind in a heading, one sentence for that kind, a muted line for the codec and version, the content ID with Copy, the size when present, the text or the data when present, and the links. The last breadcrumb is the current block. Earlier crumbs load that prefix again. A named link with a single safe segment loads the current path plus that name. Any other link loads that link's content ID, so an unnamed piece of a file still opens.

The wide layout is a table: name, content ID shortened in the middle, size. The narrow layout, under 720px, is one button per link: the name, then the shortened content ID and the size. Names wrap. They are not cut off. The content ID in the card is the full value and wraps.

Loading is a skeleton and the sentence "Looking up that block." Not found, timeout, and a Kubo refusal stay on the page, under the field, with the value still there. A stopped, starting, stopping, or failed node replaces the card with the same empty state the other pages use, and one button back to the node. The manager being unreachable is that sentence plus Try again.

Copy is in `en` and `zh`. Tokens come from the design system. The column is `min(960px, 100%)`, the same width as Files and Pins.

Dev-only, `?explore=start` and `?explore=result` draw the two shots without a manager. The production bundle does not include them.

## Tests

Go: CID version, codec, and hash for a version 0 CID, a version 1 CID, and the inlined `bafkqac3imvwgy3zao5xxe3de`; dag-json for a folder, a file, raw bytes, a symlink, and a dag-cbor link; a path that resolves to another CID; a remainder kept as `focus`; a truncated link list; a body over 4 MiB; loopback refusal; HTTP session, 503, 400, 404, and a body with no port 5001.

Web: the path and link helpers, the page states in English, and the aligned locale keys.

## Out of scope

- A raw config editor, diagnostics, and the integration harness. Peers has its own spec.
- The graph, the hex digest, featured public datasets, CAR import, and IPNS.
- A public gateway link, or fetching any gateway from the browser.
- Turning on `Gateway.AllowCodecConversion`.
- Editing the block, or listing every link of a huge directory in one response.
