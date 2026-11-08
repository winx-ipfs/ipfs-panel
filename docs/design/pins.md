# Pins

**English** · [中文](pins.zh-CN.md)

**Status: implemented.** This file is the spec for the Pins page. If the code and this file disagree, this file wins until a pull request changes it. This is roadmap slice 4. Files stay as in [Files](files.md). A pin is not a folder.

## What this is for

A beginner can see what this node is keeping, ask it to keep something new, and tell it to stop. The page says "pins", and the first sentence says what that means: the node will not drop these bytes to free space.

A content ID can be pasted. A file can be added from this computer and pinned without putting it in Files. A file that is already in Files can be pinned from that row. The browser still never talks to Kubo. The manager calls Kubo's pin RPC on loopback, and only the calls this page needs.

## Decisions

1. **Recursive and direct pins only.** The manager lists `recursive` and `direct`. It does not list `indirect`. Indirect pins are children of something the operator already asked to keep. Showing them would bury the list. The two lists are two `pin/ls` calls. A CID that is both recursive and direct is two rows.

2. **Names come from Kubo when Kubo has them.** The manager calls `pin/ls` with `names=true` and `stream=true`. If Kubo rejects the `names` argument (HTTP 400 whose message says the argument is unrecognized, unused, or unknown), the manager remembers that for the process and lists without it. Names the panel sets are also written to `<data>/pin-names.json` (mode `0600`, same atomic write as `auth.json`). On a row, Kubo's name wins when it is non-empty. Otherwise the label file is used. The list says `namesFrom` `kubo` or `panel` so an expert can see which store answered. The file is best-effort. A pin is not failed because the label file could not be written. A corrupt label file is not overwritten.

3. **Adding a pin by CID is a background job.** `POST /api/pins` checks the node, checks the target, and returns **202** with the job. It does not wait for Kubo to finish fetching. The manager then calls `pin/add` with `progress=true` on its own context. Progress is the block count Kubo sends. It is not a percent. Cancel aborts that context, which aborts the RPC. Jobs live in memory. A manager restart drops them. At most 4 jobs run at once. A fifth is **409** `too_many`. Finished jobs are kept until 40 of them exist, then the oldest finished job is dropped. Running jobs are not dropped. The job deadline is 30 minutes (`-pins-timeout` / `IPFS_PANEL_PINS_TIMEOUT`). A deadline is `timeout` on the job, not a held HTTP request.

4. **Targets are a CID or an `/ipfs/` path.** The manager accepts a CIDv0 (`Qm…`, the 34-byte sha2-256 multihash), or a CIDv1 in base32 (`b`), base58btc (`z`), or base16 (`f`). It checks the version, the codec varint, and that the multihash length matches the digest. It accepts `/ipfs/<cid>` and `/ipfs/<cid>/<more>` when each extra segment is a single path segment without `..`, `.`, a backslash, or a control character. Surrounding whitespace is ignored. `/ipns/`, `http:`, and any other shape are **400** `invalid_cid`. The normalized CID is what is sent to Kubo. Base32 and base16 are lowercased. Base58 is not, because case matters there.

5. **An optional name is a label, not a path.** Empty means unnamed. Otherwise at most 255 bytes, no leading or trailing space, no control characters, and no `/`, `\`, or `"`. A bad name is **400** `invalid_name`. When Kubo accepts `pin/add` `name`, the manager sends it and remembers that. When Kubo rejects that argument, the manager retries once without the name and keeps the label in the panel file.

6. **"Pin" on a file row starts the same job.** `POST /api/pins/from-files` with an absolute mutable-file path stats that path, takes its CID, and starts a pin job. The default name is the file or folder name when that name is a valid label. The default is recursive. The file stays in Files. Unpinning later does not delete the file. Deleting the file does not unpin.

7. **"Add file and pin" is `ipfs add`, streamed.** `POST /api/pins/upload?filename=` with `Content-Type: application/octet-stream`. `filename` is one mutable-file segment. The optional `name` query is the label. When `name` is omitted, the filename is the label if it is a valid label. The manager streams the body into `add` with `pin=true`, `cid-version=1`, and `hash=sha2-256`. It does not buffer the file. The size limit and the transfer deadline are the files limits (`-max-file-bytes`, `-files-transfer-timeout`). A known `Content-Length` over the limit is **413** `too_large` and is not sent to Kubo. This request stays open because the bytes are coming from the browser. It is not the network fetch in decision 3. The file does not appear under Files. The response is **201** and the new pin. After a successful add, the manager spends at most 5 seconds asking Kubo to store the label. If that fails, the panel label still stands and the upload still succeeds.

8. **Size is filled in when it is cheap.** After the list is sorted, the manager calls `object/stat` for at most 32 pins and copies `CumulativeSize`. That call reads the root block. It does not walk the whole DAG. The budget for all of those calls together is 8 seconds. A miss omits `size`. A list longer than 32 still returns. The pins past the first 32 omit `size`. A missing size is not zero.

9. **A stopped node is 503, not a timeout.** When the cached node state is not `up`, pin routes that talk to Kubo return **503** `{"error":"node_not_running","state":"<state>"}`. The states match Files. A dial that fails after the cache said `up` is the same 503 with `state` `down`. `GET /api/pins/jobs` and cancel do not need Kubo. They still need a session. They keep working while a fetch is in flight so the operator can cancel it.

10. **Remove is one call, and it is confirmed in the page.** `POST /api/pins/remove` with the CID and `type` (`recursive` or `direct`) calls `pin/rm`. Success deletes the panel label for that CID. Kubo saying the CID is pinned only indirectly is **409** `indirect`. Not pinned is **404** `not_found`.

11. **Session, CSRF, timeouts, and loopback match Files.** Every pins route needs a session. POST needs `X-CSRF-Token`. GET does not. Metadata (`pin/ls`, `pin/rm`, `object/stat`, the mutable-file stat) uses the files metadata deadline, 20 seconds. Upload uses the files transfer deadline, 10 minutes. The pin job uses the pin deadline in decision 3. The RPC dial is loopback only, ignores `HTTP_PROXY`, and does not follow redirects. The browser never receives port 5001 or the RPC URL. A listing over 20,000 pins or 32 MiB of JSON is **413** `too_large`.

12. **Errors are short codes.** The body is `{"error":"<code>"}`. `node_not_running` also has `state`. A job carries `error` the same way when it fails. Kubo's own message stays in the manager log.

## API

All of these need a session. POST needs the CSRF header. Unknown JSON fields are **400** `bad_request`. JSON bodies are at most 8 KiB. A wrong JSON content type is **415**.

| Method and path | Body or query | Success |
| --- | --- | --- |
| `GET /api/pins` | — | **200** list |
| `POST /api/pins` | `{"target","name"?,"recursive"?}` | **202** `{"job"}` |
| `POST /api/pins/remove` | `{"cid","type"}` | **200** `{"cid","type"}` |
| `POST /api/pins/from-files` | `{"path","name"?}` | **202** `{"job"}` |
| `POST /api/pins/upload?filename=&name=` | raw bytes | **201** one pin |
| `GET /api/pins/jobs` | — | **200** `{"jobs"}` |
| `GET /api/pins/jobs/{id}` | — | **200** one job |
| `POST /api/pins/jobs/{id}/cancel` | — | **200** one job |

`target` is a CID or an `/ipfs/` path. `recursive` defaults to true. `type` is `recursive` or `direct`. `filename` is required on upload. `{id}` is the 32 hex characters the manager issued. Anything else is **404** `not_found`.

List:

```json
{
  "namesFrom": "kubo",
  "localGateway": "http://127.0.0.1:8081",
  "maxBytes": 536870912,
  "pins": [
    {
      "cid": "bafy...",
      "name": "notes",
      "type": "recursive",
      "size": 1200
    }
  ]
}
```

`pins` is `[]` when nothing is pinned, never `null`. `name` and `size` are omitted when unknown. `localGateway` is omitted when the gateway address cannot be turned into a loopback URL. `namesFrom` is `kubo` or `panel`. `maxBytes` is the same upload limit as Files.

Job:

```json
{
  "id": "0123456789abcdef0123456789abcdef",
  "cid": "bafy...",
  "name": "notes",
  "type": "recursive",
  "state": "running",
  "progress": 12,
  "created": "2026-11-01T12:00:00Z"
}
```

`state` is `running`, `succeeded`, `failed`, or `cancelled`. `progress` is a block count. It stays 0 when Kubo has not sent one. `error` is omitted until the job fails or is cancelled. `name` is omitted when empty. `cid` is the normalized target until Kubo reports the pinned CID.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `invalid_cid` | Target or CID rejected |
| 400 | `invalid_name` | Name rejected |
| 400 | `invalid_path` | Mutable-file path or upload filename rejected |
| 400 | `bad_request` | JSON the handler cannot read, or `type` is missing |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 404 | `not_found` | Not pinned, or no such job |
| 409 | `indirect` | Pinned only as a child of something else |
| 409 | `too_many` | Four pin jobs are already running |
| 413 | `too_large` | Over the size limit, or a huge listing |
| 415 | `unsupported_media_type` | Wrong content type |
| 502 | `kubo_error` | Kubo refused for another reason |
| 503 | `node_not_running` | Node state is not `up`, or the dial failed |
| 503 | `unavailable` | Config is not a loopback RPC, or the manager failed |
| 504 | `timeout` | The metadata or upload deadline elapsed |

A job that hits its own deadline stays **200** on the status route. The job's `state` is `failed` and its `error` is `timeout`. Cancel of a finished job returns the job unchanged. Cancel of an unknown id is **404**.

## UI

The Pins item in the nav is enabled. It sits with Files, not under "Coming soon". Scenes stays under "Coming soon".

The page has a title, one sentence, and a closed disclosure. The disclosure says what pinning means, how it differs from Files, what the two types mean, where names are stored, and why a size can be blank. The disclosure uses a chevron and the token colors. It does not use the browser's default triangle.

Then a toolbar: Add pin (the one primary button) and Refresh. Search matches the name or the CID. A filter chooses all, with links, or this block only. The wide layout is a table: name, content ID shortened in the middle with Copy, type (a plain phrase, with the Kubo word in a second muted line), and size. The narrow layout (under 720px, including 390) is a card: name and menu on the first line, then "type · size" in muted type. The CID is in the menu, not on the card. The same menu holds Copy content ID, Copy local link (when the gateway is known), and Stop keeping.

Add pin is a dialog. Two or three sentences explain pinning before the fields. A switch chooses "Content ID" or "A file on this computer". The content ID field shows a validation error when the text is non-empty and not a CID or `/ipfs/` path. Submit stays disabled until the value is acceptable. The name is optional. "More options" is closed by default and chooses "Keep what it links to" (the default) or "Only this one block". The file side chooses one file, shows upload progress, and does not offer the direct-pin option. An upload does not create a folder entry.

In-progress jobs sit in their own panel: the name or the shortened CID, the plain sentence "Keeping this on the node.", the block count when Kubo has sent one, and Cancel. There is no percent bar. The panel polls while a job is running. When one succeeds, the list reloads.

Stop keeping is a confirmation dialog. The danger button is not the focused one. The copy says the node may drop the bytes later, and that a copy in Files stays until it is deleted there.

Empty, loading (skeletons plus "Loading pins."), no search matches, and node-not-running each have their own state. Node-not-running has one button to the Node page, and the jobs panel stays visible so a fetch can still be cancelled. The wording follows the node state. Other errors are toasts.

Keyboard: filter buttons, menu items (arrows, Home, End, Escape, Tab closes), dialogs (Escape, focus trap, focus returns), and Add pin. Copy is in `en` and `zh`. Tokens come from the design system. The column is `min(960px, 100%)`, the same width as Files.

The Files row menu gains "Pin on this node". The toast says the file also stays in Files, and that progress is on the Pins page.

## Files polish in this slice

The Files disclosure "About content IDs and local links" uses the same chevron and token colors. The breadcrumb root is a home icon. Its accessible name is still the root label ("Top" / "最上层"). On screens under 720px, a file card shows the name on the first line and "size · modified" in muted type on the second. The content ID is not on the card. It is in the menu, in full, and choosing it copies it. The sort control is a three-column row with room between the buttons, not a cramped wrap.

## Tests

Go: a CID table, label file mode and a corrupt file left in place, job start/progress/cancel/too many, a fake Kubo RPC for the client (names accepted, names rejected, progress stream, cancel, upload stream, size limit, loopback refusal, proxy ignored, redirects not followed), and HTTP tests for session, CSRF, 503, validation, and the routes.

Web: CID and name checks, the pins store, the page states (list, empty, stopped, invalid CID, unpin confirm), and the Files menu pin action.

Screenshots, from `apps/web` with `npm run shots`: pins list, the add dialog with a validation error, a job in progress, and empty, in light and dark, English and 中文, at 1440 and 390. The Files list shots at 390 are the compact cards.

## Out of scope

- Listing indirect pins, and a paginated browser for tens of thousands of pins.
- Downloading a pin (`ipfs cat`), previews, and editing bytes.
- Renaming a pin without pinning again.
- Jobs that survive a manager restart.
- A remote pinning service, or any gateway that is not loopback.
- Showing port 5001, a multiaddr, or the RPC URL.
