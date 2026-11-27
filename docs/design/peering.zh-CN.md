# 永久对等

[English](peering.md) · **中文**

**状态：已实现。** 这是「保持连接」的规格。代码与本文件不一致时，以本文件为准，直到某个 pull request 修改它。一次性拨号仍在[对等节点](peers.zh-CN.md)。本文件补的是那一页缺的一块。

## 给谁用

新手可以指定一些节点，让本机一直连着。链路断了会再拨。这个选择在重启之后还在。人不需要改 JSON。

入口在现有的对等节点页：已连接的一行上有「保持连接」，另有一份已保存的列表，可以添加地址或移除节点。专家不被挡住。原始配置编辑器仍然可以改 `Peering.Peers`。保存的列表和正在执行的列表不一致时，这一页能看出来，并且可以不重启就把保存的列表应用到正在运行的节点上。

浏览器仍然不跟 Kubo 说话。它跟 manager 说话。manager 在回环上调用 Kubo。这一页拿不到 Kubo RPC 地址。

## Kubo 实际做什么

对照 kubo `833e04a240d8c533509ddaaa3c202b39b0bb3024`。

`Peering.Peers` 是保存下来的列表。每一项是 peer id，以及要拨的地址（`config/peering.go:7`，`docs/config.md:2974`，`docs/config.md:2996`）。地址列表为空时，Kubo 去问 Amino DHT。守护进程只在构建节点时把这份列表抄进 peering 服务（`core/node/groups.go:368`，`core/node/peering.go:35`）。之后再改文件，正在运行的服务不会跟着变。

`ipfs swarm peering` 只改正在运行的服务。这条命令写明改动不会写入配置（`core/commands/swarm.go:93`）。`add` 接收多地址并调用 `AddPeer`（`core/commands/swarm.go:139`）。`ls` 返回服务正在保持的节点（`core/commands/swarm.go:172`）。`rm` 接收 peer id 并调用 `RemovePeer`（`core/commands/swarm.go:218`）。服务用退避再连接。这就是「断了再拨」，而且 `add` 一返回就开始，不等重启。

对数组做 `config/replace` 会替换整个数组（`repo/common/common.go:88`）。送上一份新的 `Peering.Peers` 数组会替换保存的列表。它自己不会调用 `AddPeer`。

所以两次调用都要。实时调用让节点现在就保持连接。写入配置让下一次启动仍然保持。两次都成功时，不需要重启。

## ipfs-webui 做什么

对照 ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`。

「永久」那个框默认是开的，[对等节点](peers.zh-CN.md)里已经记过。处理函数用 `config.set` 写 `Peering.Peers`，不调用 `swarm.peering`（`ipfs-webui/src/bundles/peers.js:42`，`ipfs-webui/src/bundles/peers.js:47`，`ipfs-webui/src/bundles/peers.js:60`）。那段注释在等一个 Kubo pull request；在这个钉上，它没有改变上面「不写入配置」的规则。所以 webui 的永久框是写给下一次启动的。正在运行的服务要等重启才知道。

这一页同时做实时调用，新手不用为了开始重连而去重启。配置仍然要写，因为 webui 写了，而实时调用自己不写。

## 决定

1. **一份列表，两处存储。** 保存的列表是 `Peering.Peers`。正在执行的列表是 `swarm peering ls`。页面展示保存的列表。`sync` 说明正在执行的列表是否与它一致。

2. **添加和移除都做两件事。** 添加会校验地址或 peer id，然后经与原始编辑器相同的替换路径写入 `Peering.Peers`（备份、去掉 `Identity.PrivKey`、锁住 `Addresses.API`、启动前的地址检查），再调用 `swarm/peering/add`。移除从保存的列表删掉这个 peer id，再调用 `swarm/peering/rm`。已连接的那张表仍然不写 `Peering.Peers`。`POST /api/peers/connect` 不变。

3. **两次都成功就不用重启。** `sync` 是 `ok`。页面不要求重启。

4. **不一致时可以应用。** 保存的 id 或地址与正在执行的集合不同时，`sync` 是 `apply`。原始编辑之后，或有人用 CLI 做了没有保存的 `swarm peering`，都会这样。应用会在正在运行的服务上添加和移除，直到它与保存的列表一致。它不改写配置，也不重启守护进程。旁边一句说明重启也会装上保存的列表，因为守护进程在启动时读 `Peering.Peers`。按钮是应用。重启是另一条路，用的是面板里已经有的重启。

5. **实时调用失败时才要重启。** 配置已经写上、`swarm peering` 没有成功时，`sync` 是 `restart`。保存的列表就是下一次启动会装上的那份。页面这么说，给出重启，并把应用留作再试一次。manager 不自己重启。

6. **节点停着是 503。** 缓存状态不是 `up` 时，`GET` 和每个 `POST` 都返回 **503** `{"error":"node_not_running","state":"<state>"}`。读路径是 `config/show` 和 `swarm peering`，两者都要守护进程在。页面用对等节点页已有的那种挡住的空状态。

7. **浏览器可以看见什么。** 已连接的表仍然不收到节点地址。已保存的列表收到这些节点存下来的地址，因为是人选定的，而且要能把行区分开。从已连接的一行保持时，查找发生在 manager 里：`swarm/peers` 有地址，浏览器只送了 peer id，存下来的地址只回到已保存的列表。它们不会加进已连接的表，也不会写进审计行。

8. **地址和 peer id 用对等节点页的规则。** 粘贴的地址走 `peers.ParseAddress`。peer id 是最后一段 `/p2p/` 或 `/ipfs/`。存下来的地址是去掉最后那个 peer id 之后的多地址。单独的 `/p2p/<id>` 按空地址列表保存，这就是 `docs/config.md:2996` 里的 DHT 情况。来自一行的单独 peer id，必须是 `peers.CanonicalPeerID` 接受的。然后 manager 去读这个节点当前的 swarm 地址，只保存能通过同一套地址规则的那些。一条都过不了、但 id 有效：地址列表为空。这个节点不在 swarm 里：**409** `not_connected`。

9. **一个 peer id 一行。** 给已经保存的 id 再加一个地址，是追加。同一个地址再加一次，什么都不改。上限是 64 个节点、每个 8 个地址。这是面板的上限，不是 Kubo 的。超过则添加返回 **409** `too_many`，并且不写。原始编辑器已经做大的列表仍然显示，移除仍然可用。不是对象的项留在文件里，不显示。这一页重写数组时，一项上多出来的字段保留。

10. **一行上的状态。** `connected` 表示 peer id 在 `swarm/peers` 里。`reconnecting` 表示正在运行的 peering 服务有这个 id，而 swarm 没有。`pending` 表示 id 已保存，正在运行的服务还没有它。

11. **备份与场景。** 保存的列表有变化时，把上一份涂掉秘密的文档写到 `<data>/config-backups/`，和原始编辑器恢复用的是同一批文件。私钥不在那个文件里。`Peering.Peers` 不是场景键，所以这里的改动不会把场景标成已离开。场景应用不碰这份列表。

12. **审计。** `peering_add`、`peering_remove`、`peering_apply` 记入 manager 日志，带结果、用户名、客户端 IP，以及有的话带 peer id。这一行没有地址、没有配置文档、没有私钥。

13. **错误是短代码。** 正文是 `{"error":"<code>"}`。`node_not_running` 另外有 `state`。

## API

会话和 CSRF 与[认证](auth.zh-CN.md)、[对等节点](peers.zh-CN.md)相同。GET 要会话。POST 要 `Content-Type: application/json` 和 `X-CSRF-Token`。未知 JSON 字段是 **400** `bad_request`。第二个 JSON 值是 **400** `bad_request`。

| 方法与路径 | 正文 | 成功 |
| --- | --- | --- |
| `GET /api/peers/permanent` | 无 | **200** 列表 |
| `POST /api/peers/permanent` | `{"address":"/ip4/203.0.113.10/tcp/4001/p2p/12D3Koo…"}` 或 `{"peerId":"12D3Koo…"}` | **200** 列表 |
| `POST /api/peers/permanent/remove` | `{"peerId":"12D3Koo…"}` | **200** 列表 |
| `POST /api/peers/permanent/apply` | `{}` 或空正文 | **200** 列表 |

添加时 `address` 和 `peerId` 恰好一个。移除一个未知 id 是 **200**，如果正在运行的服务里有它，仍然会去掉。

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

没有节点时 `peers` 是 `[]`，不是 `null`。要问 DHT 时 `addrs` 是 `[]`，不是 `null`。`sync` 是 `ok`、`apply` 或 `restart`。`state` 是 `connected`、`reconnecting` 或 `pending`。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `invalid_address` | 地址不符合对等节点页的规则 |
| 400 | `invalid_peer` | peer id 不是这一页接受的 |
| 400 | `bad_request` | JSON 形状不对 |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 409 | `not_connected` | 从一行保持，而这个 id 不在 swarm 里 |
| 409 | `too_many` | 会超过上限 |
| 409 | `api_locked`、`identity_locked`，以及编辑器的其他锁 | 替换路径拒绝了；这一页不会故意去触发 |
| 413 | `too_large` | Kubo 的正文超过上限 |
| 415 | `unsupported_media_type` | POST 不是 JSON |
| 502 | `kubo_error` | Kubo 拒绝了一次读取 |
| 503 | `node_not_running` | 节点状态不是 `up` |
| 503 | `unavailable` | 配置不是回环 RPC，或 manager 失败 |
| 504 | `timeout` | 到了期限 |

保存成功之后实时的 `swarm peering` 失败，是 **200** 且 `sync` 为 `restart`，不是 502。保存已经发生。页面得说出下一步做什么。

## 界面

页面仍然是对等节点。标题仍是对等节点。标题下那一句把「保持连接」算进这一页的工作。说明里写：连接一次是拨号且不保存；保持连接会存下这个节点，断了会再拨；已连接的表不显示地址；已保存的列表显示存下来的地址。

节点为 `up` 时，一张卡片里放工具条和两份列表。工具条是地址框、作为主按钮的「保持连接」、作为次按钮的「连接一次」，以及有人连着时的查找。回车是保持。空字段不调用 manager。页面自己就能拒绝的值也不调用。

已保存的列表在工具条下面。标题是「保持连接」。零行时写「还没有保持任何节点。」一行是 peer id、保存的地址、状态词，以及移除。状态词是「已连接」「正在重连」「等待中」。已连接的表除了每一行的「保持」之外不变。这个 id 已经保存时，这个动作显示「已保持」，并且不做任何事。

`sync` 为 `apply` 时显示一行：「保存的节点还不是这个节点正在保持的。」按钮是应用。第二句说重启节点也会装上保存的列表，用的是已有的重启。

`sync` 为 `restart` 时显示：「这些节点会在节点重启之后保持。」按钮是重启。应用留在那里，当作再试一次。

保持进行中时，主按钮写「正在保持…」，两个地址按钮都禁用。移除和应用进行中时写明自己在做什么，并且禁用。成功的提示是「已保持。」「已移除。」「已应用。」重启用面板里已有的节点重启，然后重新加载这一页。

节点停止、正在启动、正在停止或失败时，卡片换成这一页已经在用的空状态。已保存的列表不靠记忆显示。

文案在 `en` 和 `zh`。记号来自设计系统。栏宽仍是对等节点页的栏宽。

仅开发时，`?peers=kept`、`?peers=apply`、`?peers=restart`、`?peers=adding` 不连 manager 就能画出这些画面。`?peers=down`、`?peers=empty`、`?peers=list` 仍画出原来的画面。生产包里没有它们。

## 以后加一条路由

manager 的一块功能是一个带 `routes()` 方法的处理类型。`newMux` 挂上这些方法。认证和节点生命周期留在 `newMux` 里，因为它们闭合了进程本身。新的一块加一个类型、一个 `routes()` 方法，以及 `newMux` 里的一行。不要再把位置参数列表加长。

浏览器通过 `apps/web/src/lib/api.ts` 里的 `apiJSON` 调用 manager。一块功能在自己的页面旁边加一个有类型的函数，像 `apps/web/src/peering/api.ts` 那样，再加一个用它的 store。页面自己不调用 `fetch`。

这些路由是给这一台本机节点的。第二台节点应当在同样的处理函数前面加一个节点 id，而不是再写一套 peering。Kubo 的安装、远控、IPNS、远端固定和诊断，都可以各自是一个包、一个 `routes()` 方法和一个有类型的客户端。这一刀不去做那些。

## 测试

Go：保存的列表能加上一个地址、去掉重复、删掉一个 peer id；超过上限时拒绝再加；实时与保存不一致是 `apply`；保存之后实时调用失败是 `restart`；替换正文里没有私钥；写了备份且备份里没有私钥；`Addresses.API` 不变；拒绝非回环；HTTP 的会话、CSRF、带 `state` 的 503、400，以及一次保持返回 peer id 和存下的地址。

Web：列表解析，以及英文下的页面状态（停止、空、已保持、需要应用、需要重启、正在添加、页面自己就能拒绝的地址且不调用 manager）。

发行版二进制的测试基线覆盖：添加、不重启就能在正在运行的列表里看见、守护进程重启之后仍是同一个节点、移除，以及节点停止。版本钉和检查项在[测试基线](harness.zh-CN.md)。

## 不在范围内

- 地图、断开一条并非已保持节点的 swarm 连接，以及按节点的带宽。
- 显示人没有要求保持的那些 swarm 地址。
- 不移除节点就改掉它的某一个地址。移除会去掉整个节点。单个地址由原始编辑器改。
- 解析一条还不以 peer id 结尾的 `/dnsaddr/` 名字。
- 安装 Kubo、远控、桌面壳、IPNS、远端固定、诊断，以及第二台节点。
