# Files

English below. 中文在后半部分。Same decisions, two languages.

**Status: implemented.** This file is the spec for the Files page. If the code and this file disagree, this file wins until a pull request changes it. This is roadmap slice 3, the mutable file system part. Pinning stays slice 4, on its own page.

## English

### What this is for

A beginner can put a file on this node, find it again, rename it, move it, make a copy, download it, and delete it. The page says "files", not "MFS". A content ID is shown, and explained in a disclosure, because that is the one IPFS idea the page cannot hide.

The browser still never talks to Kubo. It talks to the manager. The manager calls Kubo's file RPC on loopback, and only the calls this page needs.

### Decisions

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

### API

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

### UI

The Files item in the nav is enabled. Pins is its own page. Scenes stays under "Coming soon".

The page has a title, one sentence, and a closed disclosure for the content ID and the local link. Then a toolbar: Upload (the one primary button), New folder, Refresh, and sort by name, size, or modified time. Sort is a button group, so it works on a phone where the table header is not the control. Name sort keeps folders first. Size and time sort by the value, and a missing time sorts last.

A breadcrumb shows the folder. The root is a home icon with an accessible name. Names wrap. They are not cut off with an ellipsis.

The wide layout is a table: name with a type icon, size, content ID shortened in the middle with Copy, modified time, and an actions menu. A folder's size cell is "—", because the number Kubo returns is not the total of the files inside. The narrow layout (under 720px, including 390) turns each row into a compact card: the name and the menu on the first line, then size and modified time in muted type on the second. The content ID is not on the card. It is in the menu, in full, and choosing it copies it. The sort control is a three-column row. The same menu holds Download, Copy local link, Copy content ID, Pin on this node, Rename, Move, Make a copy, and Delete. Pin starts a pin job for that item's content ID. The file stays in this folder. Progress is on the Pins page. The disclosure uses a chevron and the token colors.

Upload is a button and a drop target on the list. Each file has its own row with a percent. If the name exists, the page asks before replacing. New folder, rename, move, copy, and delete are dialogs. Delete is the danger button, and focus starts on Cancel. Empty, loading (skeletons plus "Loading files."), and node-not-running each have their own state. Node-not-running has one button to the Node page. The wording follows the state: stopped, starting, stopping, or needs attention. Errors that are not that state are toasts.

Keyboard: sort buttons, menu items (arrows, Home, End, Escape, Tab closes), dialogs (Escape, focus trap, focus returns), and the upload button. Drag and drop is extra, not the only way.

Copy is in `en` and `zh`. Tokens come from the design system. The files column is wider than the 760px prose column (`min(960px, 100%)`) because five facts do not fit a sentence width. It is still not a full-bleed admin table.

### Auth polish in this slice

Login and setup password fields, including confirm, have a show/hide control. The change-password fields use the same control so the icon is not English-only on that form either. A failed login leaves the username in the field. The setup button stays disabled until the username, password, confirm, and setup token (when required) are acceptable.

### Tests

Go: path table, a fake Kubo RPC for the client (streaming upload, size limit, loopback refusal, proxy ignored, redirects not followed, timeouts), HTTP tests for session, CSRF, 503, and the routes, plus the existing `IPFS_PANEL_INTEGRATION=1` test extended with a real mkdir, write, list, stat, read, move, copy, and remove.

Web: path and sort helpers, the files store, the page states, the password toggle, the kept username, and the disabled setup button.

### Out of scope

- The pins page itself. A row can start a pin job. The page and the API are in [Pins](pins.md).
- A public share link, or any gateway that is not loopback.
- Editing bytes in the browser, folder upload, previews, and version history.
- A quota other than the one size limit.
- Showing port 5001, a multiaddr, or the RPC URL.
- Pagination. A very large folder still returns, with modified times filled for at most 32 entries.

---

## 中文

**状态：已实现。** 本文是「文件」页的规格。代码和本文不一致时，在修改本文的 pull request 合进来之前，以本文为准。这是路线图第 3 片里可变文件系统的部分。固定（pin）仍是第 4 片，单独一页。

### 这片做什么

新手可以把一个文件放到这个节点上，再找到它，重命名、移动、复制一份、下载、删除。页面说「文件」，不说「MFS」。内容标识会显示，并在一处可展开的说明里解释，因为这是这一页藏不住的那个 IPFS 概念。

浏览器仍然不和 Kubo 说话。它只和 manager 说话。manager 在回环上调用 Kubo 的文件 RPC，而且只调用这一页需要的那些。

### 决定

1. **只用可变文件。** manager 调用 `files/ls`、`files/stat`、`files/mkdir`、`files/write`、`files/mv`、`files/cp`、`files/rm`、`files/read`。不调用 `add`，也不 pin。可变文件系统里的文件会留在这个节点上，直到有人删除，因为 Kubo 保留这棵树。页面不说「已固定」。固定是下一片。

2. **节点停着是 503，不是 409。** 缓存的节点状态不是 `up` 时，所有文件路由返回 **503** `{"error":"node_not_running","state":"<state>"}`。`state` 是 `down`、`starting`、`stopping` 或 `failed`。**409** 只用于 `already_exists`，也就是名字已经被占用。缓存还说 `up` 但连接失败，同样是 503，`state` 为 `down`：节点已经不在了。用 503 是为了让「名字冲突」和「节点没在跑」分得开。

3. **路径是绝对的可变文件路径。** 以 `/` 开头。根是 `/`。manager 拒绝 `..`、`.`、空段、名字前后的空格、`\`、NUL、其他控制字符、`"`、超过 255 字节的名字，以及超过 4096 字节的路径。它不会把 `..` 清掉。清掉等于把穿越藏起来。`/` 不能删除、写入或创建。

4. **大小限制默认 512 MiB。** `-max-file-bytes` / `IPFS_PANEL_MAX_FILE_BYTES`，正整数。`Content-Length` 更大的请求是 **413** `too_large`，不会发给 Kubo。没有长度的正文会边传边停，超过限制就停；这次上传如果不是在替换已有文件，manager 会删掉写到一半的文件。下载是流式的，没有第二条上限，节点上已经有的文件仍然可以取回。目录列表的 JSON 超过 32 MiB 是 `too_large`。manager 不会把整个文件缓存在内存里。

5. **两个超时。** 元数据调用（`ls`、`stat`、`mkdir`、`mv`、`cp`、`rm`）是 20 秒（`-files-timeout` / `IPFS_PANEL_FILES_TIMEOUT`）。上传和下载是 10 分钟（`-files-transfer-timeout` / `IPFS_PANEL_FILES_TRANSFER_TIMEOUT`）。都是 Go 的时长写法。到点是 **504** `timeout`。处理函数使用请求的 context，浏览器断开就会取消对 Kubo 的调用。

6. **RPC 只拨回环。** 每次调用读取 `<repo>/config`。只有全部 API 和网关 multiaddr 都是走 TCP 的 `127.0.0.1` 或 `::1` 时才使用。HTTP 客户端拒绝其他拨号，忽略 `HTTP_PROXY`，也不跟随重定向。浏览器拿不到 5001 端口，也拿不到 RPC 地址。

7. **分享链接是本机网关链接。** 列表里有 `localGateway`，例如 `http://127.0.0.1:8081`。页面拼成 `localGateway + /ipfs/<cid>`，动作叫做「复制本机链接」。提示说明这个链接只在这台电脑上有效。端口是 8081，除非面板为了避开 manager 的端口而挪过网关。页面不把它说成公开分享。

8. **会话和 CSRF 跟认证一样。** 每条文件路由都要会话。POST 要 `X-CSRF-Token`。GET 不要。公开的认证路由不变。`GET /health` 仍然开放。

9. **上传是原始正文。** `POST /api/files/upload?path=`，`Content-Type: application/octet-stream`。manager 把它流式送进 `files/write` 的 multipart，不把整个文件拿在手里。替换已有文件必须带 `overwrite=1`。文件夹不会被替换。`create=true`、`truncate=true`。`parents` 为 false，所以上传不会偷偷造出一串文件夹。

10. **修改时间是尽力而为。** 列表用 `files/ls?long=true`。类型 `1` 是文件夹。其他是文件。这个列表没有修改时间。manager 接着对最多 32 个条目调用 `files/stat`，Kubo 存了 `Mtime` 就抄过来。更大的文件夹仍然会列出。没有时间就省略，页面显示「没有记录」。

11. **删除是递归的，并且要确认。** `files/rm` 带 `recursive=true`。页面会先问，并说明删文件夹会带走里面的一切。面板里不能撤销。

12. **错误是短代码。** 正文是 `{"error":"<code>"}`。`node_not_running` 另有 `state`。Kubo 自己的句子留在 manager 日志里，不展示给新手。

### API

这些都需要会话。POST 需要 CSRF 头。未知的 JSON 字段是 **400** `bad_request`。JSON 正文最多 8 KiB。JSON 的类型不对是 **415**。

| 方法和路径 | 正文或查询 | 成功 |
| --- | --- | --- |
| `GET /api/files?path=/` | `path` 默认 `/` | **200** 列表 |
| `GET /api/files/stat?path=` | 必须有路径 | **200** 一个条目 |
| `GET /api/files/download?path=` | 文件路径 | **200** 字节，`application/octet-stream`，`Content-Disposition: attachment`，`nosniff` |
| `POST /api/files/mkdir` | `{"path"}` | **201** `{"path"}` |
| `POST /api/files/upload?path=&overwrite=` | 原始字节 | **201** 新条目 |
| `POST /api/files/rename` | `{"path","name"}` | **200** `{"path"}` |
| `POST /api/files/move` | `{"from","to"}` | **200** `{"path"}` |
| `POST /api/files/copy` | `{"from","to"}` | **200** `{"path"}` |
| `POST /api/files/delete` | `{"path"}` | **200** `{"path"}` |

`to` 是完整的目标路径，不是「文件夹再猜一个名字」。页面负责拼好。

列表：

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

Kubo 没有时间时省略 `modified`。`type` 是 `file` 或 `directory`。文件夹是空的时 `entries` 为 `[]`，不是 `null`。网关地址不能变成回环 URL 时省略 `localGateway`。页面于是不提供本机链接。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `invalid_path` | 路径或名字被拒绝，包括删除 `/` |
| 400 | `bad_request` | JSON 读不了 |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 404 | `not_found` | Kubo 没有这个文件 |
| 409 | `already_exists` | 名字被占用，或挡住的是文件夹 |
| 413 | `too_large` | 超过大小限制，或列表太大 |
| 415 | `unsupported_media_type` | 类型不对 |
| 502 | `kubo_error` | Kubo 因其他原因拒绝 |
| 503 | `node_not_running` | 节点状态不是 `up`，或连接失败 |
| 503 | `unavailable` | 配置不是回环 RPC，或 manager 失败了 |
| 504 | `timeout` | 到了截止时间 |

### 界面

导航里的「文件」可用。「固定」是单独的一页。「场景」仍在「即将推出」下面。

页面有标题、一句话，以及一处默认收起的说明，用来讲内容标识和本机链接。然后是工具条：上传（唯一的主按钮）、新建文件夹、刷新，以及按名称、大小或修改时间排序。排序是一组按钮，这样手机上不靠表头也能排。按名称排序时文件夹在前。按大小和时间排序时按数值，没有时间的排在最后。

面包屑显示当前文件夹。根是一个家的图标，并带无障碍名称。名字会换行，不用省略号截断。

宽屏是表格：带类型图标的名称、大小、中间缩短并可复制的内容标识、修改时间，以及一个动作菜单。文件夹的大小是「—」，因为 Kubo 给的数字不是里面文件的合计。窄屏（720px 以下，包括 390）每一行变成一张紧凑的卡片：第一行是名称和菜单，第二行是淡色的「大小 · 修改时间」。内容标识不在卡片上。它在菜单里，显示完整值，点一下就复制。排序是三列一行。同一个菜单里有下载、复制本机链接、复制内容标识、在此节点固定、重命名、移动、复制一份和删除。固定会按这个项目的内容标识启动一个固定任务。文件仍留在这个文件夹里。进度在「固定」页。说明用箭头图标和令牌颜色。

上传可以点按钮，也可以把文件拖到列表上。每个文件有自己的一行和百分比。名字已经存在时，页面会先问再替换。新建文件夹、重命名、移动、复制和删除都是对话框。删除用危险按钮，焦点先在「取消」上。空文件夹、加载中（骨架和「正在加载文件。」）以及节点没在跑，各有自己的状态。节点没在跑时，只有一个按钮去节点页。措辞跟着状态走：停了、正在启动、正在停止，或需要处理。其他错误用提示条。

键盘：排序按钮、菜单项（方向键、Home、End、Escape，Tab 关闭）、对话框（Escape、焦点圈定、焦点回到打开处），以及上传按钮。拖放是附加的，不是唯一的办法。

文案在 `en` 和 `zh`。颜色用设计系统的令牌。文件页的栏比 760px 的正文栏宽（`min(960px, 100%)`），因为五列事实放不进一句话的宽度。它仍然不是铺满的管理表格。

### 这一片顺手改的认证细节

登录和设置里的密码框，包括再输入一次，有显示/隐藏。修改密码的框用同一个控件，避免那个图标在中文里只剩英文。登录失败后，用户名留在框里。设置按钮在用户名、密码、确认密码，以及（需要时）设置口令都合格之前保持禁用。

### 测试

Go：路径表；假的 Kubo RPC 测客户端（流式上传、大小限制、拒绝非回环、忽略代理、不跟随重定向、超时）；HTTP 测试覆盖会话、CSRF、503 和各路由；现有的 `IPFS_PANEL_INTEGRATION=1` 测试加上真实的创建文件夹、写入、列表、状态、读取、移动、复制和删除。

界面：路径和排序、文件 store、页面状态、密码显示切换、失败后保留用户名、设置按钮的禁用。

### 不在这一片里

- 固定页本身。一行可以启动一个固定任务。页面和 API 在[固定](pins.md)。
- 公开分享链接，或任何不是回环的网关。
- 在浏览器里改字节、上传整个文件夹、预览、历史版本。
- 除这一条大小限制以外的配额。
- 显示 5001 端口、multiaddr 或 RPC 地址。
- 分页。很大的文件夹仍然会返回，修改时间最多补 32 条。
