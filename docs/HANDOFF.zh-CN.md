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
| 路线图 | 切片 1–4 已在 v0.1.0 发布；切片 5（场景）在这次工作之后位于 `develop`；切片 6（专家模式、多节点）更晚 |

v0.1.0 已交付：英文与中文的面板外壳、首次运行建管理员与会话登录、节点启停与一句话状态、可变文件系统上的文件页、固定（pins）页。linux、macOS、Windows 三平台二进制加 web 包，作为 GitHub Release 发布并附校验和。

## 已定的事

以下已定。没有理由、没有说明原因的 PR，不要重开。

- **只有 manager 调用 Kubo。** 浏览器永远拿不到 Kubo RPC 地址。见 `docs/VISION.zh-CN.md` 与 `docs/design/auth.zh-CN.md`。
- **Kubo 集成暂时留在进程边界。** `docs/design/kubo-integration.zh-CN.md` 把它与 Kubo 插件做过比较，并推荐边界。插件这条路是真的，但代价是 Windows 上要分叉 Kubo 构建，以及一个 Kubo 明确不保证稳定的 fx 图。
- **测试基线是发行的 Kubo 二进制，不是库导入。** `docs/design/harness.zh-CN.md`。进程内方案被考虑过并被否；文件里记了原因。
- **依赖是钉定的，参考克隆也钉在确切 commit。** 表格在 `docs/REFERENCE-ipfs-stack.zh-CN.md`；`.github/scripts/fetch-reference.sh` 可复现。对照清单用的 ipfs-webui v4.13.0 也钉在那里。
- **面板不需要节点的 bitswap。** 网关读取已经用到它，面板要做的事没有一件需要账本或进程内代码。见 `docs/design/kubo-integration.zh-CN.md`。不要做插件。
- **场景是四种有名字的用法。** `quiet`、`share`、`pinset`、`gateway`。键、重启规则，以及只读公共网关的例外，在 `docs/design/scenes.zh-CN.md`。控制 API 保持 loopback。

## 还开着的事

每一条都需要**决定**，不是代码。

1. **集成测试基线已设计但未实现。** `docs/design/harness.zh-CN.md` 规定了一个 CI 任务：下载发行版 Kubo 二进制，用它跑现有的 `TestRealKuboOptional`。今天那个测试除非开发者设 `IPFS_PANEL_INTEGRATION=1` 且 `PATH` 上有 `ipfs`，否则跳过，**所以 CI 从未碰过 RPC 面**。
2. **Dependabot 是否真的改道。** v0.1.1 把 `target-branch: develop` 放到了 `main` 上——那才是 GitHub 读取的那一份。Dependabot 自己的运行现在显示 `head_branch: develop`，说明生效了。剩下的检查是它下一次开的 PR：如果那个仍指向 `main`，说明设置被忽略，替代方案是把 `develop` 设为默认分支（`docs/ORG-SETUP.zh-CN.md` 第 3 步）。
3. **和 webui 的差距还在，顺序是这些。** 探索、对等节点页（地图或表格，不只是数量）、原始配置编辑器。清单在 `docs/design/webui-parity.zh-CN.md`。不要在改场景的同一个改动里开始这些。原始编辑器是专家模式，等切片 1–5 变得没劲。

## 值得点名的缺口

- **CI 里 RPC 面未被测试。** `docs/design/harness.zh-CN.md` 点名了三个猜测：`pin/ls` 的拒绝报文形状、不带 `Content-Length` 的 multipart、`object/stat` 的响应形状。今天任何一个都可能是错的，而没有测试会说。
- **`docs/design/*.md` 是权威，其中两份描述的东西还不存在。** `harness.md` 只有设计，`kubo-integration.md` 只有设计。读它们的 status 行。
- **Windows 覆盖依赖有人在那边跑测试。** CI 只有 ubuntu。切片 1–4 里的三个 Windows 缺陷，全是有人在 Windows 上跑 `go test` 才发现的。
- **`ci-ok` 要求八个任务，而 `web`/`manager` 在路径未变时跳过。** 这是有意的，`branch-policy` 里说明了规则集。加必需检查时，要同时改 `.github/workflows/ci.yml` 与 `docs/ORG-SETUP.zh-CN.md`。

## 具体的下一步

如果你只有一个会话且没有别的指示：

1. 读 `AGENTS.zh-CN.md`，再读 `docs/design/scenes.zh-CN.md`，不要把四个场景重开一遍。
2. 下一块要做的是 `docs/design/harness.zh-CN.md` 里的集成基线；如果目标是补齐 webui，就做探索。不要在做基线的同一个改动里加对等节点地图或原始配置编辑器。

场景已经在。bitswap 的问题已经回答。不要开始插件工作。

## 这份文件怎么保持真实

在改变它所述内容的**同一个 PR**里更新它。过期的交接比没有更糟，因为下一个读者会信它。

它不由 CI 强制，由 `AGENTS.zh-CN.md` 第 6 步和评审强制。发现它在漂移，先修它，再做别的。