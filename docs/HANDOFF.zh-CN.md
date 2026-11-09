# 交接

**中文** · [English](HANDOFF.md)

先读这份，再读 `AGENTS.zh-CN.md`。这份文件是工作状态：交付了什么、定了什么、还开着什么、下一步做什么。它不是教程也不是计划，而是"现在什么是真的"的答案。

最后更新于 `develop` = `5a9a648`。

## 仓库现状

| | |
| --- | --- |
| `main` | `5105641` —— v0.1.0 发布提交 |
| `develop` | `5a9a648` |
| 最新 tag | `v0.1.0` |
| 打开的 PR | 无 |
| 路线图 | 切片 1–4 已发布；切片 5（场景）未开始；切片 6（专家模式、多节点）更晚 |

v0.1.0 已交付：英文与中文的面板外壳、首次运行建管理员与会话登录、节点启停与一句话状态、可变文件系统上的文件页、固定（pins）页。linux、macOS、Windows 三平台二进制加 web 包，作为 GitHub Release 发布并附校验和。

## 已定的事

以下已定。没有理由、没有说明原因的 PR，不要重开。

- **只有 manager 调用 Kubo。** 浏览器永远拿不到 Kubo RPC 地址。见 `docs/VISION.zh-CN.md` 与 `docs/design/auth.zh-CN.md`。
- **Kubo 集成暂时留在进程边界。** `docs/design/kubo-integration.zh-CN.md` 把它与 Kubo 插件做过比较，并推荐边界。插件这条路是真的，但代价是 Windows 上要分叉 Kubo 构建，以及一个 Kubo 明确不保证稳定的 fx 图。
- **测试基线是发行的 Kubo 二进制，不是库导入。** `docs/design/harness.zh-CN.md`。进程内方案被考虑过并被否；文件里记了原因。
- **依赖是钉定的，参考克隆也钉在确切 commit。** 表格在 `docs/REFERENCE-ipfs-stack.zh-CN.md`；`.github/scripts/fetch-reference.sh` 可复现。

## 还开着的事

每一条都需要**决定**，不是代码。

1. **面板是否有任何事必须靠节点的 bitswap，而网关调用做不到？** 这个问题决定插件模型背后有没有功能支撑。答案是否，插件就没有东西可承载，边界成立。**写插件代码之前先回答它。**
2. **切片 5（场景）还没有设计。** `docs/ROADMAP.zh-CN.md` 说明了它的用途：一种能用一句话说清的用法，面板把它写进 Kubo 配置。实现前需要 `docs/design/scenes.md`，其中包括每个场景写哪些配置键、哪些改动需要重启、节点正在运行时怎么办。
3. **集成测试基线已设计但未实现。** `docs/design/harness.zh-CN.md` 规定了一个 CI 任务：下载发行版 Kubo 二进制，用它跑现有的 `TestRealKuboOptional`。今天那个测试除非开发者设 `IPFS_PANEL_INTEGRATION=1` 且 `PATH` 上有 `ipfs`，否则跳过，**所以 CI 从未碰过 RPC 面**。
4. **是否为 minimatch 告警发 0.1.1。** 修复已在 `develop`（`8dc1b86`），`main` 仍是旧版本，所以 Dependabot 告警开着。该告警只涉及开发依赖，不进产品包。曾准备过 `hotfix/0.1.1` 后主动弃置，等"补丁发布值不值得"这个决定。

## 值得点名的缺口

- **CI 里 RPC 面未被测试。** `docs/design/harness.zh-CN.md` 点名了三个猜测：`pin/ls` 的拒绝报文形状、不带 `Content-Length` 的 multipart、`object/stat` 的响应形状。今天任何一个都可能是错的，而没有测试会说。
- **`docs/design/*.md` 是权威，其中两份描述的东西还不存在。** `harness.md` 只有设计，`kubo-integration.md` 只有设计。读它们的 status 行。
- **Windows 覆盖依赖有人在那边跑测试。** CI 只有 ubuntu。切片 1–4 里的三个 Windows 缺陷，全是有人在 Windows 上跑 `go test` 才发现的。
- **`ci-ok` 要求八个任务，而 `web`/`manager` 在路径未变时跳过。** 这是有意的，`branch-policy` 里说明了规则集。加必需检查时，要同时改 `.github/workflows/ci.yml` 与 `docs/ORG-SETUP.zh-CN.md`。

## 具体的下一步

如果你只有一个会话且没有别的指示：

1. 读 `AGENTS.zh-CN.md`，再读 `docs/ROADMAP.zh-CN.md` 的切片 5。
2. 把上面第 1 个开放问题**说出来**，写进 `docs/design/kubo-integration.zh-CN.md`。它是一句话，而且永久解锁插件这个问题。
3. 如果目标改成切片 5，先写 `docs/design/scenes.md`。`docs/VISION.zh-CN.md` 点名的四个场景是：安静的节点、分享文件夹、固定一组内容、只读公共网关。每个都要从 `reference/kubo/docs/config.md` 找出它写的配置键，并附引用。

**不要从代码开始。** 这个仓库每个切片都从设计文档开始；两个先写代码后补文档的，正是两个返工的。

## 这份文件怎么保持真实

在改变它所述内容的**同一个 PR**里更新它。过期的交接比没有更糟，因为下一个读者会信它。

它不由 CI 强制，由 `AGENTS.zh-CN.md` 第 6 步和评审强制。发现它在漂移，先修它，再做别的。