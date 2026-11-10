# 场景

**中文** · [English](scenes.md)

**状态：已实现。** 这是路线图片 5 的规格。代码和这份文件不一致时，以这份文件为准，直到有 PR 改它。[愿景](../VISION.zh-CN.md)点名的四个场景就是这四个。和 ipfs-webui 的对照在[对照](webui-parity.zh-CN.md)。

## 这是做什么的

新手选一个能用一句话说出来的用法。面板先告诉他会改什么，再写进 Kubo 的配置，并用一句话说明自己做了什么。这个人不去打开 Kubo 的 JSON。

浏览器仍然不跟 Kubo 说话。manager 把配置写到磁盘上。它不调用 `ipfs config`，也不把整份文件、私钥或多地址返回去。

## 四个场景

每个场景写同一组键。当前值已经和场景要的值是一回事时，预览里不列。缺了的键，比较时用下面引用的 Kubo 默认值。`Addresses.Gateway` 在这里没有这种默认：面板自己的 loopback 网关是一个写明的值。

控制地址 `Addresses.API` 永不写入。`Identity` 永不写入。swarm 的监听地址永不写入。

| 场景 | Id | 一句话 |
| --- | --- | --- |
| 在这台机器上安静地跑 | `quiet` | 这个节点留在这台机器上，少为网络做一些事。 |
| 把一个文件夹分享给别人 | `share` | 用 IPFS 的人可以获取「文件」里的文件夹。 |
| 帮一组内容做固定 | `pinset` | 节点把你 pin 住的内容告诉网络，网关不去取别的文件。 |
| 一个只读的公共网关 | `gateway` | 其他电脑可以阅读这台节点上已有的文件。他们不能改任何东西，控制端口仍只在这台机器上。 |

`share` 不挑选文件夹。文件夹就是「文件」里已经有的那些。场景只改节点怎么宣布它们（`Provide.Strategy` 为 `mfs`，`docs/config.md:2331`）。

### 键

`quiet` 跟随 `lowpower` profile（`config/profile.go:373-393`，`docs/config.md:4683-4690`）。其他场景先把这些键放回 Kubo 通常的值，再写上构成这个场景的那一两个键。

| 键 | `quiet` | `share` | `pinset` | `gateway` | 键缺失时的通常值 |
| --- | --- | --- | --- | --- | --- |
| `Routing.Type` | `autoclient` | `auto` | `auto` | `auto` | `auto`（`config/routing.go:15`，`docs/config.md:3028`） |
| `AutoNAT.ServiceMode` | `disabled` | `enabled` | `enabled` | `enabled` | `enabled`（`docs/config.md:567`） |
| `Swarm.RelayService.Enabled` | `false` | `true` | `true` | `true` | `true`（`docs/config.md:3455`） |
| `Swarm.ConnMgr.Type` | `basic` | `basic` | `basic` | `basic` | `basic`（`config/init.go:96`） |
| `Swarm.ConnMgr.LowWater` | `20` | `32` | `32` | `32` | `32`（`config/init.go:85`，`docs/config.md:3611`） |
| `Swarm.ConnMgr.HighWater` | `40` | `96` | `96` | `96` | `96`（`config/init.go:81`，`docs/config.md:3620`） |
| `Swarm.ConnMgr.GracePeriod` | `1m0s` | `20s` | `20s` | `20s` | `20s`（`config/init.go:89`，`docs/config.md:3629`）。lowpower profile 设成一分钟（`config/profile.go:387`）。 |
| `Provide.Enabled` | `true` | `true` | `true` | `true` | `true`（`docs/config.md:2310`） |
| `Provide.Strategy` | `all` | `mfs` | `pinned` | `pinned+mfs` | `all`（`config/provide.go:13`，`docs/config.md:2385`） |
| `Gateway.NoFetch` | `false` | `false` | `true` | `true` | `false`（`docs/config.md:1301`） |
| `Gateway.NoDNSLink` | `false` | `false` | `false` | `true` | `false`（`docs/config.md:1311`） |
| `Gateway.ExposeRoutingAPI` | `true` | `true` | `true` | `false` | `true`（`config/gateway.go:12`） |
| `Addresses.Gateway` | 面板的 loopback | 面板的 loopback | 面板的 loopback | 未指定地址，面板端口 | 不作隐含匹配 |

`Provide.Strategy` 用 Kubo 写明的值：`all`（`docs/config.md:2318`）、`pinned`（`docs/config.md:2319`）、`mfs`（`docs/config.md:2331`）、`pinned+mfs`（`docs/config.md:2333`）。

面板的 loopback 网关是 `/ip4/127.0.0.1/tcp/8081`；manager 已经占了 `8081` 时用 `8082`。这就是[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)选的端口。公共网关恰好是下面两个监听，没有别的：

```text
/ip4/0.0.0.0/tcp/<面板端口>
/ip6/::/tcp/<面板端口>
```

`Addresses.Gateway` 允许这种形状（`docs/config.md:332-354`）。绑未指定地址，别的电脑才连得上。不写某一个公网 IP，启动时也不接受。

`Gateway.Writable` 不能设。Kubo 把它删了（`docs/config.md:1514-1516`）。对这个场景，只读指的是三个还在的旗标：

- `Gateway.NoFetch` 为 true：网关只提供仓库里已经有的内容（`docs/config.md:1296-1299`）。Kubo 自己的安全说明要求网关公开时用它（`docs/config.md:4757`）。
- `Gateway.NoDNSLink` 为 true：`Host` 头不会把 `/` 指到别处（`docs/config.md:1305-1309`）。
- `Gateway.ExposeRoutingAPI` 为 false：旗标开着时，Kubo 把 `/routing/v1` 挂到每一个网关监听上（`cmd/ipfs/kubo/daemon.go:1103-1125`）。默认是 true（`config/gateway.go:12`）。公共场景把它关掉，这样公开端口不提供那个 API。文档写路由 API 在 `127.0.0.1`（`docs/config.md:1387`）；守护进程的代码把它挂在网关监听上，所以这里必须写明 false。

页面用一句话说明：其他电脑可以阅读已经存在这里的文件，不能添加或修改，控制端口仍只在这台机器上。页面可以说出网关端口。它不显示多地址、`0.0.0.0` 或端口 `5001`。

## 重启

上表每个键都是在守护进程构建时读的，不是之后某次请求再读：

| 键 | 读取位置 |
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

关闭仓库不会把配置写回去（`repo/fsrepo/fsrepo.go:547-578`）。下次启动才从文件加载（`repo/fsrepo/fsrepo.go:461-466`）。

所以改动要等一次重启。面板用它已经有的生命周期。重启就是先停再启。它不是一个新状态。

| 人确认时节点的状态 | 面板做什么 |
| --- | --- |
| `up` | 写入，然后重启。响应里 `restart` 为 `started`。概览回到 `stopping`。页面继续轮询。 |
| `down`，或 `failed` 但不是 `repo_busy` | 写入，节点保持停止。`restart` 为 `later`。下次启动用新文件。 |
| `starting`、`stopping`，或 `failed` 且为 `repo_busy` | 不写。**409** `node_busy`。 |

如果写入和重启之间节点离开了 `up`，面板不把它启动。`restart` 为 `later`。

## 预览、应用、备份、撤销

配置文件必须已经存在。第一次成功启动之前没有。**409** `not_ready`。页面说先启动一次节点。

预览不写。至少有一个键在效果上不同，应用才写。没有差异时，回答是 **200**，`restart` 为 `none`，不新增备份。

写入的是整份文件：场景的键被替换，其他键保持原样，包括 `Identity` 和 `Addresses.API`。它用和其他 manager 文件一样的临时文件、fsync、改名，模式 `0600`。改名之前，新的字节用监督器启动时的同一条规则检查。检查失败就不写。

备份是每个受管键的旧值；键原先不存在时记 JSON `null`。里面没有私钥，因为私钥不在这组键里。备份放在 `<data>/scene-backups/`，模式 `0600`，目录 `0700`。保留最新的五份。撤销把最新一份写回去，删掉那份备份，并用同一条重启规则。没有备份是 **409** `nothing_to_revert`。

API 不是 loopback 时，任何写入之前就拒绝：**409** `api_not_loopback`。场景不会被应用。

## API

会话和 CSRF 与[认证](auth.zh-CN.md)、[文件](files.zh-CN.md)相同。GET 要会话。POST 要 `Content-Type: application/json` 和 `X-CSRF-Token`。不认识的 JSON 字段是 **400** `bad_request`。应用不接受空正文。撤销接受 `{}` 或空正文，不接受别的键。

| 方法与路径 | 正文或查询 | 成功 |
| --- | --- | --- |
| `GET /api/scenes` | — | **200** 列表 |
| `GET /api/scenes/preview?id=` | `id` 是四个之一 | **200** 一个场景 |
| `POST /api/scenes/apply` | `{"id"}` | **200** 结果 |
| `POST /api/scenes/revert` | `{}` | **200** 结果 |

列表：

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

`changes` 只列效果上的差异。`code` 是页面要翻译的短记号，不是原始配置值。`Swarm.ConnMgr` 把四个连接管理键合成一行：lowpower 的数字是 `quiet`，否则是 `usual`。网关地址的记号是 `public` 或 `local`。`active` 表示没有效果上的差异。`gatewayPort` 是面板的网关端口，好让页面说得出它。正文里没有多地址，也没有私钥。

结果：

```json
{ "id": "quiet", "restart": "later", "node": { "state": "down", "peerId": null, "version": null, "repoPath": "/path", "error": null } }
```

`restart` 是 `none`、`later` 或 `started`。`node` 是[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)的概览。页面用 `id` 和 `restart` 变成每种语言的一句话。服务器不发送这句话。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `unknown_scene` | `id` 缺失或不是四个之一 |
| 400 | `bad_request` | 处理程序读不了的 JSON |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 409 | `not_ready` | 还没有配置文件 |
| 409 | `node_busy` | 正在启动、正在停止，或 `repo_busy` |
| 409 | `nothing_to_revert` | 没有备份却撤销 |
| 409 | `api_not_loopback` | 控制地址不是 loopback |
| 415 | `unsupported_media_type` | 内容类型不对 |
| 500 | `unavailable` | 文件读不出或写不进 |
| 502 | `restart_failed` | 文件已写入，但重启调用失败 |

## 启动检查

[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)拒绝不是 loopback 的网关。这一片只加一个例外：

- 每个 API 地址仍然必须是 loopback。公开的 API 仍然是 `api_not_loopback`。场景永不写它（`docs/config.md:314-326`）。
- 网关地址可以不是 loopback，仅当每一个这样的地址都是面板网关端口上的未指定监听，并且上面三个只读旗标都已设置。地址形状符合但旗标不符合时，代码是 `gateway_not_readonly`；任何别的公开地址是 `gateway_not_loopback`。
- 网关仍然不能占用 manager 的端口。

网关公开时，文件和固定仍然可用。它们的 loopback 客户端用的是 API 地址。本地分享链接省掉，因为没有 loopback 网关 URL 可复制。页面不编造一个公开 URL。

## 界面

导航里的「场景」可用。它不在「即将推出」下面。

页面有标题和一句话。然后四张卡片，顺序是安静、分享、pin、网关。每张卡片是场景的名字、一句话，以及「预览」。对得上的那个标成「正在使用」。一个都对不上时，一行字说明节点用的是 Kubo 通常的设置。

预览是对话框。它重复场景的那句话，然后每个改动一行白话，配置键用较淡的字写在下面。它说明节点会重启，还是要等下次启动。 「使用这个设置」是主按钮。没有改动时，或节点正忙时，它不可用。对话框不显示多地址。

应用或撤销之后，页面用当前语言给出一句状态。重启进行时，节点卡片自己的那句话继续更新。

有备份时才显示「撤销上一次更改」。确认框的主按钮是「撤销」。焦点先在取消上。

还没准备好、加载中、连不上 manager，各有自己的状态。还没准备好时链到节点页。其他错误是提示。

文案在 `en` 和 `zh`。颜色来自设计系统。

## 测试

Go：每个场景的键、私钥和 API 地址留在原地、预览记号的 JSON 里没有多地址、节点运行时应用会重启、节点停止时应用不会把它启动、撤销恢复先前的值、公开的 API 被拒绝、Unix 上备份模式 `0600`，以及 HTTP 的会话、CSRF 和停止状态下的应用。

Web：四张卡片、预览的句子、应用之后的中文那一句，以及页面上没有端口 `5001`。

## 不做

- 原始配置编辑器。见[对照](webui-parity.zh-CN.md)。探索和对等节点是各自的页面。
- 挑选分享场景宣布哪一个文件夹。它宣布的是 MFS。
- 会去取任意 CID 的公共网关。
- 把网关绑到某一个选定的公网 IP。
- 写 `Addresses.API`、`Addresses.Swarm`、`Identity` 或 `Datastore.Spec`。
- 显示端口 `5001`、多地址或私钥。
