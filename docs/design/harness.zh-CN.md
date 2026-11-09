# 测试基线：CI 里的真实发行 Kubo

**中文** · [English](harness.md)

**状态：仅设计。** 尚无代码。这份文件是 manager 的测试如何触达 Kubo 的规范。若代码与本文件冲突，以本文件为准，直到某个 PR 修改它。

下文每条结论都带证据，因为这份文件的目的就是让后来者不必凭信任接受任何东西。来源已克隆到 `reference/`，commit 见 [技术栈参考](../REFERENCE-ipfs-stack.zh-CN.md)。

## 要解决的问题

manager 通过 loopback 上的 HTTP RPC 触达 Kubo。`internal/files`、`internal/pins` 与 `internal/node` 的端点探测合计约四十次调用：`files/ls`、`files/stat`、`files/write`（multipart）、`files/read`、`files/mv`、`files/cp`、`files/rm`、`files/mkdir`、`object/stat`、`pin/add`、`pin/ls`、`pin/rm`。

目前它们只在两类东西上被验证：手写的 `httptest` 桩，以及 `TestRealKuboOptional`——后者除非 `IPFS_PANEL_INTEGRATION=1` 且 `ipfs` 在 `PATH` 上，否则跳过。没有任何 CI 任务设置这两者。**于是 RPC 面从未被 CI 触碰，而桩编码的是我们对它的一厢情愿。猜错时桩与客户端一起错，且互相印证。**

三个最可能错的猜测，点名列出，便于后来者优先核查：

1. `pins/client.go` 靠匹配 400 报文里的词，决定是否去掉 `names=` 重试 `pin/ls`。那个报文形状是我们编的。
2. `files/client.go` 把 `files/write` 发成 multipart，且不设 `Content-Length`。Kubo 的读取器是否接受，是 Kubo 的属性。
3. `object/stat` 与 `files/stat` 的响应形状是手抄的。字段改名能通过我们所有测试。

`internal/node/fakeipfs` 覆盖不了这些，也不是为此而生。它是 **`ipfs` CLI** 的替身——`init`、`daemon`、`id`、`shutdown`、`config`（`internal/node/fakeipfs/main.go:24-35`）。它的 `daemon` 持有仓库锁、写 pidfile、等信号；**从不打开 HTTP 监听**。它正确地测试进程生命周期，但它不是 RPC 服务端。

## 决定

CI 下载**真实发行的 Kubo 二进制**，把 manager 的客户端作为子进程打上去。不用 kubo-as-a-library，不用进程内节点。

理由是所验证的契约。`/api/v0` 面是 Kubo 自己的，其兼容规则是端点不删、响应形状不改（kubo `AGENTS.md`，"Never break the `/api/v0/` RPC API"）。**这条承诺是关于用户实际安装的发行二进制的**。钉在某个 commit 上的库导入测的是一个移动靶，且看不见发布回归。

代价，直说：

- CI 每次运行、每个跑该测试的平台，下载约 30 MB。
- 比纯单元测试慢：进程启动加守护进程就绪。
- 钉住的版本要有意识地升级，而升级是一次可见的 PR 变更。

本次设计的用户指示是：CI 时间可以接受，测试必须完整。这就定了这个权衡。

## 形状

```text
.github/workflows/integration.yml
  |
  +-- 按 runner 的 OS/arch 从 kubo release 下载 ipfs vX.Y.Z
  +-- 校验官方公布的 checksum
  +-- IPFS_PANEL_INTEGRATION=1
  +-- PATH 包含解包后的二进制
  +-- go test -run TestRealKubo ./services/manager/internal/node/
```

现有测试形状已经对了，不需要重写：它在 `PATH` 上找 `ipfs`，没有二进制或没有环境变量就自己跳过，并拒绝跑在 `fakeipfs` 上（`internal/node/integration_test.go:18-27`）。**缺的是提供二进制的那个 workflow。**

客户端完全不用改。它们已拒绝非 loopback 的 base URL，并从仓库配置里读 API 地址。

## 集成测试必须覆盖什么

现有 `TestRealKuboOptional` 跑 MFS 往返与 pin 列表。那是起点，不是契约。下面每一条都只能由真实二进制裁定：

| 检查 | 为什么桩替代不了 |
| --- | --- |
| `files/write` 不带 `Content-Length` 的 multipart | 是 Kubo 的 reader 决定，不是我们的 |
| 超 `MaxBytes` 的上传与半截写入清理 | 取决于 Kubo 何时中断流 |
| `pin/add` 带 `progress=true` 的形状 | 进度分帧是 Kubo 的 |
| `pin/ls` 带 `names=true`，以及 400 回退 | 回退触发于我们猜的报文 |
| `pin/add` 带 `name=`，及其回退 | 同形状，不同参数 |
| 对 CID 的 `object/stat` | 字段名与包装层 |
| `files/ls` `long=true` 的条目字段 | `Type` 编码、`Mtime`、`Hash` |
| 目录与文件的 `files/stat` | `Type` 取值不同 |
| 对间接 pin 的 `pin/rm` | 错误文本与状态码 |
| 停机的节点 | 连接被拒，而非超时 |

最后一条在 Windows 上已由 `internal/kuborpc` 覆盖，这里不重复；列出它是因为一旦二进制在手，对真实停机守护进程跑一次很便宜。

## 版本钉定

钉一个已发布版本，只在说明理由的 PR 里改动。Kubo 发出的 API 版本串是 `/kubo/<version>/`（`version.go:33`），所以不匹配会在请求里看得见，不会静默。

写作时最新发布为 `v0.43.1`（2026-09-15；克隆上的 `git ls-remote --tags`）。`master` 报 `CurrentVersionNumber = "0.44.0-dev"`（`version.go:31`），所以**克隆领先于发布，不可用作测试基线**。

以后可以加、但现在不做：

- 对上一个次版本再加一个矩阵项，双向捕捉断裂。
- 每日对 `master` 跑，仅报告，永不阻塞 PR。

在第一版三平台全绿之前，两者都不值得建。

## 历史：这份文件替换了什么

本文件的早期草稿推荐的是相反方案：把 Kubo 当库跑在测试进程内，放在 `internal/kubotest` 下，而把发行二进制测试保留为可选。它被替换掉，是因为这里验证的是 Kubo 自己的 `/api/v0` 面，而那条兼容承诺是**关于用户实际安装的发行二进制**做出的。钉在某个 commit 上的库导入测的是移动靶，看不见发布回归。促成这次改动的明确要求是：CI 时间可以接受、测试必须完整。

如果后来的读者在某个分支、旧评审或草稿里又看到进程内方案，这一节就是它为什么不是当前设计的原因。**不要把它作为第二条路径加回来**：两套测试基线意味着对"manager 能不能对 Kubo 工作"有两个答案。

## 不在本设计内

- 替换 `fakeipfs`。它测的是另一层，且做得对。
- 进程内导入 `github.com/ipfs/kubo`。已在上面的[历史](#历史这份文件替换了什么)一节否掉；那会为了测"别的东西"而给 `go.mod` 塞进一大坨依赖。
- 测网关。切片 1–4 里 manager 既不服务也不配置它。
- 多节点测试。那是 roadmap 切片 6。

## 待定问题

- **跑哪些平台。** Windows、Linux、macOS 全覆盖最理想。三平台都有二进制；先用 CI 现有的 runner，等出现平台相关 bug 再加。
- **测试自己起守护进程，还是假定已有一个。** `TestRealKuboOptional` 用 supervisor 起，顺带用真实二进制验了生命周期。保留这个做法。假定已有守护进程的测试就测不到我们的启动路径。
- **每个测试一个仓库目录。** 现有测试用 `t.TempDir()`。Kubo init 要生成密钥；在这个规模可以接受。