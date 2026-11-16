# Raw config editor

**English** · [中文](config-editor.zh-CN.md)

**Status: implemented.** This file is the spec for the expert raw config editor. If the code and this file disagree, this file wins until a pull request changes it. It is the remaining ipfs-webui parity gap named in [ipfs-webui parity](webui-parity.md). It is expert mode. It is not a new numbered roadmap slice, and it is not on the beginner path.

## What this is for

An expert can read the Kubo config, change it as JSON, see the difference, and save it. The panel keeps a backup of the previous document and can put that backup back in one step. It says that a restart is required and offers one. It does not restart on its own.

The editor lives on Settings, in an Advanced section marked Expert. It is not a nav item. Node, Files, Explore, Peers, Pins, and Scenes stay the beginner path. [Vision](../VISION.md) already says the raw config is not the home page.

The browser still never talks to Kubo. It talks to the manager. The manager calls Kubo on loopback. The page does not gain a form for the Kubo RPC address, and it does not dial any address it is showing.

## What ipfs-webui actually ships

Checked against ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`.

Settings is `/settings*` (`ipfs-webui/src/bundles/routes.js:22`). The config block is an Ace editor with line numbers (`ipfs-webui/src/settings/editor/JsonEditor.js:15-28`). The page loads the whole config with `config.getAll` (`ipfs-webui/src/bundles/config.js:12`) and saves with `config.replace` after `JSON.parse` (`ipfs-webui/src/bundles/config-save.js:64-72`). Reset copies the last loaded text back into the editor. There is no backup, no diff, and no scene warning. A stopped daemon replaces the editor with one sentence (`ipfs-webui/public/locales/en/settings.json:120-121`). After a save, the page says the new settings apply the next time the daemon starts (`ipfs-webui/public/locales/en/settings.json:126-127`).

The webui's browser holds a Kubo RPC client. This panel does not copy that. It copies the job: edit the JSON, save it through `config/replace`, and restart later.

## Read and write

The manager reads with Kubo `config/show` and writes with Kubo `config/replace`. Both are `POST /api/v0/...` on the loopback API. `config/show` reads the file on disk (`core/commands/config.go:239-247`). `config/replace` decodes the body into Kubo's config struct and calls `SetConfig` (`core/commands/config.go:665-724`, `repo/fsrepo/fsrepo.go:654-677`).

`SetConfig` deep-merges the decoded struct onto the map already in the file (`repo/common/common.go:65-92`). A key Kubo does not know is ignored. A key that Kubo's struct omits when empty can remain at its previous value. The editor therefore does not promise that deleting a line deletes the key. The page says so. The way to change a value is to set the value.

The request body for replace is one multipart part whose `Content-Disposition` carries `filename`, which is the shape Kubo's HTTP file argument reads (`files/multipartfile.go:161-173`). The part's bytes are the JSON document. The manager does not send `expand-auto`.

## Secrets

Kubo already scrubs three things from `config/show` (`core/commands/config.go:273-286`):

- `Identity.PrivKey` (`docs/config.md:1724-1727`)
- `API.Authorizations` (`config/api.go:10`)
- `Pinning.RemoteServices.*.API.Key` (`config/remotepin.go:5`)

The manager scrubs the same three paths again, case-insensitive, before the body can reach the browser. The private key is omitted. It is not replaced with a placeholder the browser could send back.

`config/replace` rejects a non-empty `Identity.PrivKey` (`core/commands/config.go:673-675`) and then writes the key that is already in the repo (`core/commands/config.go:677-692`). It also refuses to add, remove, or change the API details of a remote pinning service (`core/commands/config.go:703-720`). The manager never puts the private key, an authorization entry, or a pinning API key into the replace body. Preservation is Kubo's, on the key it already stored.

If the submitted document contains a non-empty `Identity.PrivKey`, the manager refuses with **400** `privkey_locked` and does not call Kubo. If it contains `API.Authorizations`, or a non-empty remote pinning `API.Key`, the manager refuses with **400** `secret_locked` and does not call Kubo. An empty or missing private key is stripped and Kubo keeps the real one.

The audit line and the logs never include the document, the private key, a pinning key, or an authorization value.

## What the manager refuses to change

`Addresses.API` is how the manager finds Kubo. A save whose API address list is not exactly the current list is **409** `api_locked`. The list is the string or the array of strings Kubo stored. Reordering it, adding one, or changing a port is a change. The same list in the other JSON shape (one string, or a one-element array of that string) is the same list.

`Identity.PeerID` has to stay the peer id `config/show` returned. Kubo would overwrite a different one from the stored key (`core/commands/config.go:688-692`, `docs/config.md:1711-1714`). A different or missing peer id is **409** `identity_locked`.

Before replace, the manager also runs the same check the supervisor uses before start ([Node lifecycle](../DESIGN-node-lifecycle.md), `AssessConfig`). A document the panel would refuse to start is not written. The codes are the ones that check already returns: `api_not_loopback`, `gateway_not_loopback`, `gateway_not_readonly`, `panel_port_conflict`. Each is **409**.

Those checks are the schema sanity this panel does itself. Kubo's own decode is the rest. A document that passes the checks and that Kubo still rejects is **502** `kubo_rejected`, with `detail` set to Kubo's message, cut to 240 characters, with control characters removed. The previous file is unchanged.

## Backup and restore

A successful replace that changes the document writes the previous redacted text to `<data>/config-backups/`, mode `0600`, directory mode `0700`. The file does not contain the private key, because the text it copies was already scrubbed. The five newest are kept. A replace Kubo rejects removes the backup it just wrote. A replace that changes nothing writes no backup.

Restore reads the newest backup and sends it through the same replace path, including the locks above. It does not write another backup. On success it deletes the backup it applied. No backup is **409** `nothing_to_restore`.

Scene backups stay in `<data>/scene-backups/`. This editor does not read or write them.

## Restart

The file changes when Kubo accepts the replace. The running daemon has already read most of those keys. The page does not restart by itself. The response says `restart` `needed`. The page offers the restart the panel already has. Dismiss leaves the node running; the new file is used the next time it starts. `restart` `none` means the document matched what was already stored.

## Scenes

[Scenes](scenes.md) still apply and undo by editing the config file and, when the node is up, restarting. This editor does not take that path, and it does not touch scene backups.

`GET` reports `scene` as the id of the scene whose keys already match, or `null`. The match is the one `scenes.Catalog` already computes. `sceneKeys` is the managed-key list from that spec, so the page can warn while the expert types.

`diverged` is true on a write when a scene was active and the submitted document changes any of those keys. The diff dialog says the node will leave that setup, and that Scenes can still apply or undo. A raw edit of any other key does not set `diverged`. After a diverging save, the next scene list no longer marks that scene in use. Undo on Scenes still restores the scene keys from the scene backup.

## API

Session and CSRF match [Auth](auth.md). GET needs a session. POST needs `Content-Type: application/json` and `X-CSRF-Token`. Unknown JSON fields are **400** `bad_request`. The replace body is `{"text":"<json>"}`. Restore accepts `{}` or an empty body and no other key. `text` is at most 2 MiB, the same cap the manager uses when it reads the Kubo config file. A larger body is **413** `too_large`.

The node has to be `up`. Anything else is **503** `node_not_running` with `state`. The editor cannot fall back to writing the file: the read path is `config/show`, and that RPC is down when the node is down.

| Method and path | Body | Success |
| --- | --- | --- |
| `GET /api/config` | — | **200** document |
| `POST /api/config` | `{"text"}` | **200** result |
| `POST /api/config/restore` | `{}` | **200** result |

Document:

```json
{
  "text": "{\n  \"Addresses\": {\n    \"API\": \"/ip4/127.0.0.1/tcp/5001\"\n  }\n}\n",
  "scene": "quiet",
  "sceneKeys": ["Routing.Type", "Addresses.Gateway"],
  "backup": false
}
```

`text` is the redacted document, two-space indent, object keys sorted, one trailing newline. `scene` is a string or `null`. `sceneKeys` is always the managed list. `backup` is whether a restore point exists.

Result:

```json
{
  "text": "{}\n",
  "scene": null,
  "sceneKeys": ["Routing.Type"],
  "backup": true,
  "restart": "needed",
  "diverged": true
}
```

`restart` is `none` or `needed`. `diverged` is a boolean.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `bad_request` | JSON the handler cannot read, or an unknown field |
| 400 | `invalid_json` | `text` is not JSON, or it has trailing junk |
| 400 | `not_object` | The JSON value is not an object |
| 400 | `privkey_locked` | A non-empty `Identity.PrivKey` |
| 400 | `secret_locked` | `API.Authorizations`, or a non-empty pinning `API.Key` |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 409 | `api_locked` | `Addresses.API` missing or different |
| 409 | `identity_locked` | `Identity.PeerID` missing or different |
| 409 | `api_not_loopback` | The start check refused the API |
| 409 | `gateway_not_loopback` | The start check refused the gateway |
| 409 | `gateway_not_readonly` | A public gateway that is not the read-only shape |
| 409 | `panel_port_conflict` | The API or the gateway uses the manager port |
| 409 | `nothing_to_restore` | Restore with no backup |
| 413 | `too_large` | `text` is over 2 MiB |
| 415 | `unsupported_media_type` | Wrong content type |
| 502 | `kubo_rejected` | Kubo refused the replace. `detail` is Kubo's message |
| 503 | `node_not_running` | The node is not up. `state` is included |
| 503 | `unavailable` | The manager could not read or store the document |
| 504 | `timeout` | Kubo did not answer in time |

Audit lines are structured slog, `msg=audit`, with `event` `config_replace` or `config_restore`, `result`, `username`, and `ip`. Results include `success`, `unchanged`, `invalid`, `forbidden`, and `rejected`. The line has no document and no secret. A read is not an audit event.

## UI

Settings gains an Advanced tab, after About. The panel inside it carries an Expert tag. The sentence under the tag says this is the raw config, a wrong value can stop the node from starting, and the private key is not in the document.

The editor is a monospace code surface with line numbers. The dependency is CodeMirror 6, imported as the small packages (`@codemirror/state`, `@codemirror/view`, `@codemirror/commands`, `@codemirror/language`, `@codemirror/lang-json`, `@codemirror/lint`), not the Ace build the webui ships and not a second UI kit. The justification is the editor itself: line numbers, a gutter mark on the broken line, and bracket matching, without taking the search panel or a theme that paints the page. On this change the editor is its own chunk, 360 kB minified and 117 kB gzip, and it loads when Advanced is opened. The rest of the panel does not download it. The surface uses the panel tokens. Firefly green is not the editor background and not the selection.

The toolbar is Save, Revert, and Restore backup. Save is the one primary button. It is disabled while the JSON is invalid or the text matches the loaded document. Revert puts the loaded document back. Restore backup is shown only when `backup` is true, and it asks before discarding unsaved edits. Ctrl+S or Cmd+S opens the diff when save is available. The status line says whether the JSON is valid and, when it is not, the line nearest the error.

Save opens a diff of the loaded document and the editor, then a confirm. The dialog names a scene when the edit changes that scene's keys. Confirm runs the save. After `restart` `needed`, a second dialog offers Restart and Dismiss. Restart calls the existing node restart. `restart` `none` is a short confirmation and no restart dialog.

Leaving the page with unsaved edits asks first. Refresh does the same through the browser's before-unload prompt.

States, each with its own layout:

| State | What the page does |
| --- | --- |
| Loading | Skeleton in the shape of the editor. Not an empty document. |
| Ready | The document, Save disabled, the private-key sentence, the scene note when a scene matches. |
| Dirty | Save enabled. The status line says there are unsaved edits. |
| Invalid JSON | Save disabled. The status line names the line. The gutter marks it. |
| Diff | The confirm dialog, with the changed lines. |
| Saved | The restart dialog. |
| Node stopped | An empty state and a link to the Node page. No editor. |
| Error | The sentence for the code, and Retry. `detail`, when present, sits under the sentence. |

Copy is in `en` and `zh`. Tokens come from the design system. Light and dark, and a 1440 and a 390 layout, are both real layouts: the toolbar wraps, the editor scrolls inside itself, and the dialogs scroll their diff.

## Tests

Go: scrubbing the private key, the authorization list, and a pinning key; refusing a changed API address, a changed peer id, and a submitted private key without calling replace; a changed ordinary value calls replace and writes a `0600` backup on Unix; a Kubo rejection writes no backup; restore puts the previous text back and deletes that backup; a scene that matched stays matched when the edit avoids `sceneKeys`, and `diverged` is set when it does not; scene apply and scene undo still run after a raw edit; the HTTP session, CSRF, and a stopped node.

The released-binary harness calls the same client against Kubo v0.43.1. `config/show` has no private key while the file on disk still has one. A flipped `Discovery.MDNS.Enabled` comes back, the API address does not change, a submitted private key is refused, and a stopped node returns `node_not_running`. The settled lines are in [Harness](harness.md).

Web: the Advanced tab, the expert tag, invalid JSON disabling save, the diff before save, the Chinese restart sentence, and no private key in the rendered document.

## Out of scope

- IPNS keys, remote pinning services as a form, analytics, and the diagnostics page. Those stay deferred in [ipfs-webui parity](webui-parity.md).
- A form whose job is to set the Kubo RPC address the browser would call.
- Changing `Identity.PrivKey`, `API.Authorizations`, or a remote pinning API key.
- Permanent peering. The peers page still does not write `Peering.Peers`.
- Editing the config while the node is stopped.
- Restarting without asking.
