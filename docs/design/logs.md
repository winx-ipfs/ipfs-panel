# Logs

**English** · [中文](logs.zh-CN.md)

**Status: implemented.** This file is the spec for the Logs page. If the code and this file disagree, this file wins until a pull request changes it. It is expert-mode diagnostics for one node, not a new numbered roadmap slice. A second node stays out of this change. Permanent peering is [Permanent peering](peering.md).

## What this is for

A beginner can open one page and see what the node and this panel are doing. An expert can filter that record, follow it as it grows, and raise a Kubo subsystem's log level when a sentence is missing.

The browser still never talks to Kubo. It talks to the manager. The manager is the process that started Kubo, so it already owns the daemon's stdout and stderr. It also writes its own operational and audit log. Those two streams are the page. The browser does not receive a Kubo RPC address, and this page does not dial any address that appears in a line.

## What ipfs-webui actually ships

Checked against ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`.

Diagnostics is `/diagnostics*` (`ipfs-webui/src/bundles/routes.js:25`). The logs screen is one of three diagnostics tools (`ipfs-webui/src/diagnostics/diagnostics-content.tsx:11`, `ipfs-webui/src/diagnostics/diagnostics-content.tsx:106-108`). It tails Kubo with `ipfs.log.tail` (`ipfs-webui/src/contexts/logs/logs-context.tsx:198`), which is Kubo's `log/tail` command. Play and pause start and stop that stream (`ipfs-webui/src/diagnostics/logs-screen/log-viewer.tsx:90-97`). Clear drops the entries the browser stored (`ipfs-webui/src/contexts/logs/logs-context.tsx:211-217`). A separate control reads and writes subsystem levels with `ipfs.log.level` (`ipfs-webui/src/contexts/logs/api.ts:59-75`). The browser holds a Kubo RPC client.

This page keeps the job: a live log, pause, clear, and an expert level change. It does not keep the RPC client, `log/tail`, the retrieval check, or DHT provide.

## Sources

Two sources, and nothing else.

| Source | What it is |
| --- | --- |
| `manager` | The manager's own slog: operational lines and `msg=audit` lines. |
| `kubo` | Stdout and stderr of the daemon process this manager spawned. |

`log/tail` is experimental (`core/commands/log.go:242-243`). It is a second stream of the same daemon, delivered over the RPC this panel does not expose. This page does not call it.

The manager's slog handler records `Info` and above. `Debug` from the manager is not kept. Kubo lines are kept at whatever level the daemon actually writes, including `debug` after an expert raises it.

Stdout and stderr of short-lived `ipfs` commands (`init`, `id`, `config`) stay on the path [Node lifecycle](../DESIGN-node-lifecycle.md) already uses for `error.detail`. They are not copied onto this page. The stderr tail used for that detail stays capped at 2000 bytes and is independent of this log.

When the manager adopts a daemon it did not spawn, it has no pipes. `capture` is false. The page says so. Historical lines from an earlier spawn remain.

## Retention

Lines live in two places under `<data>/logs/`, which is the manager data directory, not the Kubo repo.

- A ring keeps the newest 2000 lines per source. The live page reads the ring.
- `manager.log` and `kubo.log` keep the same lines on disk. A file rotates at 1 MiB. The current file plus two older ones are kept (`.log`, `.log.1`, `.log.2`). Older bytes are deleted. The directory is mode `0700`. Each file is mode `0600`.

A line longer than 2048 bytes is cut on a UTF-8 boundary. The cut remainder is a following line. Empty lines are dropped.

On startup the manager reads the tail of the current file, up to 2000 lines, back into the ring. Ids start at 1 for this process. They are not stable across a manager restart.

A file write that fails does not drop the ring and does not stop the daemon. Download and the ring can disagree until the disk accepts writes again.

The ring is the live view. Debug output can push older lines out of the ring. The rotated files still hold them until they age out.

## Redaction

A line is redacted before it enters the ring or a file. The same pass runs again when a file is replayed.

These slog attribute keys, and the last dotted segment of a grouped key, are replaced with `[redacted]`, case-insensitive: `setup_token`, `password`, `privkey`, `privatekey`, `private_key`, `authorization`, `authorizations`, `csrf`, `csrf_token`, `session`, `cookie`, `token`, `secret`, `apikey`, `api_key`.

The text of every line, including Kubo's, is then scanned:

- a `PrivKey` or `private key` assignment, JSON or `key=value`
- `setup_token=…`
- the stdout sentence `ipfs-panel setup token: …` if it is ever copied into slog
- `ipfs_panel_session=…`
- an `X-CSRF-Token` assignment
- a `Bearer` credential
- `api.key=…`

The one-time setup token printed to the manager's own stdout stays on stdout. It is not ingested. The slog warning that names `setup_token` is ingested with the value redacted. Audit lines stay what they already are: event, result, username, and IP. They do not carry a document, a password, or a session cookie.

The page does not add a Kubo RPC address of its own, and it does not turn a line into a link or a dial. It does not delete a listen address the daemon itself wrote. That sentence is the log.

## Reading

Session matches [Auth](auth.md). GET needs a session. It does not need the CSRF header. POST needs `Content-Type: application/json` and `X-CSRF-Token`. Unknown JSON fields are **400** `bad_request`.

`GET /api/logs` does not call Kubo. It answers while the node is stopped, starting, or never started.

Query: `after`, `source` (`all`, `manager`, `kubo`), `level` (`debug`, `info`, `warn`, `error`), `q`, `limit`.

Defaults: `source=all`, `level=info`, `limit=500`, `after=0`, `q` empty. `level` is a minimum: `warn` returns warn and error. `q` is a case-insensitive substring of the message, at most 200 runes, and it does not match the timestamp or the source. `limit` above 1000, a negative `after`, an unknown source or level, or a longer `q` is **400** `bad_request`.

`after=0` returns the newest matching lines, up to `limit`. `after` greater than zero returns the next matching lines after that id, oldest first, up to `limit`, so a client can catch up in order. `lines` is `[]` when there are none, never `null`. `next` is the last returned id, or `after` when the page is empty.

```json
{
  "lines": [
    {
      "id": 12,
      "time": "2026-11-15T11:14:00.123Z",
      "source": "kubo",
      "level": "info",
      "text": "core daemon is ready"
    }
  ],
  "next": 12,
  "node": "up",
  "capture": true
}
```

`time` is UTC RFC3339Nano. `node` is the cached node state. `capture` is true only when the current child was spawned by this manager and the pipes are open.

`GET /api/logs/stream` is server-sent events, same query, same session cookie. `Content-Type` is `text/event-stream`. Each line is:

```text
id: 12
event: line
data: {"id":12,"time":"…","source":"kubo","level":"info","text":"…"}
```

A `status` event carries `{"node","capture"}` at the start and about every 15 seconds, with a comment ping so a proxy does not cut the idle stream. A `gap` event means the browser was too slow and a line was dropped from the live buffer. The client refetches `GET /api/logs?after=<last id>`. Lines that have already rotated out of the ring are still in the files until those rotate.

`GET /api/logs/download?source=all|manager|kubo` is `text/plain; charset=utf-8` with an attachment name `ipfs-panel-logs.txt`, `ipfs-panel-manager.txt`, or `ipfs-panel-kubo.txt`. It flushes, then sends the rotated files oldest first. One source is the stored line `time<TAB>level<TAB>text`. `all` prefixes `manager` or `kubo` and a tab. The bytes were redacted at write time.

There is no request that deletes the files. Clear view is a browser action.

## Expert level

`POST /api/logs/level` with `{"subsystem","level"}` calls Kubo `log/level` (`core/commands/log.go:126-155`). The node must be `up`. Anything else is **503** `node_not_running` with `state`, and Kubo is not called.

`subsystem` is `all`, `*`, or one token matching `^[A-Za-z0-9][A-Za-z0-9_./-]{0,63}$`. Kubo treats `all` and `*` as every subsystem (`core/commands/log.go:15-18`, `core/commands/log.go:137-138`). `level` is `debug`, `info`, `warn`, `error`, `dpanic`, `panic`, `fatal`, or `default`. Anything else is **400** `bad_subsystem` or **400** `bad_level`, and Kubo is not called.

Success is **200** `{"subsystem","level"}`. The manager does not restart the node. A non-200 from Kubo is **502** `kubo_rejected`. `detail` is Kubo's message, cut to 240 characters, with control characters removed. A deadline is **504** `timeout`. The deadline is 10 seconds. `log/level` for an unknown subsystem name does not fail in go-log v2.9.2: the level is stored for a logger created later. This panel does not treat that as an error.

`GET /api/logs/levels` calls `log/ls` (`core/commands/log.go:220-229`). Success is **200** `{"subsystems":["…"]}`, sorted, names only. The list is the `Strings` field (`core/commands/swarm.go:42-44`). The page does not fetch every subsystem's current level. The same `up` rule and the same errors apply. A body over 1 MiB is **413** `too_large`.

The audit line is structured slog, `msg=audit`, `event` `log_level`, plus `result`, `username`, `ip`, `subsystem`, and `level`. Results include `success`, `invalid`, `rejected`, and `unavailable`. A read is not an audit event.

## Kubo line shape

The manager's pipes are not a terminal, so go-log v2.9.2 writes plaintext console lines: time, level, logger name, caller, message, separated by tabs. ANSI is stripped first. A recognized level (`DEBUG`, `INFO`, `WARN`, `WARNING`, `ERROR`, `DPANIC`, `PANIC`, `FATAL`) becomes `debug`, `info`, `warn`, or `error` (`DPANIC`, `PANIC`, and `FATAL` are `error`). The caller field (`file.go:line`) is dropped. The stored text is the logger name and the message. The timestamp, when it parses as RFC3339, is the line's time. A JSON line with `level` and `msg` uses those. Anything else is kept whole, at `info`, with the time the manager read it. The message is not translated.

## API errors

| Status | `error` | When |
| --- | --- | --- |
| 400 | `bad_request` | Bad query, bad JSON, unknown field |
| 400 | `bad_level` | Level not in the allow list |
| 400 | `bad_subsystem` | Subsystem not in the allow list |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 413 | `too_large` | `log/ls` body over 1 MiB |
| 415 | `unsupported_media_type` | POST with the wrong content type |
| 502 | `kubo_rejected` | Kubo refused the level. `detail` may be set |
| 503 | `node_not_running` | Level routes while the node is not up. `state` is included |
| 503 | `unavailable` | The log store is missing, or the node snapshot failed |
| 504 | `timeout` | Kubo did not answer in time |

`GET /api/logs`, the stream, and download do not return `node_not_running`. A stopped node still has a manager log.

## UI

The nav item is Logs, after Scenes and before Settings. Settings stays at the bottom of the rail. The page is a compact title, one sentence, and one surface that fills the window. The toolbar is one wrapping row on that surface: source, level, search, follow or pause, copy, download, clear view, and Levels. The list under it is monospace, 13px, one fixed-height row per line, and only the visible rows are in the DOM. A row is a timestamp, a level tag, a source tag, and the raw message. The row scrolls horizontally. The list scrolls vertically and runs to the bottom of the window.

Follow is the default. The view sticks to the bottom. Pause keeps the stream and stops sticking. Scrolling by hand does not pause. Clear view drops what the browser is showing and leaves the files. Copy writes the lines in the view as `time`, raw level, raw source, and raw text. Download fetches the file through the manager. Search is debounced and sent as `q`. Changing source, level, or search reloads the tail and reopens the stream.

Level tags use the status washes. Firefly green is not a level and not the selection. The Following tag is the small expert-tag treatment. Paused is neutral.

Levels opens a dialog marked Expert. It asks for one subsystem (default `all`) and one level, and it says the node will not restart. It is disabled while the node is not up. The dialog is not the home of the page.

Log messages are not translated. Chrome is, in `en` and `zh`.

| State | What the page does |
| --- | --- |
| Loading | The toolbar, then skeleton rows. Not an empty log. |
| Streaming | Lines, the Following tag, stuck to the bottom. |
| Paused | The same lines, the Paused tag, the scroll position left where it was. |
| Filtered | The search or the source or the level is narrowing the list. No match is its own empty sentence and a control that clears the filter. |
| Empty | The node is up, and nothing has been written that matches. |
| Node stopped | The node is not up. With no lines, an empty state and a link to the Node page. With lines, those lines and one sentence that they are earlier output. Starting, stopping, and failed each have their own sentence. |
| Adopted | The node is up and `capture` is false. One sentence: this panel did not start the process, so it cannot read the output. Level change still works. |
| Error | The sentence for the code, and Retry. `detail`, when present, sits under the sentence. |

Light and dark, and a 1440 and a 390 layout, are both real layouts: the toolbar wraps, the list keeps the remaining height, and the dialog fits the narrow width.

## Tests

Go: redaction of a private key, a setup token, a session cookie, and a bearer credential, and a normal line left intact; the ring drops the oldest line; rotation keeps three owner-only files on Unix and a writable file on Windows; query by source, level, and text; a go-log plaintext line is parsed and an unrecognized line is kept; the slog handler redacts `setup_token`; the HTTP session and CSRF; a stopped node can read the manager log and cannot set a level (`node_not_running`); a bad level does not call Kubo; download contains `[redacted]` and not the secret.

The released-binary harness, in `TestRealKuboOptional`, points the supervisor's daemon pipes at a store. After the node is up, at least one kubo line is present. `log/ls` returns a non-empty `Strings` list. `log/level` of `core` to `error` returns Kubo's changed-level sentence. After the supervisor has reached `down`, the same call returns `node_not_running`. The settled lines are in [Harness](harness.md).

Web: the nav label, the streaming lines, pause, a filter with no match, the stopped-node sentence, the error sentence, Chinese chrome, and a message rendered as text rather than HTML.

## Out of scope

- Permanent peering. That is [Permanent peering](peering.md).
- A second node.
- `log/tail`, the retrieval check, and DHT provide.
- Deleting the log files from the page.
- Translating the lines the node wrote.
