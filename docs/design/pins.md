# Pins

English below. 中文在后半部分。Same decisions, two languages.

**Status: implemented.** This file is the spec for the Pins page. If the code and this file disagree, this file wins until a pull request changes it. This is roadmap slice 4. Files stay as in [Files](files.md). A pin is not a folder.

## English

### What this is for

A beginner can see what this node is keeping, ask it to keep something new, and tell it to stop. The page says "pins", and the first sentence says what that means: the node will not drop these bytes to free space.

A content ID can be pasted. A file can be added from this computer and pinned without putting it in Files. A file that is already in Files can be pinned from that row. The browser still never talks to Kubo. The manager calls Kubo's pin RPC on loopback, and only the calls this page needs.

### Decisions

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

### API

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

### UI

The Pins item in the nav is enabled. It sits with Files, not under "Coming soon". Scenes stays under "Coming soon".

The page has a title, one sentence, and a closed disclosure. The disclosure says what pinning means, how it differs from Files, what the two types mean, where names are stored, and why a size can be blank. The disclosure uses a chevron and the token colors. It does not use the browser's default triangle.

Then a toolbar: Add pin (the one primary button) and Refresh. Search matches the name or the CID. A filter chooses all, with links, or this block only. The wide layout is a table: name, content ID shortened in the middle with Copy, type (a plain phrase, with the Kubo word in a second muted line), and size. The narrow layout (under 720px, including 390) is a card: name and menu on the first line, then "type · size" in muted type. The CID is in the menu, not on the card. The same menu holds Copy content ID, Copy local link (when the gateway is known), and Stop keeping.

Add pin is a dialog. Two or three sentences explain pinning before the fields. A switch chooses "Content ID" or "A file on this computer". The content ID field shows a validation error when the text is non-empty and not a CID or `/ipfs/` path. Submit stays disabled until the value is acceptable. The name is optional. "More options" is closed by default and chooses "Keep what it links to" (the default) or "Only this one block". The file side chooses one file, shows upload progress, and does not offer the direct-pin option. An upload does not create a folder entry.

In-progress jobs sit in their own panel: the name or the shortened CID, the plain sentence "Keeping this on the node.", the block count when Kubo has sent one, and Cancel. There is no percent bar. The panel polls while a job is running. When one succeeds, the list reloads.

Stop keeping is a confirmation dialog. The danger button is not the focused one. The copy says the node may drop the bytes later, and that a copy in Files stays until it is deleted there.

Empty, loading (skeletons plus "Loading pins."), no search matches, and node-not-running each have their own state. Node-not-running has one button to the Node page, and the jobs panel stays visible so a fetch can still be cancelled. The wording follows the node state. Other errors are toasts.

Keyboard: filter buttons, menu items (arrows, Home, End, Escape, Tab closes), dialogs (Escape, focus trap, focus returns), and Add pin. Copy is in `en` and `zh`. Tokens come from the design system. The column is `min(960px, 100%)`, the same width as Files.

The Files row menu gains "Pin on this node". The toast says the file also stays in Files, and that progress is on the Pins page.

### Files polish in this slice

The Files disclosure "About content IDs and local links" uses the same chevron and token colors. The breadcrumb root is a home icon. Its accessible name is still the root label ("Top" / "最上层"). On screens under 720px, a file card shows the name on the first line and "size · modified" in muted type on the second. The content ID is not on the card. It is in the menu, in full, and choosing it copies it. The sort control is a three-column row with room between the buttons, not a cramped wrap.

### Tests

Go: a CID table, label file mode and a corrupt file left in place, job start/progress/cancel/too many, a fake Kubo RPC for the client (names accepted, names rejected, progress stream, cancel, upload stream, size limit, loopback refusal, proxy ignored, redirects not followed), and HTTP tests for session, CSRF, 503, validation, and the routes.

Web: CID and name checks, the pins store, the page states (list, empty, stopped, invalid CID, unpin confirm), and the Files menu pin action.

Screenshots, from `apps/web` with `npm run shots`: pins list, the add dialog with a validation error, a job in progress, and empty, in light and dark, English and 中文, at 1440 and 390. The Files list shots at 390 are the compact cards.

### Out of scope

- Listing indirect pins, and a paginated browser for tens of thousands of pins.
- Downloading a pin (`ipfs cat`), previews, and editing bytes.
- Renaming a pin without pinning again.
- Jobs that survive a manager restart.
- A remote pinning service, or any gateway that is not loopback.
- Showing port 5001, a multiaddr, or the RPC URL.

---

## 中文

**状态：已实现。** 本文是「固定」页的规格。代码和本文不一致时，在修改本文的 pull request 合进来之前，以本文为准。这是路线图第 4 片。文件页仍按[文件](files.md)。固定不是文件夹。

### 这片做什么

新手能看到这个节点留着什么，让它再留一份，也能让它停止保留。页面说「固定」，第一句话说明那是什么意思：节点不会为了腾出空间丢掉这些字节。

可以粘贴一个内容标识。也可以从这台电脑添加一个文件并固定，而不放进「文件」。已经在「文件」里的，可以从那一行固定。浏览器仍然不和 Kubo 说话。manager 在回环上调用 Kubo 的 pin RPC，而且只调用这一页需要的那些。

### 决定

1. **只列递归和直接固定。** manager 列出 `recursive` 和 `direct`，不列 `indirect`。间接固定是操作者已经要求保留的东西的子块。把它们列出来，列表会被淹没。两次列表就是两次 `pin/ls`。同一个 CID 既是递归又是直接时，是两行。

2. **Kubo 有名称时，名称来自 Kubo。** manager 调用 `pin/ls`，带 `names=true` 和 `stream=true`。如果 Kubo 拒绝 `names` 参数（HTTP 400，并且消息说这个参数无法识别、未被使用或未知），manager 在这个进程里记住这件事，之后不再带这个参数。面板自己起的名称同时写到 `<data>/pin-names.json`（权限 `0600`，和 `auth.json` 一样先写临时文件再改名）。某一行上，Kubo 的名称非空就用它，否则用标签文件。列表里的 `namesFrom` 是 `kubo` 或 `panel`，熟手能看出是哪一边回答的。标签文件是尽力而为。写不进去，固定本身不算失败。损坏的标签文件不会被覆盖。

3. **按 CID 添加固定是后台任务。** `POST /api/pins` 检查节点、检查目标，然后 **202** 返回任务。它不等 Kubo 从网络取完。manager 随后在自己的 context 上调用 `pin/add`，并带 `progress=true`。进度是 Kubo 报出的块数，不是百分比。取消会中止那个 context，从而中止这次 RPC。任务只在内存里。manager 重启就没了。同时最多跑 4 个。第 5 个是 **409** `too_many`。已结束的任务保留到 40 个，然后丢掉最旧的那个。正在跑的不丢。任务截止时间是 30 分钟（`-pins-timeout` / `IPFS_PANEL_PINS_TIMEOUT`）。到点记在任务上，是 `timeout`，不是一条一直占着的 HTTP 请求。

4. **目标是 CID 或 `/ipfs/` 路径。** manager 接受 CIDv0（`Qm…`，34 字节的 sha2-256 multihash），或 CIDv1 的 base32（`b`）、base58btc（`z`）、base16（`f`）。它检查版本、codec 变长整数，以及 multihash 的长度和摘要一致。它接受 `/ipfs/<cid>` 和 `/ipfs/<cid>/<更多>`，多出来的每一段都必须是单一路径段，不能是 `..`、`.`、反斜杠或控制字符。两端的空白会忽略。`/ipns/`、`http:` 以及其他形状都是 **400** `invalid_cid`。发给 Kubo 的是规范化之后的 CID。base32 和 base16 会改成小写。base58 不改，因为大小写在那里有意义。

5. **可选的名称是标签，不是路径。** 空着就是没有名称。否则最多 255 字节，不能有首尾空白，不能有控制字符，也不能有 `/`、`\` 或 `"`。名称不合格是 **400** `invalid_name`。Kubo 接受 `pin/add` 的 `name` 时，manager 会送上去并记住。Kubo 拒绝这个参数时，manager 不带名称再试一次，并把标签留在面板文件里。

6. **文件行上的「固定」走同一个任务。** `POST /api/pins/from-files` 带一条绝对的可变文件路径，先取这条路径的状态和 CID，再启动固定任务。默认名称是文件或文件夹的名字，前提是这个名字能当标签。默认是递归。文件仍留在「文件」里。之后取消固定不会删除文件。删除文件也不会取消固定。

7. **「添加文件并固定」是流式的 `ipfs add`。** `POST /api/pins/upload?filename=`，`Content-Type: application/octet-stream`。`filename` 是一个可变文件的路径段。可选的 `name` 查询是标签。没写 `name` 时，如果文件名本身是合格标签，就用文件名。manager 把正文流式送进 `add`，带 `pin=true`、`cid-version=1`、`hash=sha2-256`。它不把整个文件放进内存。大小限制和传输截止时间跟文件页一样（`-max-file-bytes`、`-files-transfer-timeout`）。`Content-Length` 已知且超过限制时是 **413** `too_large`，不会发给 Kubo。这条请求会保持打开，因为字节来自浏览器。它不是决定 3 里的网络抓取。这个文件不会出现在「文件」里。成功是 **201** 和这条新的固定。添加成功之后，manager 最多再花 5 秒请 Kubo 把标签存上。这一步失败也没关系，面板上的标签还在，上传仍然算成功。

8. **大小只在便宜的时候填写。** 列表排序之后，manager 对最多 32 条调用 `object/stat`，并抄下 `CumulativeSize`。这一调用读的是根块，不会把整棵 DAG 走完。这些调用加在一起的预算是 8 秒。失败就省略 `size`。超过 32 条的列表仍然返回。排在前面 32 条之后的不带 `size`。没有大小不是零。

9. **节点停着是 503，不是超时。** 缓存的节点状态不是 `up` 时，要跟 Kubo 说话的固定路由返回 **503** `{"error":"node_not_running","state":"<state>"}`。状态和文件页一致。缓存还说 `up` 但连接失败，同样是 503，`state` 为 `down`。`GET /api/pins/jobs` 和取消不需要 Kubo。它们仍然要会话。抓取还在进行时它们也可用，这样操作者能取消。

10. **移除是一次调用，页面上要确认。** `POST /api/pins/remove` 带 CID 和 `type`（`recursive` 或 `direct`），调用 `pin/rm`。成功会删掉这个 CID 的面板标签。Kubo 说这个 CID 只是被间接固定时，是 **409** `indirect`。没有固定是 **404** `not_found`。

11. **会话、CSRF、超时和回环跟文件页一样。** 每条固定路由都要会话。POST 要 `X-CSRF-Token`。GET 不要。元数据（`pin/ls`、`pin/rm`、`object/stat`、可变文件的 stat）用文件页的元数据截止时间，20 秒。上传用文件页的传输截止时间，10 分钟。固定任务用决定 3 的截止时间。RPC 只拨回环，忽略 `HTTP_PROXY`，也不跟随重定向。浏览器拿不到 5001 端口，也拿不到 RPC 地址。列表超过 20,000 条，或 JSON 超过 32 MiB，是 **413** `too_large`。

12. **错误是短代码。** 正文是 `{"error":"<code>"}`。`node_not_running` 另有 `state`。任务失败时，任务上的 `error` 用同一套代码。Kubo 自己的句子留在 manager 日志里。

### API

这些都需要会话。POST 需要 CSRF 头。未知的 JSON 字段是 **400** `bad_request`。JSON 正文最多 8 KiB。JSON 的类型不对是 **415**。

| 方法和路径 | 正文或查询 | 成功 |
| --- | --- | --- |
| `GET /api/pins` | — | **200** 列表 |
| `POST /api/pins` | `{"target","name"?,"recursive"?}` | **202** `{"job"}` |
| `POST /api/pins/remove` | `{"cid","type"}` | **200** `{"cid","type"}` |
| `POST /api/pins/from-files` | `{"path","name"?}` | **202** `{"job"}` |
| `POST /api/pins/upload?filename=&name=` | 原始字节 | **201** 一条固定 |
| `GET /api/pins/jobs` | — | **200** `{"jobs"}` |
| `GET /api/pins/jobs/{id}` | — | **200** 一个任务 |
| `POST /api/pins/jobs/{id}/cancel` | — | **200** 一个任务 |

`target` 是 CID 或 `/ipfs/` 路径。`recursive` 默认 true。`type` 是 `recursive` 或 `direct`。上传必须有 `filename`。`{id}` 是 manager 发出的 32 位十六进制。其他都是 **404** `not_found`。

列表：

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

没有固定时 `pins` 是 `[]`，不是 `null`。没有名称或大小时省略 `name` 和 `size`。网关地址不能变成回环 URL 时省略 `localGateway`。`namesFrom` 是 `kubo` 或 `panel`。`maxBytes` 和文件页是同一条上传限制。

任务：

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

`state` 是 `running`、`succeeded`、`failed` 或 `cancelled`。`progress` 是块数。Kubo 还没报时保持 0。失败或取消之前省略 `error`。没有名称时省略 `name`。在 Kubo 报出真正固定的 CID 之前，`cid` 是规范化之后的目标。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `invalid_cid` | 目标或 CID 被拒绝 |
| 400 | `invalid_name` | 名称被拒绝 |
| 400 | `invalid_path` | 可变文件路径或上传文件名被拒绝 |
| 400 | `bad_request` | JSON 读不了，或缺少 `type` |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 404 | `not_found` | 没有固定，或没有这个任务 |
| 409 | `indirect` | 只是作为别的东西的子块被固定 |
| 409 | `too_many` | 已经有四个固定任务在跑 |
| 413 | `too_large` | 超过大小限制，或列表太大 |
| 415 | `unsupported_media_type` | 类型不对 |
| 502 | `kubo_error` | Kubo 因其他原因拒绝 |
| 503 | `node_not_running` | 节点状态不是 `up`，或连接失败 |
| 503 | `unavailable` | 配置不是回环 RPC，或 manager 失败了 |
| 504 | `timeout` | 元数据或上传到了截止时间 |

任务自己到点时，状态路由仍是 **200**。任务的 `state` 是 `failed`，`error` 是 `timeout`。取消一个已经结束的任务，原样返回。取消一个不存在的 id 是 **404**。

### 界面

导航里的「固定」可用。它和「文件」放在一起，不在「即将推出」下面。「场景」仍在「即将推出」下面。

页面有标题、一句话，以及一处默认收起的说明。说明里讲固定是什么意思、它和「文件」有什么不同、两种类型是什么意思、名称存在哪里，以及为什么大小可以是空的。这处说明用箭头图标和令牌颜色，不用浏览器默认的三角。

然后是工具条：添加固定（唯一的主按钮）和刷新。搜索匹配名称或内容标识。筛选可以选全部、连同它指向的内容，或只这一块。宽屏是表格：名称、中间缩短并可复制的内容标识、类型（一句白话，第二行用淡色写出 Kubo 的词），以及大小。窄屏（720px 以下，包括 390）是卡片：第一行是名称和菜单，然后是淡色的「类型 · 大小」。内容标识不在卡片上，在菜单里。同一个菜单里有复制内容标识、复制本机链接（知道网关时）和不再保留。

添加固定是一个对话框。字段之前用两三句话说明固定是什么。一个切换在「内容标识」和「这台电脑上的文件」之间选择。内容标识框在文字非空、又不是 CID 或 `/ipfs/` 路径时显示校验错误。值合格之前，提交保持禁用。名称可选。「更多选项」默认收起，用来选「连同它指向的内容」（默认）或「只这一块」。文件这一侧选择一个文件，显示上传进度，不提供「只这一块」。上传不会在文件夹里新建一项。

进行中的任务单独放在一块：名称或缩短的内容标识、「正在把这个留在节点上。」、Kubo 报过块数时的块数，以及取消。没有百分比进度条。有任务在跑时，页面会轮询。有一个成功了，列表就重新加载。

不再保留是确认对话框。焦点不在危险按钮上。文案说明节点以后可能会丢掉这些字节，而「文件」里的副本会留到你在那边删除。

空列表、加载中（骨架和「正在加载固定。」）、搜索没有匹配，以及节点没在跑，各有自己的状态。节点没在跑时，只有一个按钮去节点页，任务面板仍然在，这样还能取消正在进行的抓取。措辞跟着节点状态走。其他错误用提示条。

键盘：筛选按钮、菜单项（方向键、Home、End、Escape，Tab 关闭）、对话框（Escape、焦点圈定、焦点回到打开处），以及添加固定。文案在 `en` 和 `zh`。颜色用设计系统的令牌。栏宽是 `min(960px, 100%)`，和文件页一样。

文件行的菜单多一项「在此节点固定」。提示说明这个文件也会留在「文件」里，进度在「固定」页。

### 这一片顺手改的文件页

文件页「关于内容标识和本机链接」这处说明改用同样的箭头和令牌颜色。面包屑的根是一个家的图标。它的无障碍名称仍是根的标签（「Top」/「最上层」）。720px 以下，文件卡片第一行是名称，第二行是淡色的「大小 · 修改时间」。内容标识不在卡片上，在菜单里，显示完整值，点一下就复制。排序是三列一行，按钮之间有空隙，不再挤在一起换行。

### 测试

Go：CID 表；标签文件的权限，以及损坏的文件留在原地；任务的启动、进度、取消、过多；假的 Kubo RPC 测客户端（接受 names、拒绝 names、进度流、取消、流式上传、大小限制、拒绝非回环、忽略代理、不跟随重定向）；HTTP 测试覆盖会话、CSRF、503、校验和各路由。

界面：CID 和名称检查、固定 store、页面状态（列表、空、节点停了、不合格的 CID、取消固定的确认），以及文件菜单里的固定动作。

截图在 `apps/web` 用 `npm run shots`：固定列表、带校验错误的添加对话框、进行中的任务、空列表，浅色和深色，英文和中文，1440 和 390。390 上的文件列表截图就是紧凑卡片。

### 不在这一片里

- 列出间接固定，以及为几万条固定做分页浏览。
- 下载一条固定（`ipfs cat`）、预览、改字节。
- 不重新固定就改名称。
- 熬过 manager 重启的任务。
- 远程固定服务，或任何不是回环的网关。
- 显示 5001 端口、multiaddr 或 RPC 地址。
