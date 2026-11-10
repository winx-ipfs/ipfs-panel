# 测试基线：CI 里的真实发行 Kubo

**中文** · [English](harness.md)

**状态：已实现。** 指向 `develop` 的 pull request 会下载一个钉定的 Kubo 发行版，并用它跑 `TestRealKuboOptional`。没有设置 `IPFS_PANEL_INTEGRATION=1`、或者 `PATH` 上没有 `ipfs` 的机器，测试仍然跳过。若代码与本文件冲突，以本文件为准，直到某个 PR 修改它。

下文每条结论都带证据，因为这份文件的目的就是让后来者不必凭信任接受任何东西。来源已克隆到 `reference/`，commit 见 [技术栈参考](../REFERENCE-ipfs-stack.zh-CN.md)。

## 要解决的问题

manager 通过 loopback 上的 HTTP RPC 触达 Kubo。`internal/files`、`internal/pins` 与 `internal/node` 的端点探测合计约四十次调用：`files/ls`、`files/stat`、`files/write`（multipart）、`files/read`、`files/mv`、`files/cp`、`files/rm`、`files/mkdir`、`object/stat`、`pin/add`、`pin/ls`、`pin/rm`。

单元测试用的是手写的 `httptest` 桩。`TestRealKuboOptional` 只在 `IPFS_PANEL_INTEGRATION=1` 且 `ipfs` 在 `PATH` 上时才跑。指向 `develop` 的 pull request 会把这两样都设上，并把钉定的二进制放进 `PATH`。本地没有这两样时，测试仍然跳过。桩编码的是我们自己的猜测。猜错时桩与客户端一起错，并且互相印证，所以发行二进制必须在 CI 里跑。

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
.github/workflows/ci.yml
  integration                 目标分支是 develop 的 pull request
    uses .github/workflows/integration.yml
      |
      +-- .github/scripts/fetch-kubo.sh
      |     从 GitHub Release 下载 kubo_v0.43.1_linux-amd64.tar.gz
      |     官方公布的 sha512 必须等于脚本里的钉
      |     然后压缩包必须对上这个 sha512
      +-- IPFS_PANEL_INTEGRATION=1
      +-- IPFS_PANEL_KUBO_VERSION 来自脚本（0.43.1）
      +-- PATH 包含解包后的二进制
      +-- go test -count=1 -run TestRealKubo ./services/manager/internal/node/
      +-- 跳过即失败
  ci-ok needs integration
```

`integration.yml` 是 `workflow_call` 工作流，不是单独的必须检查。规则集只要求 `ci-ok`。另一个工作流里的任务没法成为 `ci-ok` 的 `needs`。把 `integration` 自己设成必须检查会挡住合并：它在 push 上、以及指向 `main` 的 pull request 上会跳过，而被跳过的必须检查会挡住合并。`ci-ok` 对那些不是每次事件都跑的任务，已经把跳过当成成功。这个任务失败时，`ci-ok` 会像 `manager` 失败一样失败。

`-count=1` 是为了不让一次被缓存的跳过通过。测试在 `PATH` 上找 `ipfs`，没有二进制或没有环境变量就自己跳过，并拒绝跑在 `fakeipfs` 上（`internal/node/integration_test.go:21-30`）。工作流在发生这种跳过时失败，因为跳过的 `go test` 退出码是 0。

RPC 客户端不用改。它们已经拒绝非 loopback 的 base URL。测试用 `LoopbackEndpoints` 从仓库读 API 源。supervisor 的就绪探测会先等仓库里的 `api` 文件，再调用 `ipfs id`。不等的话，`ipfs id` 会打开仓库、拿走 `repo.lock`，真实守护进程就会退出。这个竞态对 `fakeipfs` 是看不见的。记在[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)。

## 集成测试必须覆盖什么

`TestRealKuboOptional` 保留 MFS 往返，并加上下面这些检查。每一条都只能由真实二进制裁定：

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

最后一条在 Windows 上已由 `internal/kuborpc` 覆盖。集成测试在 supervisor 停掉守护进程之后，也对真实进程再跑一次，因为二进制在手时这一下很便宜。断言是 `node_not_running`，也就是连接被拒，不是超时。

### v0.43.1 裁定了什么

下面是跑发行版 `v0.43.1` 二进制得到的，不是读参考克隆。克隆领先于这个发行版。

- 不带 `Content-Length` 的 multipart `files/write` 会被接受。内容能原样读回。
- 超过 `MaxBytes` 的上传返回 `too_large`，之后那条路径不存在。
- 带 `progress=true` 的 `pin/add` 先发出带 `Progress`（节点数）的 JSON 对象，再发出带 `Pins` 的对象。仓库里已有的一个文件，本地固定时 `Progress` 是 2。
- 带 `names=true` 的 `pin/ls` 和带 `name=` 的 `pin/add` 都会被接受。`NamesFrom` 是 `kubo`，名称会出现在这条固定上。对这个发行版，400 回退不会发生。如果这个发行版开始用客户端认不出的报文拒绝这些参数，测试失败；如果回退真的发生，测试也失败，因为那就表示这个发行版不再接受它们。
- `POST /api/v0/object/stat` 返回 HTTP 500，正文是 `{"Message":"removed, use 'ipfs dag' or 'ipfs files' instead","Code":0,"Type":"error"}`。`FillSizes` 让 `Size` 保持未设置。一旦出现大小，测试就失败。`dag/stat` 会遍历 DAG，而[固定](pins.zh-CN.md) 不做这件事，所以客户端仍调用 `object/stat`，失败就保持失败。
- `files/ls` 带 `long=true` 返回 `Name`、`Type`（整数：0 是文件，1 是目录）、`Size` 和 `Hash`。它不返回 `Mtime`。`files/stat` 的 `Type` 是字符串 `file` 或 `directory`。`ipfs files touch --mtime` 之后，`files/stat` 会带回那个 `Mtime`，列表条目的修改时间来自这次 stat。
- 对一个只被间接固定的 CID 做 `pin/rm`，返回 HTTP 500，正文是 `{"Message":"not pinned or pinned indirectly","Code":0,"Type":"error"}`。客户端把它映射成 `indirect`。
- supervisor 停掉守护进程之后，files stat 返回 `node_not_running`。

`IPFS_PANEL_KUBO_VERSION=0.43.1` 把这些结果钉死。任何别的非空值都会让测试失败，直到那个升级 PR 写上新的预期。空值是本地运行：调用仍然必须成功，两种名称结果必须自洽（`kubo` 且带名称，或者回退之后是 `panel`）。

## 版本钉定

钉一个已发布版本，只在说明理由的 PR 里改动。Kubo 发出的 API 版本串是 `/kubo/<version>/`（`version.go:33`），所以不匹配会在请求里看得见，不会静默。

钉是 `v0.43.1`，写在 `.github/scripts/fetch-kubo.sh` 里，附带 `kubo_v0.43.1_linux-amd64.tar.gz` 的官方 sha512。2026-11-10 又对过一次 `git ls-remote --tags https://github.com/ipfs/kubo` 和 GitHub releases API：`v0.43.1`（2026-09-15）仍是最新的稳定发行。名为 `v0.5.x` 的标签比 `v0.43.1` 更旧。参考克隆仍报 `CurrentVersionNumber = "0.44.0-dev"`（`version.go:31`），所以**克隆领先于发布，不可用作测试基线**。钉留在 `v0.43.1`，是因为它仍是最新发行，也就是用户实际安装的那个二进制。

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

## 随这个任务一起定下的

- **跑哪些平台。** Ubuntu，也就是 CI 现有的 runner。只有 linux/amd64 的校验和被钉住。脚本拒绝任何别的操作系统或架构。等出现平台相关 bug 再加平台，并在同一个 PR 里写上那个平台的官方校验和。现在没有 macOS 或 Windows 矩阵。
- **测试自己起守护进程。** `TestRealKuboOptional` 用 supervisor，顺带用真实二进制验了生命周期。假定已有守护进程的测试就测不到我们的启动路径。
- **仓库目录。** `t.TempDir()`。Kubo init 要生成密钥；在这个规模可以接受。