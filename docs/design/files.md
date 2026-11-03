# Files

**English** · [中文](files.zh-CN.md)

**Status: implemented.** This file is the spec for the Files page. If the code and this file disagree, this file wins until a pull request changes it. This is roadmap slice 3, the mutable file system part. Pinning stays slice 4, on its own page.

## What this is for

A beginner can put a file on this node, find it again, rename it, move it, make a copy, download it, and delete it. The page says "files", not "MFS". A content ID is shown, and explained in a disclosure, because that is the one IPFS idea the page cannot hide.

The browser still never talks to Kubo. It talks to the manager. The manager calls Kubo's file RPC on loopback, and only the calls this page needs.

## Decisions

1. **Mutable files only.** The manager calls `files/ls`, `files/stat`, `files/mkdir`, `files/write`, `files/mv`, `files/cp`, `files/rm`, and `files/read`. It does not call `add`, and it does not pin. A file in the mutable file system stays on this node until someone deletes it, because Kubo keeps that tree. The page does not say "pinned". Pins are the next slice.

2. **A stopped node is 503, not 409.** When the cached node state is not `up`, every files route returns **503** `{"error":"node_not_running","state":"<state>"}`. `state` is `down`, `starting`, `stopping`, or `failed`. **409** is only `already_exists`, when the name is taken. A dial that fails after the cache said `up` is the same 503 with `state` `down`: the node went away. 503 was chosen so a name conflict and a stopped node cannot be confused.

3. **Paths are absolute mutable-file paths.** They start with `/`. The root is `/`. The manager rejects `..`, `.`, an empty segment, a trailing or leading space in a name, `\`, NUL, other control characters, `"`, a name longer than 255 bytes, and a path longer than 4096 bytes. It does not clean `..` away. Cleaning would hide a traversal. `/` cannot be deleted, written, or created.

4. **The size limit is 512 MiB unless configured.** `-max-file-bytes` / `IPFS_PANEL_MAX_FILE_BYTES`, a positive integer. A request whose `Content-Length` is larger is **413** `too_large` and is not sent to Kubo. A body with no length is streamed and stopped after the limit; the manager then deletes the partial file when this upload was not replacing one. Downloads stream with no second cap, so a file already on the node can still be fetched. A directory listing larger than 32 MiB of JSON is `too_large`. File bytes are not buffered whole in the manager.

5. **Two timeouts.** Metadata calls (`ls`, `stat`, `mkdir`, `mv`, `cp`, `rm`) use 20s (`-files-timeout` / `IPFS_PANEL_FILES_TIMEOUT`). Upload and download use 10m (`-files-transfer-timeout` / `IPFS_PANEL_FILES_TRANSFER_TIMEOUT`). Both are Go durations. A deadline is **504** `timeout`. The handler uses the request context, so a disconnected browser cancels the Kubo call.

6. **The RPC dial is loopback only.** Each call reads `<repo>/config` and uses the API address only when every API and gateway multiaddr is `127.0.0.1` or `::1` over TCP. The HTTP client refuses any other dial, ignores `HTTP_PROXY`, and does not follow redirects. The browser never receives port 5001 or the RPC URL.

7. **The share link is a local gateway link.** The list includes `localGateway`, for example `http://127.0.0.1:8081`. The page builds `localGateway + /ipfs/<cid>` and calls the action "Copy local link". The toast says the link only works on this computer. The port is 8081 unless the panel had to move the gateway off the manager's port. The page does not present it as a public share.

8. **Session and CSRF match auth.** Every files route needs a session. POST needs `X-CSRF-Token`. GET does not. The public auth routes are unchanged. `GET /health` stays open.

9. **Upload is a raw body.** `POST /api/files/upload?path=` with `Content-Type: application/octet-stream`. The manager streams it into `files/write` as multipart without holding the whole file. `overwrite=1` is required to replace a file. A folder is never replaced. `create=true` and `truncate=true`. `parents` is false, so an upload cannot invent a chain of folders.

10. **Modified time is best-effort.** `files/ls?long=true` is the list. Type `1` is a folder. Anything else is a file. That listing has no modified time. The manager then calls `files/stat` for up to 32 entries and copies `Mtime` when Kubo stored one. Larger folders still list. Missing times are omitted, and the page says "Not recorded".

11. **Delete is recursive and confirmed.** `files/rm` is called with `recursive=true`. The page asks first, and says a folder delete removes everything inside. This cannot be undone from the panel.

12. **Errors are short codes.** The body is `{"error":"<code>"}`. `node_not_running` also has `state`. Kubo's own message stays in the manager log. It is not shown to the beginner.

## API

All of these need a session. POST needs the CSRF header. Unknown JSON fields are **400** `bad_request`. JSON bodies are at most 8 KiB. A wrong JSON content type is **415**.

| Method and path | Body or query | Success |
| --- | --- | --- |
| `GET /api/files?path=/` | `path` defaults to `/` | **200** list |
| `GET /api/files/stat?path=` | path required | **200** one entry |
| `GET /api/files/download?path=` | file path | **200** bytes, `application/octet-stream`, `Content-Disposition: attachment`, `nosniff` |
| `POST /api/files/mkdir` | `{"path"}` | **201** `{"path"}` |
| `POST /api/files/upload?path=&overwrite=` | raw bytes | **201** the new entry |
| `POST /api/files/rename` | `{"path","name"}` | **200** `{"path"}` |
| `POST /api/files/move` | `{"from","to"}` | **200** `{"path"}` |
| `POST /api/files/copy` | `{"from","to"}` | **200** `{"path"}` |
| `POST /api/files/delete` | `{"path"}` | **200** `{"path"}` |

`to` is the full destination path, not a folder plus a guess. The page builds it.

List:

```json
{
  "path": "/",
  "localGateway": "http://127.0.0.1:8081",
  "maxBytes": 536870912,
  "entries": [
    {
      "name": "notes.txt",
      "type": "file",
      "size": 1200,
      "cid": "bafy...",
      "modified": "2026-11-01T12:00:00Z"
    }
  ]
}
```

`modified` is omitted when Kubo has no time. `type` is `file` or `directory`. `entries` is `[]` when the folder is empty, never `null`. `localGateway` is omitted when the gateway address cannot be turned into a loopback URL. The page then hides the local-link action.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `invalid_path` | Path or name rejected, including deleting `/` |
| 400 | `bad_request` | JSON the handler cannot read |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 404 | `not_found` | Kubo has no such file |
| 409 | `already_exists` | Name taken, or a folder in the way |
| 413 | `too_large` | Over the size limit, or a huge listing |
| 415 | `unsupported_media_type` | Wrong content type |
| 502 | `kubo_error` | Kubo refused for another reason |
| 503 | `node_not_running` | Node state is not `up`, or the dial failed |
| 503 | `unavailable` | Config is not a loopback RPC, or the manager failed |
| 504 | `timeout` | The deadline elapsed |

## UI

The Files item in the nav is enabled. Pins is its own page. Scenes stays under "Coming soon".

The page has a title, one sentence, and a closed disclosure for the content ID and the local link. Then a toolbar: Upload (the one primary button), New folder, Refresh, and sort by name, size, or modified time. Sort is a button group, so it works on a phone where the table header is not the control. Name sort keeps folders first. Size and time sort by the value, and a missing time sorts last.

A breadcrumb shows the folder. The root is a home icon with an accessible name. Names wrap. They are not cut off with an ellipsis.

The wide layout is a table: name with a type icon, size, content ID shortened in the middle with Copy, modified time, and an actions menu. A folder's size cell is "—", because the number Kubo returns is not the total of the files inside. The narrow layout (under 720px, including 390) turns each row into a compact card: the name and the menu on the first line, then size and modified time in muted type on the second. The content ID is not on the card. It is in the menu, in full, and choosing it copies it. The sort control is a three-column row. The same menu holds Download, Copy local link, Copy content ID, Pin on this node, Rename, Move, Make a copy, and Delete. Pin starts a pin job for that item's content ID. The file stays in this folder. Progress is on the Pins page. The disclosure uses a chevron and the token colors.

Upload is a button and a drop target on the list. Each file has its own row with a percent. If the name exists, the page asks before replacing. New folder, rename, move, copy, and delete are dialogs. Delete is the danger button, and focus starts on Cancel. Empty, loading (skeletons plus "Loading files."), and node-not-running each have their own state. Node-not-running has one button to the Node page. The wording follows the state: stopped, starting, stopping, or needs attention. Errors that are not that state are toasts.

Keyboard: sort buttons, menu items (arrows, Home, End, Escape, Tab closes), dialogs (Escape, focus trap, focus returns), and the upload button. Drag and drop is extra, not the only way.

Copy is in `en` and `zh`. Tokens come from the design system. The files column is wider than the 760px prose column (`min(960px, 100%)`) because five facts do not fit a sentence width. It is still not a full-bleed admin table.

## Auth polish in this slice

Login and setup password fields, including confirm, have a show/hide control. The change-password fields use the same control so the icon is not English-only on that form either. A failed login leaves the username in the field. The setup button stays disabled until the username, password, confirm, and setup token (when required) are acceptable.

## Tests

Go: path table, a fake Kubo RPC for the client (streaming upload, size limit, loopback refusal, proxy ignored, redirects not followed, timeouts), HTTP tests for session, CSRF, 503, and the routes, plus the existing `IPFS_PANEL_INTEGRATION=1` test extended with a real mkdir, write, list, stat, read, move, copy, and remove.

Web: path and sort helpers, the files store, the page states, the password toggle, the kept username, and the disabled setup button.

## Out of scope

- The pins page itself. A row can start a pin job. The page and the API are in [Pins](pins.md).
- A public share link, or any gateway that is not loopback.
- Editing bytes in the browser, folder upload, previews, and version history.
- A quota other than the one size limit.
- Showing port 5001, a multiaddr, or the RPC URL.
- Pagination. A very large folder still returns, with modified times filled for at most 32 entries.
