# 日志

**中文** · [English](logs.md)

**状态：已实现。** 这是日志页的规格。如果代码和这份文件不一致，以这份文件为准，直到一个 pull request 改它。它是单节点的专家诊断，不是新的编号切片。第二台节点不在这次改动里。永久对等是[永久对等](peering.zh-CN.md)。

## 这是做什么的

新手打开一页，就能看见节点和这个面板正在做什么。专家可以筛选这份记录，跟着它往下走，并在缺一句话的时候提高某个 Kubo 子系统的日志级别。

浏览器仍然不跟 Kubo 说话。它跟 manager 说话。manager 是启动 Kubo 的那个进程，所以它本来就握着守护进程的 stdout 和 stderr。它也写自己的运行日志和审计日志。这一页就是这两路。浏览器拿不到 Kubo RPC 地址，这一页也不拨日志行里出现的任何地址。

## ipfs-webui 实际有什么

对照 ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`。

诊断是 `/diagnostics*`（`ipfs-webui/src/bundles/routes.js:25`）。日志屏是三个诊断工具之一（`ipfs-webui/src/diagnostics/diagnostics-content.tsx:11`，`ipfs-webui/src/diagnostics/diagnostics-content.tsx:106-108`）。它用 `ipfs.log.tail` 跟踪 Kubo（`ipfs-webui/src/contexts/logs/logs-context.tsx:198`），也就是 Kubo 的 `log/tail`。播放和暂停是开、关那条流（`ipfs-webui/src/diagnostics/logs-screen/log-viewer.tsx:90-97`）。清空丢掉浏览器自己存的条目（`ipfs-webui/src/contexts/logs/logs-context.tsx:211-217`）。另一组控件用 `ipfs.log.level` 读写子系统级别（`ipfs-webui/src/contexts/logs/api.ts:59-75`）。浏览器握着一个 Kubo RPC 客户端。

这一页留下这件事：一份实时日志、暂停、清空，以及一次专家级别调整。它不留下那个 RPC 客户端、`log/tail`、检索检查，或 DHT provide。

## 来源

两个来源，没有第三个。

| 来源 | 是什么 |
| --- | --- |
| `manager` | manager 自己的 slog：运行行，以及 `msg=audit` 的审计行。 |
| `kubo` | 这个 manager 拉起的守护进程的 stdout 和 stderr。 |

`log/tail` 是实验性的（`core/commands/log.go:242-243`）。它是同一守护进程的第二路输出，走的是这个面板不暴露的 RPC。这一页不调用它。

manager 的 slog 处理器记下 `Info` 及以上。manager 的 `Debug` 不留。Kubo 的行按守护进程实际写出的级别保留，专家调高之后也包括 `debug`。

短命的 `ipfs` 命令（`init`、`id`、`config`）的 stdout 和 stderr 仍走[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)已经用来填 `error.detail` 的那条路。它们不抄到这一页。给那个 detail 用的 stderr 尾巴仍然最多 2000 字节，和这份日志互不相干。

manager 收养一个不是它启动的守护进程时，没有管道。`capture` 为假。页面会说明。更早一次启动留下的历史行还在。

## 保留

行放在两处，都在 `<data>/logs/` 下。那是 manager 的数据目录，不是 Kubo 仓库。

- 环形缓冲每个来源留最新的 2000 行。实时页面读这个环。
- `manager.log` 和 `kubo.log` 把同样的行写到磁盘。一个文件到 1 MiB 就轮转。当前文件再加两份旧的（`.log`、`.log.1`、`.log.2`）。更旧的字节删掉。目录模式 `0700`。每个文件模式 `0600`。

超过 2048 字节的行在 UTF-8 边界截断。截掉的后半段成为下一行。空行丢掉。

启动时 manager 把当前文件的末尾读回环里，最多 2000 行。这一进程的 id 从 1 开始。manager 重启之后 id 不作数。

文件写失败不会丢掉环，也不会停掉守护进程。磁盘重新接受写入之前，下载和环可能不一致。

环是实时视图。调试输出可以把更旧的行挤出环。轮转文件在自己被清掉之前仍留着它们。

## 涂掉秘密

一行在进入环或文件之前先涂掉。重放文件时再涂一次。

这些 slog 属性名，以及分组键的最后一段，不区分大小写，值换成 `[redacted]`：`setup_token`、`password`、`privkey`、`privatekey`、`private_key`、`authorization`、`authorizations`、`csrf`、`csrf_token`、`session`、`cookie`、`token`、`secret`、`apikey`、`api_key`。

每一行的正文，包括 Kubo 的，再扫一遍：

- `PrivKey` 或 `private key` 的赋值，JSON 或 `key=value`
- `setup_token=…`
- 如果 stdout 那句 `ipfs-panel setup token: …` 被抄进了 slog
- `ipfs_panel_session=…`
- `X-CSRF-Token` 的赋值
- `Bearer` 凭证
- `api.key=…`

一次性安装令牌打在 manager 自己的 stdout 上，留在 stdout。它不被收进来。点名 `setup_token` 的那条 slog 警告会进来，值已被涂掉。审计行保持原样：事件、结果、用户名、IP。它们不带文档、密码或会话 cookie。

这一页不自己添一个 Kubo RPC 地址，也不把一行变成链接或拨号。它不删掉守护进程自己写下的监听地址。那句话就是日志。

## 读取

会话与[认证](auth.zh-CN.md)一致。GET 需要会话，不需要 CSRF 头。POST 需要 `Content-Type: application/json` 和 `X-CSRF-Token`。未知 JSON 字段是 **400** `bad_request`。

`GET /api/logs` 不调用 Kubo。节点停着、正在启动，或从未启动，它都回答。

查询参数：`after`、`source`（`all`、`manager`、`kubo`）、`level`（`debug`、`info`、`warn`、`error`）、`q`、`limit`。

默认：`source=all`、`level=info`、`limit=500`、`after=0`、`q` 为空。`level` 是下限：`warn` 返回警告和错误。`q` 是消息的大小写不敏感子串，最多 200 个码位，不匹配时间戳或来源。`limit` 超过 1000、负的 `after`、未知的来源或级别，或更长的 `q`，是 **400** `bad_request`。

`after=0` 返回最新的匹配行，最多 `limit` 条。`after` 大于零时，返回这个 id 之后的下一批，从旧到新，最多 `limit` 条，这样客户端可以按顺序补上。没有行时 `lines` 是 `[]`，不是 `null`。`next` 是返回的最后一条 id；这一页为空时，`next` 就是 `after`。

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

`time` 是 UTC 的 RFC3339Nano。`node` 是缓存的节点状态。`capture` 仅当当前子进程是这个 manager 拉起的、并且管道还开着时为真。

`GET /api/logs/stream` 是服务器发送事件，查询参数相同，会话 cookie 相同。`Content-Type` 是 `text/event-stream`。每一行是：

```text
id: 12
event: line
data: {"id":12,"time":"…","source":"kubo","level":"info","text":"…"}
```

开始时，以及大约每 15 秒，有一条 `status` 事件，正文是 `{"node","capture"}`，并带一条注释 ping，免得代理掐掉空闲的流。`gap` 事件表示浏览器太慢，实时缓冲丢了一行。客户端用 `GET /api/logs?after=<最后的 id>` 再取。已经转出环的行，在文件自己被轮转掉之前仍在文件里。

`GET /api/logs/download?source=all|manager|kubo` 是 `text/plain; charset=utf-8`，附件名是 `ipfs-panel-logs.txt`、`ipfs-panel-manager.txt` 或 `ipfs-panel-kubo.txt`。它先刷盘，再按从旧到新发送轮转文件。单一来源的行是存盘格式 `time<TAB>level<TAB>text`。`all` 在前面加上 `manager` 或 `kubo` 和一个制表符。这些字节在写入时已经涂过。

没有删除文件的请求。清空视图是浏览器里的动作。

## 专家级别

`POST /api/logs/level`，正文 `{"subsystem","level"}`，调用 Kubo 的 `log/level`（`core/commands/log.go:126-155`）。节点必须是 `up`。否则是 **503** `node_not_running`，带 `state`，并且不调用 Kubo。

`subsystem` 是 `all`、`*`，或一个匹配 `^[A-Za-z0-9][A-Za-z0-9_./-]{0,63}$` 的记号。Kubo 把 `all` 和 `*` 当成全部子系统（`core/commands/log.go:15-18`，`core/commands/log.go:137-138`）。`level` 是 `debug`、`info`、`warn`、`error`、`dpanic`、`panic`、`fatal` 或 `default`。其他的是 **400** `bad_subsystem` 或 **400** `bad_level`，并且不调用 Kubo。

成功是 **200** `{"subsystem","level"}`。manager 不重启节点。Kubo 的非 200 是 **502** `kubo_rejected`。`detail` 是 Kubo 的消息，截到 240 个字符，控制字符去掉。超时是 **504** `timeout`。期限是 10 秒。go-log v2.9.2 里，对一个还不存在的子系统名调用 `log/level` 不会失败：级别会存着，等日志器创建时再用。这个面板不把这种情况当成错误。

`GET /api/logs/levels` 调用 `log/ls`（`core/commands/log.go:220-229`）。成功是 **200** `{"subsystems":["…"]}`，排序后的名字。列表是 `Strings` 字段（`core/commands/swarm.go:42-44`）。页面不去取每个子系统的当前级别。同样的 `up` 规则和同样的错误适用。正文超过 1 MiB 是 **413** `too_large`。

审计行是结构化 slog，`msg=audit`，`event` 为 `log_level`，再加上 `result`、`username`、`ip`、`subsystem` 和 `level`。结果包括 `success`、`invalid`、`rejected` 和 `unavailable`。读取不是审计事件。

## Kubo 行的形状

manager 的管道不是终端，所以 go-log v2.9.2 写出纯文本控制台行：时间、级别、日志器名、调用方、消息，用制表符分开。先去掉 ANSI。认得出的级别（`DEBUG`、`INFO`、`WARN`、`WARNING`、`ERROR`、`DPANIC`、`PANIC`、`FATAL`）变成 `debug`、`info`、`warn` 或 `error`（`DPANIC`、`PANIC`、`FATAL` 都是 `error`）。调用方字段（`file.go:line`）丢掉。存下来的正文是日志器名加消息。时间戳能按 RFC3339 解析时，就是这一行的时间。带 `level` 和 `msg` 的 JSON 行用那两个字段。其余整行保留，级别算 `info`，时间是 manager 读到它的时间。消息不翻译。

## API 错误

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `bad_request` | 查询不对、JSON 不对、未知字段 |
| 400 | `bad_level` | 级别不在允许列表里 |
| 400 | `bad_subsystem` | 子系统不在允许列表里 |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 413 | `too_large` | `log/ls` 正文超过 1 MiB |
| 415 | `unsupported_media_type` | POST 的内容类型不对 |
| 502 | `kubo_rejected` | Kubo 拒绝了这个级别。`detail` 可能有 |
| 503 | `node_not_running` | 节点不在运行时调用级别接口。带 `state` |
| 503 | `unavailable` | 日志存储不存在，或节点快照失败 |
| 504 | `timeout` | Kubo 没有及时回答 |

`GET /api/logs`、流，以及下载，不返回 `node_not_running`。停着的节点仍然有 manager 的日志。

## 界面

导航项是日志，在场景之后、设置之前。设置仍在侧栏底部。页面是紧凑的标题、一句话，然后一块铺满窗口的工作面。工具条是这一面上可换行的一行：来源、级别、查找、跟随或暂停、复制、下载、清空视图，以及级别。下面的列表是等宽 13px，一行一个固定高度，DOM 里只有可见行。一行是时间戳、级别标签、来源标签，和原始消息。行本身横向滚动。列表纵向滚动，一直铺到窗口底部。

跟随是默认。视图贴在底部。暂停保持连接，不再贴底。用手滚动不会暂停。清空视图丢掉浏览器正在显示的，文件不动。复制把视图里的行写成时间、原始级别、原始来源和原始正文。下载通过 manager 取文件。查找有防抖，作为 `q` 送出。改来源、级别或查找会重新取末尾，并重新打开流。

级别标签用状态色的淡底。萤光绿不是级别，也不是选区。跟随中的标签用专家小标签那一套。暂停是中性的。

级别打开一个标成专家的对话框。它要一个子系统（默认 `all`）和一个级别，并说明节点不会重启。节点不在运行时它不可用。对话框不是这一页的主体。

日志消息不翻译。界面文字翻译，在 `en` 和 `zh`。

| 状态 | 页面做什么 |
| --- | --- |
| 载入中 | 工具条，然后骨架行。不是一份空日志。 |
| 流动 | 有行，跟随中的标签，贴在底部。 |
| 暂停 | 同样的行，暂停标签，滚动位置留在原地。 |
| 已筛选 | 查找、来源或级别在收窄列表。没有匹配是单独的一句，外加一个清除筛选的控件。 |
| 空 | 节点在运行，没有匹配的内容。 |
| 节点已停止 | 节点不在运行。没有行时，是空状态和一条去节点页的链接。有行时，显示那些行，外加一句它们是更早的输出。正在启动、正在停止、失败，各有自己的一句。 |
| 收养 | 节点在运行且 `capture` 为假。一句：这个面板没有启动这个进程，所以读不到输出。改级别仍然可以。 |
| 错误 | 按代码给的那一句，以及重试。有 `detail` 时放在这句话下面。 |

浅色和深色，1440 和 390，都是真实布局：工具条换行，列表保住剩下的高度，对话框适应窄宽度。

## 测试

Go：涂掉私钥、安装令牌、会话 cookie 和 bearer 凭证，普通行保持原样；环丢掉最旧的一行；轮转在 Unix 上留下三个仅所有者的文件，在 Windows 上留下一个可写文件；按来源、级别和文本查询；一条 go-log 纯文本行能解析，认不出的行被保留；slog 处理器涂掉 `setup_token`；HTTP 会话和 CSRF；停着的节点能读 manager 的日志，不能设置级别（`node_not_running`）；坏的级别不调用 Kubo；下载里有 `[redacted]`，没有那个秘密。

发行版二进制的测试基线在 `TestRealKuboOptional` 里，把 supervisor 的守护进程管道指到一个存储。节点起来之后，至少有一条 kubo 行。`log/ls` 返回非空的 `Strings` 列表。把 `core` 的 `log/level` 设成 `error`，会返回 Kubo 那句级别已改的话。supervisor 到达 `down` 之后，同一次调用返回 `node_not_running`。裁定过的句子在[测试基线](harness.zh-CN.md)。

Web：导航文字、流动中的行、暂停、没有匹配的筛选、节点已停止的那句、错误那句、中文界面，以及一条消息按文本渲染而不是 HTML。

## 不在范围内

- 永久对等。那是[永久对等](peering.zh-CN.md)。
- 第二个节点。
- `log/tail`、检索检查，以及 DHT provide。
- 从页面删除日志文件。
- 翻译节点写下的那些行。
