# Permanent peering

**English** · [中文](peering.zh-CN.md)

**Status: implemented.** This file is the spec for keeping peers connected. If the code and this file disagree, this file wins until a pull request changes it. The one-time dial stays in [Peers](peers.md). This file is the missing piece of that page.

## What this is for

A beginner can choose peers this node should stay connected to. If the link drops, the node dials again. The choice survives a restart. The person does not edit JSON.

The entrance is the existing Peers page: Keep connected on a connected row, and a kept list where an address can be added or a peer removed. An expert is not blocked. The raw config editor can still change `Peering.Peers`. This page notices when the saved list and the running list are not the same, and it can apply the saved list without a restart.

The browser still never talks to Kubo. It talks to the manager. The manager calls Kubo on loopback. The page does not receive a Kubo RPC address.

## What Kubo actually does

Checked against kubo `833e04a240d8c533509ddaaa3c202b39b0bb3024`.

`Peering.Peers` is the saved list. Each entry is a peer id and the addresses to dial (`config/peering.go:7`, `docs/config.md:2974`, `docs/config.md:2996`). An empty address list means Kubo asks the Amino DHT. The daemon copies that list into the peering service once, when the node is built (`core/node/groups.go:368`, `core/node/peering.go:35`). A later edit of the file does not update the running service.

`ipfs swarm peering` changes the running service only. The command says the changes are not saved to the config (`core/commands/swarm.go:93`). `add` takes multiaddrs and calls `AddPeer` (`core/commands/swarm.go:139`). `ls` returns the peers the service is keeping (`core/commands/swarm.go:172`). `rm` takes peer ids and calls `RemovePeer` (`core/commands/swarm.go:218`). The service reconnects with a backoff. That is the "if it drops, dial again" behaviour, and it starts as soon as `add` returns. It does not wait for a restart.

`config/replace` of an array replaces that array (`repo/common/common.go:88`). Sending a new `Peering.Peers` array replaces the saved list. It does not, by itself, call `AddPeer`.

So both calls are required. The live call keeps the peer now. The config write keeps the peer after the next start. A restart is not required when both succeed.

## What ipfs-webui does

Checked against ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`.

The permanent box defaults to on, as [Peers](peers.md) already recorded. The handler writes `Peering.Peers` with `config.set` and does not call `swarm.peering` (`ipfs-webui/src/bundles/peers.js:42`, `ipfs-webui/src/bundles/peers.js:47`, `ipfs-webui/src/bundles/peers.js:60`). The comment in that file is waiting on a Kubo pull request that, at this pin, has not changed the "not saved to the config" rule above. On this pin the webui's permanent box is saved for the next start. The running service does not hear about it until then.

This page does the live call as well, so the beginner does not have to restart to start the reconnects. It still writes the config, which the webui does and the live call does not.

## Decisions

1. **One list, two stores.** The saved list is `Peering.Peers`. The running list is `swarm peering ls`. The page shows the saved list. `sync` says whether the running list matches it.

2. **Add and remove do both.** An add validates the address or the peer id, writes `Peering.Peers` through the same replace path as the raw editor (backup, `Identity.PrivKey` stripped, `Addresses.API` locked, the start-time address checks), then calls `swarm/peering/add`. A remove deletes that peer id from the saved list, then calls `swarm/peering/rm`. The connected-peers table still does not write `Peering.Peers`. `POST /api/peers/connect` is unchanged.

3. **No restart when both succeed.** `sync` is `ok`. The page does not ask for a restart.

4. **Apply when they differ.** `sync` is `apply` when the saved ids or addresses are not the running set. That happens after a raw edit, or after a CLI `swarm peering` that was not saved. Apply adds and removes on the running service until it matches the saved list. It does not rewrite the config and it does not restart the daemon. The sentence also says a restart loads the saved list, because the daemon reads `Peering.Peers` at start. Apply is the button. Restart is the other way, and it is the restart the panel already has.

5. **Restart when the live call failed.** If the config write succeeded and `swarm peering` did not, `sync` is `restart`. The saved list is what the next start will load. The page says so, offers the restart, and offers Apply as a retry. The manager does not restart by itself.

6. **A stopped node is 503.** `GET` and every `POST` return **503** `{"error":"node_not_running","state":"<state>"}` when the cached state is not `up`. The read path is `config/show` and `swarm peering`, and both need the daemon. The page uses the same blocked state as the rest of Peers.

7. **What the browser may see.** The connected table still receives no peer addresses. The kept list receives the addresses stored for those peers, because the person chose them and has to be able to tell the rows apart. Keeping a connected row does the lookup in the manager: `swarm/peers` has the address, the browser sent only the peer id, and the stored addresses come back on the kept list only. They are not added to the connected table and they are not written into the audit line.

8. **Addresses and peer ids use the Peers rules.** A pasted address is `peers.ParseAddress`. The peer id is the last `/p2p/` or `/ipfs/` component. The stored address is that multiaddr with the final peer id removed. A bare `/p2p/<id>` is stored with an empty address list, which is the DHT case in `docs/config.md:2996`. A peer id by itself, from a row, must be one `peers.CanonicalPeerID` accepts. The manager then reads that peer's current swarm addresses and stores the ones that pass the same address rules. None pass, and the id is valid: the address list is empty. The peer is not in the swarm: **409** `not_connected`.

9. **One row per peer id.** Adding an address for an id that is already saved appends it. The same address again changes nothing. The cap is 64 peers and 8 addresses each. That is a panel cap, not Kubo's. Past it, add returns **409** `too_many` and writes nothing. A list the raw editor already made larger is still shown, and remove still works. Entries that are not objects are left in the file and are not shown. Extra fields on an entry are kept when this page rewrites the array.

10. **State on a row.** `connected` means the peer id is in `swarm/peers`. `reconnecting` means the running peering service has the id and the swarm does not. `pending` means the id is saved and the running service does not have it yet.

11. **Backup and scenes.** A saved-list change writes the previous redacted document to `<data>/config-backups/`, the same files the raw editor restores. The private key is not in that file. `Peering.Peers` is not a scene key, so a change here does not mark a scene as left. Scene apply does not touch this list.

12. **Audit.** `peering_add`, `peering_remove`, and `peering_apply` go to the manager log with the result, the username, the client IP, and the peer id when there is one. The line has no address, no config document, and no private key.

13. **Errors are short codes.** The body is `{"error":"<code>"}`. `node_not_running` also has `state`.

## API

Session and CSRF match [Auth](auth.md) and [Peers](peers.md). GET needs a session. POST needs `Content-Type: application/json` and `X-CSRF-Token`. Unknown JSON fields are **400** `bad_request`. A second JSON value is **400** `bad_request`.

| Method and path | Body | Success |
| --- | --- | --- |
| `GET /api/peers/permanent` | none | **200** the list |
| `POST /api/peers/permanent` | `{"address":"/ip4/203.0.113.10/tcp/4001/p2p/12D3Koo…"}` or `{"peerId":"12D3Koo…"}` | **200** the list |
| `POST /api/peers/permanent/remove` | `{"peerId":"12D3Koo…"}` | **200** the list |
| `POST /api/peers/permanent/apply` | `{}` or an empty body | **200** the list |

Exactly one of `address` and `peerId` on add. Remove of an unknown id is **200** and still drops the id from the running service if it is there.

```json
{
  "sync": "ok",
  "peers": [
    {
      "peerId": "12D3KooWExample",
      "addrs": ["/ip4/203.0.113.10/tcp/4001"],
      "state": "reconnecting"
    }
  ]
}
```

`peers` is `[]` when there are none, never `null`. `addrs` is `[]` when the DHT will be asked, never `null`. `sync` is `ok`, `apply`, or `restart`. `state` is `connected`, `reconnecting`, or `pending`.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `invalid_address` | The address fails the Peers rules |
| 400 | `invalid_peer` | The peer id is not one this page accepts |
| 400 | `bad_request` | JSON shape is wrong |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 409 | `not_connected` | Keep-from-row, and that id is not in the swarm |
| 409 | `too_many` | The cap would be exceeded |
| 409 | `api_locked`, `identity_locked`, and the other editor locks | The replace path refused, which this page does not try to trigger |
| 413 | `too_large` | A Kubo body is over its cap |
| 415 | `unsupported_media_type` | POST is not JSON |
| 502 | `kubo_error` | Kubo refused a read |
| 503 | `node_not_running` | Node state is not `up` |
| 503 | `unavailable` | Config is not a loopback RPC, or the manager failed |
| 504 | `timeout` | The deadline elapsed |

A live `swarm peering` failure after a successful save is **200** with `sync` `restart`, not a 502. The save happened. The page has to say what to do next.

## UI

The page is still Peers. The title stays Peers. The sentence under it says the job includes keeping peers. The disclosure says Connect once dials and does not save, Keep connected stores the peer and the node redials if the link drops, the connected table does not show addresses, and the kept list shows the addresses that were saved.

When the node is up, one card holds the toolbar and both lists. The toolbar is the address field, Keep connected as the primary button, Connect once as the quiet button, and the filter when someone is connected. Enter keeps. An empty field does not call the manager. A value the page can already reject does not either.

The kept list is under the toolbar. Its heading is "Kept connected". Zero rows say "No peers are kept yet." A row is the peer id, the saved addresses, the state word, and Remove. The state words are "Connected", "Reconnecting", and "Waiting". The connected table is unchanged apart from a Keep action on each row. The action says "Kept" and does nothing when that id is already saved.

`sync` `apply` shows one line: "The saved peers are not what this node is keeping yet." Apply is the button. A second line says restarting the node loads the saved list too, with the existing restart.

`sync` `restart` shows: "The saved peers will be kept after the node restarts." Restart is the button. Apply is there as Try again.

While a keep is in flight the primary button says "Keeping…" and both address buttons stay disabled. Remove and Apply say what they are doing and stay disabled. Success toasts are "Kept.", "Removed.", and "Applied." A restart uses the node restart the panel already has and then reloads this page.

A stopped, starting, stopping, or failed node replaces the card with the same empty state the page already uses. The kept list is not shown from memory.

Copy is in `en` and `zh`. Tokens come from the design system. The column stays the Peers column.

Dev-only, `?peers=kept`, `?peers=apply`, `?peers=restart`, and `?peers=adding` draw those shots without a manager. `?peers=down`, `?peers=empty`, and `?peers=list` still draw the shots they drew before. The production bundle does not include them.

## How a later slice adds a route

A manager feature is a handler with a `routes()` method. `newMux` mounts those methods. Auth and the node lifecycle stay in `newMux` because they close over the process. A new area adds a type, a `routes()` method, and one line in `newMux`. It does not grow a positional parameter list.

The browser calls the manager through `apiJSON` in `apps/web/src/lib/api.ts`. A feature adds a typed function next to its page, the way `apps/web/src/peering/api.ts` does, and a store that uses it. The page does not call `fetch` itself.

These routes are for the one local node. A second node should put a node id in front of the same handlers, not a second peering implementation. Kubo install, remote control, IPNS, remote pinning, and diagnostics can each be a package, a `routes()` method, and a typed client. This slice does not build those.

## Tests

Go: the saved list gains an address, dedupes it, and drops a peer id; the cap refuses another peer; live and saved differ as `apply`; a failed live call after a save is `restart`; the replace body has no private key; a backup is written and has no private key; `Addresses.API` is unchanged; loopback refusal; HTTP session, CSRF, 503 with `state`, 400, and a keep that returns the peer id and the stored address.

Web: the list parser, and the page states in English (stopped, empty, kept, apply, restart, adding, a rejected address that does not call the manager).

The released-binary harness covers add, the running list without a restart, the same peer after a daemon restart, remove, and a stopped node. The pin and the checks are in [the harness](harness.md).

## Out of scope

- A map, disconnect of a swarm connection that is not a kept peer, and per-peer bandwidth.
- Showing swarm addresses for peers the person did not keep.
- Editing one address off a peer without removing the peer. Remove drops the peer. The raw editor can change a single address.
- Resolving a `/dnsaddr/` name that does not already end in a peer id.
- Installing Kubo, remote control, a desktop shell, IPNS, remote pinning, diagnostics, and a second node.
