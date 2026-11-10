# Scenes

**English** · [中文](scenes.zh-CN.md)

**Status: implemented.** This file is the spec for roadmap slice 5. If the code and this file disagree, this file wins until a pull request changes it. The four scenes are the ones named in [Vision](../VISION.md). How they compare with ipfs-webui is in [ipfs-webui parity](webui-parity.md).

## What this is for

A beginner picks a setup they can say out loud. The panel shows what will change, writes it into the Kubo config, and says in one sentence what it did. The person does not open Kubo's JSON.

The browser still never talks to Kubo. The manager writes the config file on disk. It does not call `ipfs config`, and it never returns the file, the private key, or a multiaddr.

## The four scenes

Each scene writes the same set of keys. A key whose current value already means the same thing as the scene's value is not listed in the preview. Missing keys use Kubo's default for that comparison, from the citations below. `Addresses.Gateway` has no such default here: the panel's own loopback gateway is an explicit value.

The control address `Addresses.API` is never written. `Identity` is never written. Swarm listen addresses are never written.

| Scene | Id | One sentence |
| --- | --- | --- |
| A quiet node on this machine | `quiet` | This node stays on this machine and does less work for the network. |
| Share a folder with someone | `share` | People using IPFS can fetch the folders kept in Files. |
| Pin a set of content | `pinset` | The node tells the network about what you pin, and the gateway does not fetch other files. |
| A read-only public gateway | `gateway` | Other computers can read files already on this node. They cannot change anything, and the control port stays on this machine. |

`share` does not pick a folder. The folders are the ones already in Files. The scene only changes how the node announces them (`Provide.Strategy` `mfs`, `docs/config.md:2331`).

### Keys

`quiet` follows the `lowpower` profile (`config/profile.go:373-393`, `docs/config.md:4683-4690`). The other scenes put those keys back to Kubo's usual values and then set the one or two keys that make the scene.

| Key | `quiet` | `share` | `pinset` | `gateway` | Usual value when the key is missing |
| --- | --- | --- | --- | --- | --- |
| `Routing.Type` | `autoclient` | `auto` | `auto` | `auto` | `auto` (`config/routing.go:15`, `docs/config.md:3028`) |
| `AutoNAT.ServiceMode` | `disabled` | `enabled` | `enabled` | `enabled` | `enabled` (`docs/config.md:567`) |
| `Swarm.RelayService.Enabled` | `false` | `true` | `true` | `true` | `true` (`docs/config.md:3455`) |
| `Swarm.ConnMgr.Type` | `basic` | `basic` | `basic` | `basic` | `basic` (`config/init.go:96`) |
| `Swarm.ConnMgr.LowWater` | `20` | `32` | `32` | `32` | `32` (`config/init.go:85`, `docs/config.md:3611`) |
| `Swarm.ConnMgr.HighWater` | `40` | `96` | `96` | `96` | `96` (`config/init.go:81`, `docs/config.md:3620`) |
| `Swarm.ConnMgr.GracePeriod` | `1m0s` | `20s` | `20s` | `20s` | `20s` (`config/init.go:89`, `docs/config.md:3629`). The lowpower profile sets one minute (`config/profile.go:387`). |
| `Provide.Enabled` | `true` | `true` | `true` | `true` | `true` (`docs/config.md:2310`) |
| `Provide.Strategy` | `all` | `mfs` | `pinned` | `pinned+mfs` | `all` (`config/provide.go:13`, `docs/config.md:2385`) |
| `Gateway.NoFetch` | `false` | `false` | `true` | `true` | `false` (`docs/config.md:1301`) |
| `Gateway.NoDNSLink` | `false` | `false` | `false` | `true` | `false` (`docs/config.md:1311`) |
| `Gateway.ExposeRoutingAPI` | `true` | `true` | `true` | `false` | `true` (`config/gateway.go:12`) |
| `Addresses.Gateway` | panel loopback | panel loopback | panel loopback | unspecified, panel port | no implicit match |

`Provide.Strategy` values are the ones Kubo documents: `all` (`docs/config.md:2318`), `pinned` (`docs/config.md:2319`), `mfs` (`docs/config.md:2331`), `pinned+mfs` (`docs/config.md:2333`).

The panel loopback gateway is `/ip4/127.0.0.1/tcp/8081`, or `8082` when the manager is already on `8081`. That is the port [Node lifecycle](../DESIGN-node-lifecycle.md) chooses. The public gateway is exactly these two listeners and no others:

```text
/ip4/0.0.0.0/tcp/<panel port>
/ip6/::/tcp/<panel port>
```

`Addresses.Gateway` allows that shape (`docs/config.md:332-354`). Binding the unspecified address is what makes it reachable from other computers. A specific public IP is not written and is not accepted at start.

`Gateway.Writable` cannot be set. Kubo removed it (`docs/config.md:1514-1516`). Read-only, for this scene, means three flags that are still real:

- `Gateway.NoFetch` true: the gateway serves only what is already in the repo (`docs/config.md:1296-1299`). Kubo's own security note says to use this when the gateway is public (`docs/config.md:4757`).
- `Gateway.NoDNSLink` true: a `Host` header does not retarget `/` (`docs/config.md:1305-1309`).
- `Gateway.ExposeRoutingAPI` false: Kubo attaches `/routing/v1` to every gateway listener when the flag is on (`cmd/ipfs/kubo/daemon.go:1103-1125`). The flag's default is true (`config/gateway.go:12`). The public scene turns it off so the public port does not serve that API. The doc text says the routing API is on `127.0.0.1` (`docs/config.md:1387`); the daemon code serves it on the gateway listeners, which is why the flag is explicit.

The page says this in one sentence: other computers can read files already stored here, they cannot add or change files, and the control port stays on this machine. It may name the gateway port. It does not show a multiaddr, `0.0.0.0`, or port `5001`.

## Restart

Every key above is read when the daemon is built, not on a later request:

| Key | Read at |
| --- | --- |
| `Routing.Type` | `cmd/ipfs/kubo/daemon.go:458` |
| `Swarm.ConnMgr` | `core/node/groups.go:43-53` |
| `AutoNAT.ServiceMode` | `core/node/groups.go:98` |
| `Swarm.RelayService.Enabled` | `core/node/groups.go:120` |
| `Provide.Enabled` | `core/node/groups.go:359` |
| `Provide.Strategy` | `core/node/provider.go:1162` |
| `Gateway.NoFetch` | `core/corehttp/gateway.go:160` |
| `Gateway.NoDNSLink` | `core/corehttp/gateway.go:396` |
| `Gateway.ExposeRoutingAPI` | `cmd/ipfs/kubo/daemon.go:1124` |
| `Addresses.Gateway` | `cmd/ipfs/kubo/daemon.go:1079` |

Closing the repo does not write the config back (`repo/fsrepo/fsrepo.go:547-578`). The next start loads the file (`repo/fsrepo/fsrepo.go:461-466`).

So a change applies only after a restart. The panel uses the lifecycle it already has. Restart is stop, then start. It is not a new state.

| Node state when the person confirms | What the panel does |
| --- | --- |
| `up` | Write, then restart. The response says `restart` `started`. The overview comes back as `stopping`. The page keeps polling. |
| `down`, or `failed` for a reason other than `repo_busy` | Write, and leave the node stopped. `restart` is `later`. The next Start uses the new file. |
| `starting`, `stopping`, or `failed` with `repo_busy` | Do not write. **409** `node_busy`. |

If the node leaves `up` between the write and the restart, the panel does not start it. `restart` is `later`.

## Preview, apply, backup, revert

The config file has to exist. Before the first successful start it does not. **409** `not_ready`. The page says to start the node once.

Preview does not write. Apply writes only when at least one key is effectively different. If nothing differs, the answer is **200** with `restart` `none` and no new backup.

The write is the whole file, with the scene's keys replaced and every other key left as it was, including `Identity` and `Addresses.API`. It uses the same temp-file, fsync, and rename as the other manager files, mode `0600`. Before the rename, the new bytes are checked with the same rule the supervisor uses at start. If that check fails, nothing is written.

A backup is the previous value of each managed key, including JSON `null` when the key was absent. It does not contain the private key, because that key is not in the set. Backups live in `<data>/scene-backups/`, mode `0600`, directory mode `0700`. The five newest are kept. Revert writes the newest backup back, deletes that backup, and uses the same restart rule. No backup is **409** `nothing_to_revert`.

A public API is refused before any write: **409** `api_not_loopback`. The scene is not applied.

## API

Session and CSRF match [Auth](auth.md) and [Files](files.md). GET needs a session. POST needs `Content-Type: application/json` and `X-CSRF-Token`. Unknown JSON fields are **400** `bad_request`. An empty body is not valid on apply. Revert accepts `{}` or an empty body and no other key.

| Method and path | Body or query | Success |
| --- | --- | --- |
| `GET /api/scenes` | — | **200** list |
| `GET /api/scenes/preview?id=` | `id` is one of the four | **200** one scene |
| `POST /api/scenes/apply` | `{"id"}` | **200** result |
| `POST /api/scenes/revert` | `{}` | **200** result |

List:

```json
{
  "ready": true,
  "gatewayPort": 8081,
  "nodeState": "up",
  "revert": true,
  "scenes": [
    {
      "id": "quiet",
      "active": false,
      "restart": true,
      "changes": [
        { "key": "Routing.Type", "code": "autoclient" },
        { "key": "Swarm.ConnMgr", "code": "quiet" }
      ]
    }
  ]
}
```

`changes` lists only effective differences. `code` is a short token the page translates. It is not the raw config value. `Swarm.ConnMgr` is one row for the four connection-manager keys: `quiet` for the lowpower numbers, `usual` otherwise. Gateway address codes are `public` or `local`. `active` means there is no effective difference. `gatewayPort` is the panel's gateway port, so the page can name it. The body has no multiaddr and no private key.

Result:

```json
{ "id": "quiet", "restart": "later", "node": { "state": "down", "peerId": null, "version": null, "repoPath": "/path", "error": null } }
```

`restart` is `none`, `later`, or `started`. `node` is the overview from [Node lifecycle](../DESIGN-node-lifecycle.md). The page turns `id` and `restart` into one sentence per language. The server does not send the sentence.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `unknown_scene` | `id` is missing or not one of the four |
| 400 | `bad_request` | JSON the handler cannot read |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 409 | `not_ready` | No config file yet |
| 409 | `node_busy` | Starting, stopping, or `repo_busy` |
| 409 | `nothing_to_revert` | Revert with no backup |
| 409 | `api_not_loopback` | The control address is not loopback |
| 415 | `unsupported_media_type` | Wrong content type |
| 500 | `unavailable` | The file could not be read or written |
| 502 | `restart_failed` | The file was written and the restart call failed |

## Start check

[Node lifecycle](../DESIGN-node-lifecycle.md) refuses a gateway that is not loopback. This slice adds one exception, and only this one:

- Every API address is still loopback. A public API is still `api_not_loopback`. The scene never writes it (`docs/config.md:314-326`).
- A gateway address may be non-loopback only when every such address is the unspecified listener on the panel gateway port, and the three read-only flags above are set. Otherwise the code is `gateway_not_readonly` when the address shape matches but the flags do not, and `gateway_not_loopback` for any other public address.
- The gateway still must not use the manager's port.

Files and pins keep working when the gateway is public. Their loopback client uses the API address. The local share link is omitted, because there is no loopback gateway URL to copy. The page does not invent a public URL.

## UI

The Scenes item in the nav is enabled. It is not under "Coming soon".

The page has a title and one sentence. Then four cards, in the order quiet, share, pinset, gateway. Each card is the scene's name, one sentence, and Preview. The scene that matches is marked "In use". When none matches, one line says the node is on Kubo's usual settings.

Preview is a dialog. It repeats the scene sentence, then one plain line per change, with the config key in muted type under the line. It says whether the node will restart or whether the change waits for the next start. Use this setup is the primary button. It is not available when nothing would change, or while the node is busy. The dialog does not show a multiaddr.

After apply or revert, one sentence is the status, in the language of the page. The node card's own sentence keeps updating while a restart is in progress.

Undo the last change is shown when a backup exists. The confirm dialog's primary button is Undo. Cancel is focused first.

Not ready, loading, and a manager error each have their own state. Not ready links to the Node page. Other errors are toasts.

Copy is in `en` and `zh`. Tokens come from the design system.

## Tests

Go: each scene's keys, the private key and the API address left in place, preview codes with no multiaddr in the JSON, apply while up restarts, apply while down does not start the node, revert restores the previous values, a public API is refused, backup mode `0600` on Unix, and the HTTP session, CSRF, and stopped-node apply.

Web: the four cards, the preview lines, the Chinese sentence after apply, and no port `5001` on the page.

## Out of scope

- A raw config editor. See [ipfs-webui parity](webui-parity.md). Explore and Peers are separate pages.
- Choosing which folder the share scene announces. It announces MFS.
- A public gateway that fetches arbitrary CIDs.
- Binding the gateway to one chosen public IP.
- Writing `Addresses.API`, `Addresses.Swarm`, `Identity`, or `Datastore.Spec`.
- Showing port `5001`, a multiaddr, or the private key.
