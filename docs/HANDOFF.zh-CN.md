# 交接

**中文** · [English](HANDOFF.md)

先读这份，再读 `AGENTS.zh-CN.md`。这份文件是工作状态：交付了什么、定了什么、还开着什么、下一步做什么。它不是教程也不是计划，而是"现在什么是真的"的答案。

这份文件不记 commit 哈希。更新它的那个 PR 一合并，哈希就过期了，写在这里等于构造上就是错的。直接读分支：

```bash
git log --oneline -1 origin/main origin/develop && git tag -l
```

## 仓库现状

| | |
| --- | --- |
| `main` | 发布分支。上面每个提交都是一次发布；最新是 v0.1.1 |
| `develop` | 集成分支。功能先落这里 |
| 最新 tag | v0.1.1 |
| 打开的 PR | 无 |
| 路线图 | 切片 1–4 已在 v0.1.0 发布；切片 5（场景）在 `develop`；探索、对等节点，以及发行版 Kubo 测试基线在 `develop`；切片 6（专家模式、多节点）更晚 |

v0.1.0 已交付：英文与中文的面板外壳、首次运行建管理员与会话登录、节点启停与一句话状态、可变文件系统上的文件页、固定（pins）页。linux、macOS、Windows 三平台二进制加 web 包，作为 GitHub Release 发布并附校验和。

## 已定的事

以下已定。没有理由、没有说明原因的 PR，不要重开。

- **只有 manager 调用 Kubo。** 浏览器永远拿不到 Kubo RPC 地址。见 `docs/VISION.zh-CN.md` 与 `docs/design/auth.zh-CN.md`。
- **Kubo 集成暂时留在进程边界。** `docs/design/kubo-integration.zh-CN.md` 把它与 Kubo 插件做过比较，并推荐边界。插件这条路是真的，但代价是 Windows 上要分叉 Kubo 构建，以及一个 Kubo 明确不保证稳定的 fx 图。
- **测试基线是发行的 Kubo 二进制，不是库导入。** `docs/design/harness.zh-CN.md`。进程内方案被考虑过并被否；文件里记了原因。
- **依赖是钉定的，参考克隆也钉在确切 commit。** 表格在 `docs/REFERENCE-ipfs-stack.zh-CN.md`；`.github/scripts/fetch-reference.sh` 可复现。对照清单用的 ipfs-webui v4.13.0 也钉在那里。
- **面板不需要节点的 bitswap。** 网关读取已经用到它，面板要做的事没有一件需要账本或进程内代码。见 `docs/design/kubo-integration.zh-CN.md`。不要做插件。
- **场景是四种有名字的用法。** `quiet`、`share`、`pinset`、`gateway`。键、重启规则，以及只读公共网关的例外，在 `docs/design/scenes.zh-CN.md`。控制 API 保持 loopback。
- **探索看的是一个内容标识的里面。** manager 在回环上调用 `dag/resolve` 和 `dag/get`。浏览器不去请求网关，也拿不到 RPC 地址。规格是 `docs/design/explore.zh-CN.md`。
- **对等节点是一张表和一次拨号。** manager 在回环上调用 `swarm/peers` 和 `swarm/connect`。已经连上的节点地址不进浏览器。没有地图，这一页也不写 `Peering.Peers`。规格是 `docs/design/peers.zh-CN.md`。

## 还开着的事

每一条都需要**决定**，不是代码。

1. **Dependabot 是否真的改道。** v0.1.1 把 `target-branch: develop` 放到了 `main` 上——那才是 GitHub 读取的那一份。Dependabot 自己的运行现在显示 `head_branch: develop`，说明生效了。剩下的检查是它下一次开的 PR：如果那个仍指向 `main`，说明设置被忽略，替代方案是把 `develop` 设为默认分支（`docs/ORG-SETUP.zh-CN.md` 第 3 步）。
2. **和 webui 的差距还在。** 剩下的缺口是原始配置编辑器。清单在 `docs/design/webui-parity.zh-CN.md`。原始编辑器是专家模式，仍然要等。不要在做别的事的同一个改动里开始它。

## 值得点名的缺口

- **指向 `develop` 的 pull request 会用 Kubo v0.43.1 跑 RPC 面。** 版本钉和校验和在 `.github/scripts/fetch-kubo.sh`。在这个发行版上，`object/stat` 是已删除的命令，所以固定的大小保持未设置。`docs/design/harness.zh-CN.md` 里的其他检查能对上这个二进制。本地没有 `IPFS_PANEL_INTEGRATION=1` 时，测试仍然跳过。
- **`docs/design/*.md` 是权威。** `harness.md` 已实现。`kubo-integration.md` 是一份决定，不是待做的构建。读它们的 status 行。
- **Windows 覆盖依赖有人在那边跑测试。** 测试基线只在 ubuntu 上跑。切片 1–4 里的三个 Windows 缺陷，全是有人在 Windows 上跑 `go test` 才发现的。
- **`ci-ok` 要求九个任务。** `web` 和 `manager` 在路径未变时跳过。`integration` 在指向 `develop` 的 pull request 上跑，其他时候跳过。这是有意的，`docs/ORG-SETUP.zh-CN.md` 说明了规则集。加必需检查时，要同时改 `.github/workflows/ci.yml` 与 `docs/ORG-SETUP.zh-CN.md`。

## 具体的下一步

如果你只有一个会话且没有别的指示：

1. 读 `AGENTS.zh-CN.md`，再读 `docs/design/harness.zh-CN.md`，不要把测试基线、对等节点、探索或四个场景重开一遍。
2. 原始配置编辑器是剩下的 webui 缺口，仍然要等。除非任务就是它，否则不要开始。不要在做别的事的同一个改动里开始它。

场景已经在。探索已经在。对等节点已经在。发行版 Kubo 的测试基线已经在。bitswap 的问题已经回答。不要开始插件工作。

## 这份文件怎么保持真实

在改变它所述内容的**同一个 PR**里更新它。过期的交接比没有更糟，因为下一个读者会信它。

它不由 CI 强制，由 `AGENTS.zh-CN.md` 第 6 步和评审强制。发现它在漂移，先修它，再做别的。