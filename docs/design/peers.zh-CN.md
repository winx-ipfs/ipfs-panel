# 对等节点

**中文** · [English](peers.md)

**状态：已实现。** 本文是「对等节点」页的规格。代码和本文不一致时，在修改本文的 pull request 合进来之前，以本文为准。它是探索之后下一块 webui 差距，不是路线图里新的一片，也不是专家模式。

## 这片做什么

新手能看见这个节点连着哪些其他节点，是一份列表，不只是一个数量，也可以粘贴一个地址再拨一个。页面说「对等节点」。它不画地图，也不把地址写进 Kubo 的配置。

浏览器仍然不和 Kubo 说话。它只和 manager 说话。manager 在回环上调用 Kubo，而且只调用这一页需要的那些。页面拿不到 Kubo RPC 地址。已经连上的那些节点的地址不进浏览器，这和节点页给数量做的决定是同一条。

## ipfs-webui 实际有什么

对照的是 ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`。

路由是 `/peers`（`ipfs-webui/src/bundles/routes.js:20`）。导航项排在探索后面（`ipfs-webui/src/navigation/NavBar.js:84`）。

这一页是三块（`ipfs-webui/src/peers/PeersPage.js:39-46`）：一颗「添加连接」按钮、一张世界地图、一张节点表。命令行教学那一行给出 `ipfs swarm connect`。按斜杠键会聚焦到筛选框。还有一次产品导览。

列表来自 `swarm.peers`，带 `verbose`、`identify` 和超时（`ipfs-webui/src/bundles/peers.js:9`）。在这条路由上大约每 5 秒刷新一次（`ipfs-webui/src/bundles/peers.js:92`）。每一行是一条连接。一行留下 peer id、多地址、从这条多地址解析出的传输字符串、延迟、方向、打开的流的名字、代理版本，以及一个位置（`ipfs-webui/src/bundles/peer-locations.js:136-166`）。表的列是位置、延迟、peer id、连接、代理版本和打开的流（`ipfs-webui/src/peers/PeersTable/PeersTable.js:210-216`）。连接那一格复制的是 `address + /p2p/ + peerId`（`ipfs-webui/src/peers/PeersTable/PeersTable.js:94-110`）。

webui 里的地图不是装饰。它从节点的多地址里取出公网 IP（`ipfs-webui/src/bundles/peer-locations.js:335-337`），经网关上的 `ipfs-geoip` 查询（`ipfs-webui/src/bundles/peer-locations.js:367`），再把坐标画出来（`ipfs-webui/src/peers/WorldMap/WorldMap.js:85-88`，`ipfs-webui/src/peers/WorldMap/WorldMap.js:154-160`）。私有地址改标成「本地网络」。

添加连接会检查这段文字是不是一条多地址，并且对得上 p2p、电路或 WebRTC 的形状（`ipfs-webui/src/peers/AddConnection/AddConnection.js:15-21`）。然后调用 `swarm.connect`。「永久」那个框默认是开的（`ipfs-webui/src/peers/AddConnection/AddConnection.js:29`，`ipfs-webui/src/peers/AddConnection/AddConnection.js:127-130`），并用 `config.set` 写入 `Peering.Peers`（`ipfs-webui/src/bundles/peers.js:40-61`）。

这一页留下列表和一次性拨号。不留下地图、多地址、位置、传输字符串、流的名字、命令行教学、导览，以及永久那个框。

## 决定

1. **没有地图。** 一张忠实的地图需要对方的 IP，而那个 IP 就是这个面板已经丢掉的 `Addr` 字段（`core/commands/swarm.go:433`）。凭空猜一个国家，就是在编造位置。这一片不调用 `ipfs-geoip`，也不交付地图。

2. **一个 peer id 一行。** `swarm/peers` 为每条打开的连接返回一个对象（`core/commands/swarm.go:265-308`）。节点页的数量是不同的 `Peer` 值的个数（`internal/activity/activity.go:63-70`）。这一页用同样的不同计数，所以节点上的数字和这里的 `total` 可以对上。连到同一个节点的两条连接合成一行。`latencyMs` 是解析出来的最小延迟。已知连接全是入站时 `direction` 为 `in`，全是出站时为 `out`，两种都有时为 `both`。`agent` 是第一条非空的代理版本。

3. **一行可以有什么。** manager 调用 `swarm/peers` 时带 `latency=true`、`direction=true` 和 `identify=true`。这些旗标填上延迟、方向和 identify（`core/commands/swarm.go:271-306`）。`verbose` 不开，这样响应里不会再塞满流。浏览器收到 `peerId`，有的话再收到 `direction`、`latencyMs` 和 `agent`。它收不到 `Addr`、`Muxer`、`Streams`，也收不到 `Identify` 里除了 `AgentVersion` 以外的字段。Identify 还带地址和公钥（`core/commands/id.go:29-35`，复制发生在 `core/commands/swarm.go:457-476`）。那些留在 manager 进程里，也不写进日志。

4. **延迟和方向是页面说得出口的数。** Kubo 把延迟发成 Go 的时长字符串；时长为 0 时发 `n/a`（`core/commands/swarm.go:282-285`）。`n/a` 以及 `time.ParseDuration` 不接受的值，省略 `latencyMs`。正的时长变成整数毫秒，四舍五入。不足 0.5 毫秒变成 `0`，页面说「< 1 毫秒」。方向是 `inet.Direction`（`core/commands/swarm.go:436`）。Kubo 印出 inbound 和 outbound（`core/commands/swarm.go:497-505`）。webui 把 JSON 的 `1` 当成入站、`2` 当作出站（`ipfs-webui/src/peers/PeersTable/PeersTable.js:225-236`）。对照过 go-libp2p v0.50.0，也就是 `go.mod:54` 里的版本：这两个常量就是 1 和 2。其他值在那条连接上忽略。代理版本先去掉两端空白，丢掉控制字符和格式字符，再截到 128 个码位，这是 Kubo 自己的上限（`core/commands/cmdutils/sanitize.go:8`，`core/commands/cmdutils/sanitize.go:17-46`）。结果为空就省略 `agent`。

5. **列表有上限。** 行按延迟排序，未知的排在最后，然后按 peer id。最多返回 500 行。`total` 是截断之前的不同计数。`total` 大于行数时 `truncated` 为 true。没有节点时 `peers` 是 `[]`，不是 `null`。Kubo 的正文超过 8 MiB 时拒绝，**413** `too_large`。列表的期限是 20 秒。

6. **节点停着是 503。** 缓存的节点状态不是 `up` 时，`GET /api/peers` 和 `POST /api/peers/connect` 返回 **503** `{"error":"node_not_running","state":"<state>"}`。缓存还说 `up` 但连接失败，同样是 503，`state` 为 `down`。处理函数使用请求的 context，浏览器断开就会取消对 Kubo 的调用。

7. **RPC 只拨回环。** 调用读取 `<repo>/config`。只有 API 地址是带端口的 `http` 回环来源时才使用。客户端拒绝其他拨号，忽略 `HTTP_PROXY`，也不跟随重定向。响应里没有 API 地址，没有 `5001` 端口，也没有多地址。

8. **连接只拨一次。** `POST /api/peers/connect` 接受一个 JSON 对象，`{"address":"<multiaddr>"}`。manager 用这一个参数调用 `swarm/connect`（`core/commands/swarm.go:641-686`）。它不调用 `config.set`，也不写 `Peering.Peers`。那个勾选框是在改配置，和原始编辑器一起等。成功是 **200** `{"peerId":"<id>"}`，用的是从地址里解析出的 peer id，不是 Kubo 的 `Strings` 那一行。Kubo 自己的消息留在 manager 日志里。地址已经通过下面的规则、Kubo 仍返回非 200，是 **502** `connect_failed`。期限是 30 秒。到点是 **504** `timeout`。

9. **地址是一条已经点名了对方的多地址。** 两端空白忽略。空的拒绝。超过 2048 字节拒绝。网址、单独一个 peer id、空格、反斜杠、`?`、`#` 和 `@` 都拒绝。值必须以 `/` 开头，并以 `/p2p/<peer-id>` 或 `/ipfs/<peer-id>` 结尾。peer id 是 go-libp2p v0.50.0 里 `peer.Decode` 接受的形式之一：`Qm…` 的 sha2-256 peer id（34 字节的多哈希 `0x12` / `0x20`），以 `1` 开头的 base58 多哈希且是 identity（`0x00`）或 sha2-256，或编码为 libp2p-key（`0x72`）的 CIDv1，底为 base32（`b`）、base58btc（`z`）或 base16（`f`）。最后那个 peer id 之前的组件只能是：

   | 组件 | 值 |
   | --- | --- |
   | `ip4` | 点分 IPv4 |
   | `ip6` | IPv6 地址，不带区域 |
   | `tcp`、`udp` | 端口 1 到 65535，没有前导零 |
   | `dns`、`dns4`、`dns6`、`dnsaddr`、`sni` | 主机名 |
   | `certhash` | 一段不含斜杠的记号 |
   | `p2p`、`ipfs` | 一个 peer id，结尾之前也可以出现 |
   | `quic`、`quic-v1`、`webtransport`、`webrtc`、`webrtc-direct`、`tls`、`ws`、`wss`、`p2p-circuit` | 没有值 |

   manager 不解析 `/dnsaddr/…`。还没有以 peer id 结尾的地址会被拒绝，包括单独一个 `/dnsaddr/` 名字。Kubo 会试着解析那个（`core/commands/swarm.go:788-810`）。这一页不解析。规则拒绝的地址是 **400** `invalid_address`，不会发给 Kubo。

10. **错误是短代码。** 正文是 `{"error":"<code>"}`。`node_not_running` 另外带 `state`。

11. **会话和其他页一样。** `GET /api/peers` 需要会话。它不需要 CSRF 头。`POST /api/peers/connect` 需要会话和 CSRF 头。正文是 `application/json`。缺了或是别的类型，是 **415**。空正文、第二个 JSON 值或未知的键，是 **400** `bad_request`。`GET /health` 仍然开放。

## API

| 方法与路径 | 正文 | 成功 |
| --- | --- | --- |
| `GET /api/peers` | 无 | **200** 列表 |
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

`truncated` 为 false 时省略。`direction`、`latencyMs` 和 `agent` 用不上时省略。`direction` 是 `in`、`out` 或 `both`。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `invalid_address` | 地址不符合决定 9 的规则 |
| 400 | `bad_request` | JSON 形状不对 |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 413 | `too_large` | Kubo 的 peers 正文超过 8 MiB |
| 415 | `unsupported_media_type` | POST 不是 JSON |
| 502 | `connect_failed` | Kubo 拒绝了这次拨号 |
| 502 | `kubo_error` | Kubo 因其他原因拒绝了列表 |
| 503 | `node_not_running` | 节点状态不是 `up`，或连接失败 |
| 503 | `unavailable` | 配置不是回环 RPC，或 manager 失败 |
| 504 | `timeout` | 期限到了 |

## 界面

对等节点在导航里排在「探索」后面：节点、文件、探索、对等节点、固定、场景、设置。

页面有标题、一句话，以及一处默认收起的说明。说明写明：对等节点是正在和这个节点说话的另一个 IPFS 节点；粘贴的地址只拨一次；这一页既不显示其他节点的地址，也不保存地址。

节点是 `up` 时，一张卡片里放一个输入框，主按钮只有「连接」。标签是「节点地址」。提示写明地址要以 `/p2p/` 和一个 peer id 结尾。空着不会去叫 manager。页面自己就能拒绝的值也不会，输入框会说明原因。回车提交。拨号进行中，按钮写成「正在连接…」并保持禁用。成功会清空输入框，并给出提示「已连接。」失败留在输入框下面：地址被拒绝、拨号失败，或时间太长。

至少有一个节点时，列表在那张卡片下面。一个都没有、筛选又是空的，句子「还没有人连着。连接会自己出现在这里，你也可以在上面加一个。」留在连接卡片里。第一次加载是骨架，加上句子「正在加载对等节点。」之后的刷新把行留在屏幕上。摘要是「已连接 {count} 个」。筛选在用时，摘要数的是对上的行。筛选对得上 peer id，或忽略大小写的代理版本。以 `/p2p/` 或 `/ipfs/` 开头的查询，在比较 peer id 之前先去掉这一次前缀。Escape 清空筛选。没有对上的，说「没有对上的节点。」`truncated` 为真时，一句「显示 {shown} / {total}。」筛选只看 manager 返回的那些行。

宽布局从 720px 起，是一张表：节点、方向、延迟、软件。peer id 是完整值，会换行。不用在中间截断。一行上的复制按钮写下 peer id。方向是「对方连过来」「我们连过去」或「双向」。延迟是「< 1 毫秒」「{n} 毫秒」，或从 1000 毫秒起带一位小数的「{n} 秒」。缺了延迟或软件，是一个破折号，不是「未知」这个词。

窄布局在 720px 以下，每个节点一张卡片：换行的 peer id 和复制按钮，然后是方向、延迟和软件，每一项都带自己的标签。

页面正在显示列表、节点又是 `up` 时，每 5 秒刷新一次。节点停了、正在启动、正在停止或需要处理时，卡片和列表换成和其他页一样的空状态，以及一颗回到节点的按钮。连不上管理服务时，是那句话加上「再试一次」。

节点页上，数量在显示的时候链到这里。数量本身不变。

文案在 `en` 和 `zh`。令牌来自设计系统。栏宽是 `min(960px, 100%)`，和「文件」「探索」「固定」一样。

仅开发时，`?peers=down`、`?peers=empty` 和 `?peers=list` 不需要 manager 就能画出这些截图。生产包里没有它们。

## 测试

Go：不同的节点收成一行；留下最低的延迟；方向混在一起是 `both`；`n/a` 省略延迟；代理文本被截断并去掉控制字符；列表 JSON 里没有 `Addr`、没有 `/ip4/`、没有公钥，也没有端口 `5001`；500 的上限会置上 `truncated` 并保留 `total`；地址规则接受一条 IPv4 TCP 的 `/p2p/` 地址、一条 QUIC 地址、一条 `/dnsaddr/…/p2p/` 地址，以及用了 `p2p-circuit` 的中继，并拒绝网址、单独的 peer id、缺了 peer id 和错误的端口；拒绝非回环；HTTP 的会话、CSRF、带 `state` 的 503、400，以及只返回 `peerId` 的连接成功。

Web：地址辅助函数、英文下的页面状态（已停止、空、列表、一个被拒绝且没有去叫 manager 的地址），以及对齐的语言键。

## 不做

- 世界地图、国家、旗帜，或从 IP 推出来的任何位置。
- 显示或复制一个已连接节点的多地址、传输字符串、公钥或监听地址。
- 打开的流的名字、按节点的带宽，以及命令行教学。
- 永久对等、`Peering.Peers`、`swarm peering`，以及断开。原始配置编辑器仍要等。
- 解析一条还没有以 peer id 结尾的 `/dnsaddr/` 名字。
- 诊断，以及集成测试基线。
