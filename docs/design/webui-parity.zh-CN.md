# 与 ipfs-webui 的对照

**中文** · [English](webui-parity.md)

**状态：切片 5 的清单。** 这不是一份“把 webui 抄过来”的规格。它记下 [ipfs-webui](https://github.com/ipfs/ipfs-webui) v4.13.0 实际有什么，以及每一项在这个面板里落在哪。commit 是 `c51d980920d60e9d41f53f461886d6e1ea046298`，钉在[技术栈参考](../REFERENCE-ipfs-stack.zh-CN.md)。

路由表在 `ipfs-webui/src/bundles/routes.js:13-28`。导航在 `ipfs-webui/src/navigation/NavBar.js:81-86`：状态、文件、探索、对等节点、设置、诊断。固定和欢迎页是路由，不在这组导航里。

面板保持自己的新手语气（[愿景](../VISION.zh-CN.md)）和自己的设计系统。下面没有任何一条是要求把这个产品改成 webui 的样子。

## 每一项落在哪

| Webui 表面 | 它做什么 | 这个面板 |
| --- | --- | --- |
| 状态 `/` 与 `/status*` | 已连接的一句话、peer id、版本、仓库大小、对等节点数、带宽图和速率 | **已有**，这一片再加一小块。节点页已经有那句话、版本和 peer id（[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)）。这一片加上对等节点数和入/出速率，节点停着的时候两者都藏起来。仓库大小、图表、分析横幅仍延后。 |
| 文件 `/files*`、`/ipfs*`、`/ipns*` | 可变文件系统：上传、文件夹、下载、重命名、删除、分享链接、pin、查看、CAR 导入、按路径添加 | **已有**新手这条路（[文件](files.zh-CN.md)）：添加、下载、重命名、移动、复制、删除、本地链接，以及从一行发起 pin。查看、CAR 导入、按路径添加、浏览 `/ipfs` 或 `/ipns` 路径仍延后。 |
| 探索 `/explore*` | 走网关的 IPLD 浏览器 | **延后。** 不在这个 PR。记在[交接](../HANDOFF.zh-CN.md)里，作为下一块 webui 缺口。 |
| 对等节点 `/peers` | 世界地图、节点表、手动连接 | **延后。** 地图和连接框不在这个 PR。节点页只显示数量。 |
| 固定 `/pins*` | 进行中的 pin 任务。不在导航里 | **已有**，而且比这条路由宽。[固定](pins.zh-CN.md)列出节点留着什么、添加 pin、取消任务。 |
| 设置 `/settings*` | Kubo RPC 地址、本地网关 URL、分享链接形状、IPNS 密钥、远程 pin 服务、检索检查 URL、语言、分析、命令行教学、原始 JSON 编辑器 | **部分已有。** 语言和主题在设置里。这一片的场景，让愿景里的四种用法不必去改 JSON。原始配置编辑器、IPNS 密钥、远程 pin 服务、分析开关仍延后。面板不加 Kubo RPC 地址的表单。 |
| 设置里的分析 `/settings/analytics` | 分析开关 | **延后。** |
| 诊断 `/diagnostics*` | 日志、检索检查、DHT provide | **延后。** 专家模式，切片 6。 |
| 欢迎 `/welcome` | IPFS 和 webui 是什么 | **已有**，更短。节点页说明节点是什么。没有第二套产品导览。 |
| 空白 `/blank` | 外壳用的空路由 | **不适用。** |

## 这一片从状态页拿走的

状态页的对等节点数来自 `swarm peers`（`ipfs-webui/src/status/StatusConnected.js:6`，计数在 `ipfs-webui/src/bundles/peers.js:27-32`），带宽来自 `stats bw`（`ipfs-webui/src/bundles/node-bandwidth.js:10`）。两者都是真正的 Kubo RPC。图表、按节点的带宽、国家图不抄。

manager 调同样两条 RPC，只留下一个数量和两个速率。响应形状见[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)。`swarm peers` 里的地址在浏览器看到正文之前丢掉（`core/commands/swarm.go:433` 是那个不转发的地址字段）。

## 有意留在外面的

- 探索、对等节点地图、原始配置编辑器。它们是下一块 webui 缺口，不是这一片。
- 任何把 Kubo RPC 地址交给浏览器的屏幕。webui 的设置页有一张（`ipfs-webui/src/settings/SettingsPage.js:60`）。这个面板没有。
