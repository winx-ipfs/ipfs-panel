# Peers

**English** · [中文](peers.zh-CN.md)

**Status: implemented.** This file is the spec for the Peers page. If the code and this file disagree, this file wins until a pull request changes it. It is the next webui parity gap after Explore, not a new roadmap slice and not expert mode.

## What this is for

A beginner can see which other nodes this one is connected to, as a list rather than a count, and can dial one more by pasting an address. The page says "peers". It does not draw a map, and it does not save the address into Kubo's config.

The browser still never talks to Kubo. It talks to the manager. The manager calls Kubo on loopback, and only the calls this page needs. The page does not receive a Kubo RPC address. Addresses of peers that are already connected stay off the browser, the same decision the Node page already made for the count.

## What ipfs-webui actually ships

Checked against ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`.

The route is `/peers` (`ipfs-webui/src/bundles/routes.js:20`). The nav item sits after Explore (`ipfs-webui/src/navigation/NavBar.js:84`).

The page is three pieces (`ipfs-webui/src/peers/PeersPage.js:39-46`): an "Add connection" button, a world map, and a peer table. A CLI tutor line offers `ipfs swarm connect`. A slash key focuses the filter. There is a product tour.

The list comes from `swarm.peers` with `verbose`, `identify`, and a timeout (`ipfs-webui/src/bundles/peers.js:9`). On this route it refreshes about every 5 seconds (`ipfs-webui/src/bundles/peers.js:92`). Each row is one connection. The row keeps the peer id, the multiaddr, a transport string parsed from that multiaddr, latency, direction, open stream names, agent version, and a location (`ipfs-webui/src/bundles/peer-locations.js:136-166`). The table columns are location, latency, peer id, connection, agent version, and open streams (`ipfs-webui/src/peers/PeersTable/PeersTable.js:210-216`). The connection cell copies `address + /p2p/ + peerId` (`ipfs-webui/src/peers/PeersTable/PeersTable.js:94-110`).

The map is not decorative in the webui. It looks up a public IP taken from the peer multiaddr (`ipfs-webui/src/bundles/peer-locations.js:335-337`) through `ipfs-geoip` on a gateway (`ipfs-webui/src/bundles/peer-locations.js:367`), then plots those coordinates (`ipfs-webui/src/peers/WorldMap/WorldMap.js:85-88`, `ipfs-webui/src/peers/WorldMap/WorldMap.js:154-160`). Private addresses are labelled "Local Network" instead.

Add connection checks that the text is a multiaddr matching a p2p, circuit, or WebRTC shape (`ipfs-webui/src/peers/AddConnection/AddConnection.js:15-21`). It then calls `swarm.connect`. The "permanent" box defaults to on (`ipfs-webui/src/peers/AddConnection/AddConnection.js:29`, `ipfs-webui/src/peers/AddConnection/AddConnection.js:127-130`) and writes `Peering.Peers` with `config.set` (`ipfs-webui/src/bundles/peers.js:40-61`).

This page keeps the list and the one-time dial. It does not keep the map, the multiaddr, the location, the transport string, the stream names, the CLI tutor, the tour, or the permanent checkbox.

## Decisions

1. **No map.** A faithful map needs the peer's IP, and that IP is the `Addr` field this panel already discards (`core/commands/swarm.go:433`). Guessing a country from nothing would invent a location. This slice does not call `ipfs-geoip` and does not ship a map.

2. **One row per peer id.** `swarm/peers` returns one object per open connection (`core/commands/swarm.go:265-308`). The Node page count is the number of distinct `Peer` values (`internal/activity/activity.go:63-70`). This page uses the same distinct count, so the number on Node and `total` here can agree. Two connections to one peer become one row. `latencyMs` is the smallest parsed latency. `direction` is `in` when every known connection is inbound, `out` when every one is outbound, and `both` when the peer has both. `agent` is the first non-empty agent version.

3. **What a row may contain.** The manager calls `swarm/peers` with `latency=true`, `direction=true`, and `identify=true`. Those flags fill latency, direction, and identify (`core/commands/swarm.go:271-306`). `verbose` is left off so the response is not also filled with streams. The browser receives `peerId`, and when present `direction`, `latencyMs`, and `agent`. It does not receive `Addr`, `Muxer`, `Streams`, or any field of `Identify` except `AgentVersion`. Identify also carries addresses and a public key (`core/commands/id.go:29-35`, copied in `core/commands/swarm.go:457-476`). Those stay in the manager process and are not logged.

4. **Latency and direction are numbers the page can say.** Kubo sends latency as a Go duration string, or `n/a` when the duration is 0 (`core/commands/swarm.go:282-285`). `n/a` and anything `time.ParseDuration` rejects omit `latencyMs`. A positive duration becomes whole milliseconds, rounded half up. A duration under 0.5 ms becomes `0`, and the page says "< 1 ms". Direction is `inet.Direction` (`core/commands/swarm.go:436`). Kubo prints inbound and outbound (`core/commands/swarm.go:497-505`). The webui treats JSON `1` as inbound and `2` as outbound (`ipfs-webui/src/peers/PeersTable/PeersTable.js:225-236`). Checked against go-libp2p v0.50.0, the version in `go.mod:54`: those constants are 1 and 2. Any other value is ignored for that connection. Agent version is trimmed, control and format characters are dropped, and it is cut at 128 runes, which is Kubo's own cap (`core/commands/cmdutils/sanitize.go:8`, `core/commands/cmdutils/sanitize.go:17-46`). An empty result omits `agent`.

5. **The list is capped.** Rows are ordered by latency, unknowns last, then by peer id. At most 500 rows are returned. `total` is the distinct count before the cap. `truncated` is true when `total` is greater than the number of rows. `peers` is `[]` when there are none, never `null`. The Kubo body is refused over 8 MiB with **413** `too_large`. The deadline for the list is 20 seconds.

6. **A stopped node is 503.** When the cached node state is not `up`, `GET /api/peers` and `POST /api/peers/connect` return **503** `{"error":"node_not_running","state":"<state>"}`. A dial that fails after the cache said `up` is the same 503 with `state` `down`. The handler uses the request context, so a disconnected browser cancels the Kubo call.

7. **The RPC dial is loopback only.** The call reads `<repo>/config` and uses the API address only when it is an `http` loopback origin with a port. The client refuses any other dial, ignores `HTTP_PROXY`, and does not follow redirects. The response has no API URL, no port `5001`, and no multiaddr.

8. **Connect dials once.** `POST /api/peers/connect` takes one JSON object, `{"address":"<multiaddr>"}`. The manager calls `swarm/connect` with that one argument (`core/commands/swarm.go:641-686`). It does not call `config.set`, and it does not write `Peering.Peers`. That checkbox is config editing, and it waits with the raw editor. Success is **200** `{"peerId":"<id>"}` using the peer id parsed from the address, not Kubo's `Strings` line. Kubo's message stays in the manager log. A non-200 from Kubo, after the address passed the rules below, is **502** `connect_failed`. The deadline is 30 seconds. A deadline is **504** `timeout`.

9. **The address is one multiaddr that already names the peer.** Surrounding whitespace is ignored. Empty is rejected. Longer than 2048 bytes is rejected. A URL, a bare peer id, a space, a backslash, `?`, `#`, and `@` are rejected. The value must start with `/` and end with `/p2p/<peer-id>` or `/ipfs/<peer-id>`. The peer id is one of the forms `peer.Decode` accepts in go-libp2p v0.50.0: a `Qm…` sha2-256 peer id (34-byte multihash `0x12` / `0x20`), a base58 multihash that starts with `1` and is identity (`0x00`) or sha2-256, or a CIDv1 whose codec is libp2p-key (`0x72`) in base32 (`b`), base58btc (`z`), or base16 (`f`). Components before the final peer id are only:

   | Component | Value |
   | --- | --- |
   | `ip4` | dotted IPv4 |
   | `ip6` | an IPv6 address, no zone |
   | `tcp`, `udp` | a port from 1 to 65535, no leading zero |
   | `dns`, `dns4`, `dns6`, `dnsaddr`, `sni` | a hostname |
   | `certhash` | one token, no slash |
   | `p2p`, `ipfs` | a peer id, which may also appear before the end |
   | `quic`, `quic-v1`, `webtransport`, `webrtc`, `webrtc-direct`, `tls`, `ws`, `wss`, `p2p-circuit` | no value |

   `/dnsaddr/…` is not resolved in the manager. An address that does not already end in a peer id is rejected, including a bare `/dnsaddr/` name. Kubo would try to resolve that (`core/commands/swarm.go:788-810`). This page does not. An address the rules reject is **400** `invalid_address` and is not sent to Kubo.

10. **Errors are short codes.** The body is `{"error":"<code>"}`. `node_not_running` also has `state`.

11. **Session matches the other pages.** `GET /api/peers` needs a session. It does not need the CSRF header. `POST /api/peers/connect` needs the session and the CSRF header. The body is `application/json`. A missing or other type is **415**. An empty body, a second JSON value, or an unknown key is **400** `bad_request`. `GET /health` stays open.

## API

| Method and path | Body | Success |
| --- | --- | --- |
| `GET /api/peers` | none | **200** the list |
| `POST /api/peers/connect` | `{"address":"/ip4/203.0.113.10/tcp/4001/p2p/12D3Koo…"}` | **200** `{"peerId":"12D3Koo…"}` |

```json
{
  "total": 2,
  "peers": [
    {
      "peerId": "12D3KooWExample",
      "direction": "out",
      "latencyMs": 18,
      "agent": "kubo/0.43.0"
    }
  ]
}
```

`truncated` is omitted when it is false. `direction`, `latencyMs`, and `agent` are omitted when they do not apply. `direction` is `in`, `out`, or `both`.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `invalid_address` | The address fails the rules in decision 9 |
| 400 | `bad_request` | JSON shape is wrong |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 413 | `too_large` | The Kubo peers body is over 8 MiB |
| 415 | `unsupported_media_type` | POST is not JSON |
| 502 | `connect_failed` | Kubo refused the dial |
| 502 | `kubo_error` | Kubo refused the list for another reason |
| 503 | `node_not_running` | Node state is not `up`, or the dial failed |
| 503 | `unavailable` | Config is not a loopback RPC, or the manager failed |
| 504 | `timeout` | The deadline elapsed |

## UI

Peers sits in the nav after Explore: Node, Files, Explore, Peers, Pins, Scenes, Settings.

The page has a title, one sentence, and a closed disclosure. The disclosure says a peer is another IPFS node this one is talking to, that a pasted address is dialled once, and that the page neither shows other nodes' addresses nor saves the address.

When the node is `up`, a card holds one field and Connect as the only primary button. The label is "Peer address". The hint says the address needs to end in `/p2p/` and a peer id. An empty field does not call the manager. A value the page can already reject does not either, and the field explains why. Enter submits. While the dial is in flight the button says "Connecting…" and stays disabled. Success clears the field and shows the toast "Connected." Failure stays under the field: the address was refused, the dial failed, or it took too long.

The list sits under that card once there is at least one peer. Zero peers, with an empty filter, keeps the sentence "No one is connected yet. Connections show up here on their own, or you can add one above." in the connect card. The first load is a skeleton and the sentence "Loading peers." A later refresh keeps the rows on screen. The summary is "{count} connected". When the filter is in use, the summary counts the rows that match. The filter matches a peer id, or an agent version ignoring case. A query that starts with `/p2p/` or `/ipfs/` has that prefix stripped once before the peer id is compared. Escape clears the filter. No match says "No peer matches that." When `truncated` is set, one sentence says "Showing {shown} of {total}." The filter only sees the rows the manager returned.

The wide layout, from 720px, is a table: peer, direction, latency, software. The peer id is the full value and wraps. It is not cut off in the middle. A copy button on the row writes the peer id. Direction is "They connected", "We connected", or "Both ways". Latency is "< 1 ms", "{n} ms", or "{n} s" with one decimal from 1000 ms up. Missing latency or software is an em dash, not the word "unknown".

The narrow layout, under 720px, is one card per peer: the wrapping peer id and the copy button, then direction, latency, and software, each with its label.

The page refreshes every 5 seconds while it is showing a list and the node is up. A stopped, starting, stopping, or failed node replaces the card and the list with the same empty state the other pages use, and one button back to the node. The manager being unreachable is that sentence plus Try again.

On the Node page, the peer count links here when the count is shown. The count itself is unchanged.

Copy is in `en` and `zh`. Tokens come from the design system. The column is `min(960px, 100%)`, the same width as Files, Explore, and Pins.

Dev-only, `?peers=down`, `?peers=empty`, and `?peers=list` draw those shots without a manager. The production bundle does not include them.

## Tests

Go: distinct peers collapse to one row; the lowest latency wins; mixed direction is `both`; `n/a` omits latency; agent text is capped and stripped; the list JSON has no `Addr`, no `/ip4/`, no public key, and no port `5001`; the 500 cap sets `truncated` and keeps `total`; address rules accept an IPv4 TCP `/p2p/` address, a QUIC address, a `/dnsaddr/…/p2p/` address, and a relay that uses `p2p-circuit`, and reject a URL, a bare peer id, a missing peer id, and a bad port; loopback refusal; HTTP session, CSRF, 503 with `state`, 400, and a connect success that returns only `peerId`.

Web: the address helper, the page states in English (stopped, empty, list, a rejected address that does not call the manager), and the aligned locale keys.

## Out of scope

- A world map, a country, a flag, or any location derived from an IP.
- Showing or copying a connected peer's multiaddr, transport string, public key, or listen addresses.
- Open stream names, per-peer bandwidth, and the CLI tutor.
- Permanent peering, `Peering.Peers`, `swarm peering`, and disconnect. The raw config editor still waits.
- Resolving a `/dnsaddr/` name that does not already end in a peer id.
- Diagnostics, and the integration harness.
