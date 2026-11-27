# 原始配置编辑器

**中文** · [English](config-editor.md)

**状态：已实现。** 这是专家用的原始配置编辑器的规格。如果代码和这份文件不一致，以这份文件为准，直到一个 pull request 改它。它是[与 ipfs-webui 的对照](webui-parity.zh-CN.md)里剩下的那一项。它是专家模式。它不是新的编号切片，也不在新手路径上。

## 这是做什么的

专家可以读 Kubo 的配置，用 JSON 改它，先看差异，再保存。面板留下上一份文档的备份，并能一步放回去。它说明需要重启，并提供重启。它不自己重启。

编辑器在设置里，高级这一节，标成专家。它不是导航项。节点、文件、探索、对等节点、固定、场景仍是新手路径。[愿景](../VISION.zh-CN.md)已经说过，原始配置不是首页。

浏览器仍然不跟 Kubo 说话。它跟 manager 说话。manager 在回环上调用 Kubo。这一页不加 Kubo RPC 地址的表单，也不拨它正在显示的任何地址。

## ipfs-webui 实际有什么

对照 ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`。

设置是 `/settings*`（`ipfs-webui/src/bundles/routes.js:22`）。配置块是带行号的 Ace 编辑器（`ipfs-webui/src/settings/editor/JsonEditor.js:15-28`）。页面用 `config.getAll` 取整份配置（`ipfs-webui/src/bundles/config.js:12`），`JSON.parse` 之后用 `config.replace` 保存（`ipfs-webui/src/bundles/config-save.js:64-72`）。重置把上次载入的文本抄回编辑器。没有备份，没有差异，也没有场景警告。守护进程停着的时候，编辑器换成一句话（`ipfs-webui/public/locales/en/settings.json:120-121`）。保存之后，页面说新设置在下次启动守护进程时生效（`ipfs-webui/public/locales/en/settings.json:126-127`）。

webui 的浏览器自己拿着 Kubo RPC 客户端。这个面板不抄那个。它抄的是这件事：编辑 JSON，经 `config/replace` 保存，稍后再重启。

## 读和写

manager 用 Kubo 的 `config/show` 读，用 `config/replace` 写。两者都是回环 API 上的 `POST /api/v0/...`。`config/show` 读磁盘上的文件（`core/commands/config.go:239-247`）。`config/replace` 把正文解进 Kubo 的配置结构，再调用 `SetConfig`（`core/commands/config.go:665-724`，`repo/fsrepo/fsrepo.go:654-677`）。

`SetConfig` 把解出来的结构深合并到文件里已有的 map 上（`repo/common/common.go:65-92`）。Kubo 不认识的键会被忽略。结构在空值时省略的键，可能留着原来的值。所以编辑器不保证删掉一行就删掉那个键。页面把这句话写出来。要改一个值，就把它设成想要的值。

replace 的请求正文是一段 multipart，`Content-Disposition` 带 `filename`。这是 Kubo 的 HTTP 文件参数所读的形状（`files/multipartfile.go:161-173`）。这一段的字节就是 JSON 文档。manager 不发送 `expand-auto`。

## 秘密

Kubo 已经从 `config/show` 里抹掉三样东西（`core/commands/config.go:273-286`）：

- `Identity.PrivKey`（`docs/config.md:1724-1727`）
- `API.Authorizations`（`config/api.go:10`）
- `Pinning.RemoteServices.*.API.Key`（`config/remotepin.go:5`）

manager 在正文能到浏览器之前，再按不区分大小写抹掉同样三条路径。私钥是省掉，不是换成一个浏览器可以原样送回来的占位符。

`config/replace` 拒绝非空的 `Identity.PrivKey`（`core/commands/config.go:673-675`），然后写回仓库里已有的那把钥匙（`core/commands/config.go:677-692`）。它也拒绝增删远程 pin 服务，或改它们的 API 细节（`core/commands/config.go:703-720`）。manager 从不把私钥、授权条目或 pin 的 API 密钥放进 replace 正文。保留是 Kubo 做的，留的是它已经存着的那把钥匙。

如果提交的文档里有非空的 `Identity.PrivKey`，manager 以 **400** `privkey_locked` 拒绝，并且不调用 Kubo。如果里面有 `API.Authorizations`，或非空的远程 pin `API.Key`，manager 以 **400** `secret_locked` 拒绝，并且不调用 Kubo。空的或缺失的私钥会被剥掉，Kubo 保留真的那把。

审计行和日志里没有文档、没有私钥、没有 pin 密钥、也没有授权值。

## manager 拒绝改的东西

`Addresses.API` 是 manager 找到 Kubo 的方式。一次保存如果 API 地址列表和当前列表不完全一样，就是 **409** `api_locked`。列表是 Kubo 存的那个字符串，或那个字符串数组。重排、多加一个、改一个端口，都是改动。同一些地址换成另一种 JSON 形状（一个字符串，或只含那个字符串的数组）仍是同一个列表。

`Identity.PeerID` 必须保持 `config/show` 返回的那个 peer id。Kubo 会用存着的钥匙覆盖一个不同的值（`core/commands/config.go:688-692`，`docs/config.md:1711-1714`）。不同或缺失的 peer id 是 **409** `identity_locked`。

replace 之前，manager 还跑监督器启动前的同一条检查（[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)，`AssessConfig`）。面板拒绝启动的文档就不写。代码是那条检查已经返回的：`api_not_loopback`、`gateway_not_loopback`、`gateway_not_readonly`、`panel_port_conflict`。每一个都是 **409**。

这些检查是这个面板自己做的结构健全性。剩下的是 Kubo 自己的解码。通过了这些检查、Kubo 仍然拒绝的文档，是 **502** `kubo_rejected`，`detail` 是 Kubo 的消息，截到 240 个字符，控制字符去掉。之前的文件不动。

## 备份和恢复

一次成功的、并且确实改了文档的 replace，把上一份已涂掉秘密的文本写到 `<data>/config-backups/`，模式 `0600`，目录模式 `0700`。文件里没有私钥，因为它抄的文本已经涂过。保留最新的五份。Kubo 拒绝的那次 replace 会删掉刚写的备份。没有变化的 replace 不写备份。

恢复读取最新的备份，并走同一条 replace 路径，包括上面的锁。它不再写一份新备份。成功之后删掉它用掉的那份。没有备份是 **409** `nothing_to_restore`。

场景备份仍在 `<data>/scene-backups/`。这个编辑器不读也不写它们。

## 重启

Kubo 接受 replace 时，文件就变了。正在跑的守护进程已经读过这些键里的大多数。页面不自己重启。响应里的 `restart` 是 `needed`。页面提供面板已经有的那次重启。关掉对话框就让节点继续跑；新文件在下次启动时用。`restart` 为 `none` 表示文档和已经存着的一致。

## 场景

[场景](scenes.zh-CN.md)仍然靠改配置文件来应用和撤销，节点在跑的时候再重启。这个编辑器不走那条路，也不碰场景备份。

`GET` 的 `scene` 是键已经对上的那个场景的 id，否则是 `null`。对上与否就是 `scenes.Catalog` 已经算的那个。`sceneKeys` 是那份规格里的受管键列表，所以专家打字的时候页面可以警告。

一次写入时，如果原来有一个场景正在用，而提交的文档改了其中任何一个键，`diverged` 为真。差异对话框说明节点会离开那个用法，场景页仍然可以应用或撤销。改其他键不会让 `diverged` 为真。一次偏离的保存之后，下一次场景列表不再把那个场景标成正在使用。场景页的撤销仍从场景备份里恢复那些键。

## API

会话和 CSRF 与[认证](auth.zh-CN.md)一致。GET 需要会话。POST 需要 `Content-Type: application/json` 和 `X-CSRF-Token`。未知的 JSON 字段是 **400** `bad_request`。replace 的正文是 `{"text":"<json>"}`。恢复接受 `{}` 或空正文，不接受别的键。`text` 最大 2 MiB，和 manager 读 Kubo 配置文件的上限一样。更大的正文是 **413** `too_large`。

节点必须是 `up`。其他状态是 **503** `node_not_running`，并带 `state`。编辑器不能改去直接写文件：读的路径是 `config/show`，节点停着的时候这条 RPC 也停着。

| 方法与路径 | 正文 | 成功 |
| --- | --- | --- |
| `GET /api/config` | — | **200** 文档 |
| `POST /api/config` | `{"text"}` | **200** 结果 |
| `POST /api/config/restore` | `{}` | **200** 结果 |

文档：

```json
{
  "text": "{\n  \"Addresses\": {\n    \"API\": \"/ip4/127.0.0.1/tcp/5001\"\n  }\n}\n",
  "scene": "quiet",
  "sceneKeys": ["Routing.Type", "Addresses.Gateway"],
  "backup": false
}
```

`text` 是涂掉秘密之后的文档，两格缩进，对象键排序，末尾一个换行。`scene` 是字符串或 `null`。`sceneKeys` 始终是受管列表。`backup` 表示有没有可恢复的点。

结果：

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

`restart` 是 `none` 或 `needed`。`diverged` 是布尔值。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `bad_request` | 处理函数读不了的 JSON，或未知字段 |
| 400 | `invalid_json` | `text` 不是 JSON，或后面还有多余内容 |
| 400 | `not_object` | JSON 值不是对象 |
| 400 | `privkey_locked` | 非空的 `Identity.PrivKey` |
| 400 | `secret_locked` | `API.Authorizations`，或非空的 pin `API.Key` |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 409 | `api_locked` | `Addresses.API` 缺失或不同 |
| 409 | `identity_locked` | `Identity.PeerID` 缺失或不同 |
| 409 | `api_not_loopback` | 启动检查拒绝了 API |
| 409 | `gateway_not_loopback` | 启动检查拒绝了网关 |
| 409 | `gateway_not_readonly` | 公共网关不是只读那种形状 |
| 409 | `panel_port_conflict` | API 或网关用了 manager 的端口 |
| 409 | `nothing_to_restore` | 没有备份却要恢复 |
| 413 | `too_large` | `text` 超过 2 MiB |
| 415 | `unsupported_media_type` | 内容类型不对 |
| 502 | `kubo_rejected` | Kubo 拒绝了 replace。`detail` 是 Kubo 的消息 |
| 503 | `node_not_running` | 节点不在运行。带 `state` |
| 503 | `unavailable` | manager 读不了或存不了文档 |
| 504 | `timeout` | Kubo 没有及时回答 |

审计行是结构化的 slog，`msg=audit`，带 `event`（`config_replace` 或 `config_restore`）、`result`、`username` 和 `ip`。结果包括 `success`、`unchanged`、`invalid`、`forbidden` 和 `rejected`。这一行没有文档，也没有秘密。读不是审计事件。

## 界面

设置增加高级这一页，排在关于之后。里面的面板带一个专家标签。标签下面的一句话说，这是原始配置，一个错的值可以让节点启动不了，私钥不在这份文档里。

编辑器是带行号的等宽代码面。依赖是 CodeMirror 6，按小包引入（`@codemirror/state`、`@codemirror/view`、`@codemirror/commands`、`@codemirror/language`、`@codemirror/lang-json`、`@codemirror/lint`），不是 webui 那套 Ace，也不是第二套界面库。理由就是编辑器本身：行号、坏行上的槽标记、括号配对，而不带搜索面板，也不带会把页面涂掉的主题。这一次的编辑器是单独一块，压缩前 360 kB，gzip 后 117 kB，打开高级时才加载。面板的其他页面不下载它。这个面用面板的色板。萤光绿不是编辑器的底，也不是选区。

工具条是保存、还原、恢复备份。保存是唯一的主按钮。JSON 无效，或文本和载入的文档一样时，它是禁用的。还原把载入的文档放回去。只有 `backup` 为真时才显示恢复备份，并且在丢掉未保存的编辑之前先问。Ctrl+S 或 Cmd+S 在可以保存时打开差异。状态行说明 JSON 是否有效；无效时写出最接近错误的那一行。

保存先打开载入文档和编辑器的差异，再确认。编辑改了某个场景的键时，对话框点出那个场景。确认才执行保存。`restart` 为 `needed` 之后，第二个对话框提供重启和关闭。重启调用已有的节点重启。`restart` 为 `none` 是一句短确认，没有重启对话框。

带着未保存的编辑离开页面时先问。刷新通过浏览器的离开提示做同样的事。

每种状态有自己的布局：

| 状态 | 页面做什么 |
| --- | --- |
| 加载 | 编辑器形状的骨架。不是一份空文档。 |
| 就绪 | 文档，保存禁用，私钥那句话，场景对上时的那句说明。 |
| 已改 | 保存可用。状态行说还有未保存的编辑。 |
| JSON 无效 | 保存禁用。状态行写出那一行。槽上做标记。 |
| 差异 | 确认对话框，里面是改过的行。 |
| 已保存 | 重启对话框。 |
| 节点已停 | 空状态，链到节点页。没有编辑器。 |
| 错误 | 那个代码对应的句子，以及重试。有 `detail` 时放在句子下面。 |

文案在 `en` 和 `zh`。色板来自设计系统。浅色和深色、1440 和 390，都是真实布局：工具条会换行，编辑器在自己里面滚动，对话框里的差异也会滚动。

## 测试

Go：涂掉私钥、授权列表和 pin 密钥；API 地址变了、peer id 变了、提交了私钥时拒绝并且不调用 replace；改一个普通值会调用 replace，并在 Unix 上写下模式 `0600` 的备份；Kubo 拒绝时不留备份；恢复把上一份文本放回去并删掉那份备份；编辑避开 `sceneKeys` 时原来对上的场景仍然对上，没避开时 `diverged` 为真；原始编辑之后，场景的应用和撤销仍然能跑；HTTP 会话、CSRF，以及停着的节点。

发行版二进制的测试基线用同一个客户端打 Kubo v0.43.1。`config/show` 里没有私钥，磁盘上的文件里仍有。翻转 `Discovery.MDNS.Enabled` 能读回来，API 地址不变，提交的私钥被拒绝，停着的节点返回 `node_not_running`。裁定过的句子在[测试基线](harness.zh-CN.md)。

Web：高级这一页、专家标签、无效 JSON 禁用保存、保存前的差异、中文的重启句子，以及渲染出来的文档里没有私钥。

## 不在范围内

- IPNS 密钥、做成表单的远程 pin 服务、分析，以及诊断页。这些仍在[与 ipfs-webui 的对照](webui-parity.zh-CN.md)里延后。
- 一张表单，它的工作是设置浏览器要去调用的 Kubo RPC 地址。
- 改 `Identity.PrivKey`、`API.Authorizations` 或远程 pin 的 API 密钥。
- 保持连接的控件。那是[永久对等](peering.zh-CN.md)。这个编辑器仍然可以改 `Peering.Peers`。对等节点页会发现保存的列表和正在执行的列表不一致。
- 节点停着的时候编辑配置。
- 不问就重启。
