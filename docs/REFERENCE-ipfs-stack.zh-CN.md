# IPFS Go 技术栈：该读什么，以及什么是真的

**中文** · [English](REFERENCE-ipfs-stack.md)

这份文档写给接手这个仓库的人，尤其是 AI。它只回答一个问题：**当你要确认 IPFS Go 技术栈的行为时，真相在哪，你怎么自己验证？**

面板从不猜测 Kubo 的行为。设计文档或 PR 里的每条结论都应当能追溯到本文的某个来源。**指不出文件和行号，就写"未核实"，不要写成事实。**

## 为什么有这份文档

做切片 1–4 和发布 v0.1.0 的过程中，有三条说法被写下来、后来又被推翻。每一条如果用源码核对，都不会发生：

1. **"扩展 fakeipfs 这个假 CLI"** —— 错。假 CLI 只测进程生命周期，从不实现 RPC。
2. **"只用 boxo 就能在测试里替代 Kubo"** —— 错。boxo 有 `mfs`、pinner、offline exchange，但**没有 HTTP RPC 服务端**。`/api/v0` 那套在 Kubo 里。
3. **"进程内库节点是对的测试基线"** —— 对本项目是错的。被验证的契约是**关于用户实际装的那个发行二进制**做的。

模式完全一样：凭记忆给出一个听起来合理的答案，而源码只隔一个 `grep`。本文就是补这个洞。

## 来源与固定版本

已克隆到 `reference/`（已 gitignore，永不提交）。本仓库所有文档的结论都对照这些确切 commit 核过。

| 仓库 | Commit | 日期 | 许可证 |
| --- | --- | --- | --- |
| `ipfs/kubo` | `833e04a240d8c533509ddaaa3c202b39b0bb3024` | 2026-09-27 | Apache-2.0 + MIT |
| `ipfs/boxo` | `3d6ac39a0f76c824ffbb90e3a729e78f62bc2d12` | 2026-09-21 | Apache-2.0 + MIT |
| `ipfs/helia` | `cba1ed69b4369924790d9eccdb8c9089435f4f20` | 2026-09-29 | Apache-2.0 + MIT |
| `ipfs/ipfs-docs` | `1261c016f2b6c700e9354e46cedcdf38c9ad7b91` | 2026-09-27 | — |
| `ipfs/specs` | `3000be69497e47c399cdb719440d0809f085388b` | 2026-09-17 | — |

按这些确切 commit 建立或刷新克隆：

```bash
.github/scripts/fetch-reference.sh
```

脚本是幂等的：已在指定 commit 的跳过、缺失的克隆、过期的移到指定 commit。**不要**在这些目录里跑 `git reset --hard origin/HEAD`——那会移到 `master`，本文件里所有行号立刻失效。

提升钉定版本是一次有意的变更：改脚本里的 commit、更新上面的表，并重新核对每条引用（见[写之前怎么验证](#写之前怎么验证)）。

`git ls-remote --tags https://github.com/ipfs/kubo` 给出已发布版本。写作时最新为 `v0.43.1`（2026-09-15）。克隆的 `kubo/version.go:31` 是 `0.44.0-dev`，所以**克隆领先于发布**——永远不要拿 `master` 当测试基线。

## 分工：kubo 还是 boxo

Kubo 自己的 `AGENTS.md` 写明了规则，值得记住，因为它直接告诉你该往哪看：

> Boxo 是 IPFS 的 Go SDK……Kubo 只是 boxo 的一个消费者，不是唯一一个，所以可复用的积木放在 boxo……**属于 kubo 的：**守护进程与产品，例如配置 schema、CLI 命令（`core/commands/`）、`/api/v0/` RPC 面、节点构建与生命周期、磁盘上的仓库、插件、迁移。
> —— `reference/kubo/AGENTS.md`

于是：

| 你想知道 | 读哪里 |
| --- | --- |
| `/api/v0/files/*`、`/api/v0/pin/*` 的请求与响应形状 | `kubo/core/commands/` |
| RPC handler 如何挂载与提供服务 | `kubo/core/corehttp/`、`kubo/core/corehttp/commands.go` |
| 配置文件 schema、默认值、`docs/config.md` | `kubo/config/`、`kubo/docs/config.md` |
| 节点构建、fx 依赖图 | `kubo/core/node/`、`kubo/core/core.go` |
| 插件接口与加载 | `kubo/plugin/`、`kubo/plugin/loader/` |
| MFS 语义、目录 CID 的形成 | `boxo/mfs/` |
| 固定（pinning）内部：`pin.Pinner` | `boxo/pinning/pinner/` |
| UnixFS 导入：分块、DAG 布局 | `boxo/ipld/unixfs/`、`boxo/chunker/` |
| 测试用的离线区块访问 | `boxo/exchange/offline/` |
| 区块与 blockstore 层 | `boxo/blockstore/`、`boxo/blockservice/` |
| 协议定义（线上格式） | `specs/` ——这是真相之源，不是任何实现 |

## 值得预先掌握的事实

每条都对照克隆核过。要确认就跟引用走。

### `/api/v0` 契约

- handler 由 `corehttp.CommandsOption(cctx)` 挂载，它调用 `commandsOption(cctx, corecommands.Root)` —— `kubo/core/corehttp/commands.go:124,202`。
- `commands.Root` 的子命令静态注册在 `rootSubcommands` 里，包括 `"files": FilesCmd`（134 行）与 `"pin": pin.PinCmd`（154 行）—— `kubo/core/commands/root.go`。**命令存在不需要插件。**
- 兼容规则是 kubo 的项目规矩，不是代码细节：永不删除端点、重命名字段、改变默认值或改变响应形状。新增端点或可选参数是允许的 —— `kubo/AGENTS.md`。
- API 版本串是 `/kubo/<version>/` —— `kubo/version.go:33`。

### MFS 与 files

- `files/write` 是 multipart。服务端读取器是 Kubo 的；本仓库的客户端是手写的。这个不对称正是 multipart 路径必须用真实二进制验证的原因。
- **`files/ls` 与 `files/stat` 对 `Type` 的编码不同，这里极易出错。** `files/ls` 发整数 `int(fsn.Type())` —— `kubo/core/commands/files.go:761`；`files/stat` 发字符串 `"directory"` 或 `"file"` —— 同文件 412-414 行。Kubo 侧的形状是 `lsEntry.Type int` 与 `statOutput.Type string`；本仓库相应地为 `lsEntry.Type int` 与 stat 路径上的 string。**用一个类型套两处的客户端必然在一处失败，而手写桩会同意作者随手选的那个。**
- `files/ls` 带 `long=true` 返回 `Name`、`Type`、`Size`、`Hash`、`Mtime`、`MtimeNsecs` —— `kubo/core/commands/files.go:168-180`。
- MFS 根操作走 `boxo/mfs`，其 HAMT 参数决定目录的 CID —— `boxo/AGENTS.md` 的「mfs/, files/, blockservice/」一节。

### 固定（pinning）

- `pin.Pinner` 在 fx 图里生成 —— `kubo/core/node/core.go:62`。因此它可以用 `fx.Decorate` 替换。
- `pin/ls` 接受 `names` 与 `stream`。本仓库在 Kubo 拒绝时去掉 `names` 重试；那个拒绝报文的具体形状是猜测，已在 [测试基线](design/harness.zh-CN.md)里如实标出。

### 插件

- 六个接口，都在 `kubo/plugin/`：`PluginDaemon`（`kubo/plugin/daemon.go:9`）、`PluginDaemonInternal`（`kubo/plugin/daemoninternal.go:10`）、`PluginFx`（`kubo/plugin/fx.go:17`）、`PluginDatastore`（`kubo/plugin/datastore.go:9`）、`PluginIPLD`（`kubo/plugin/ipld.go:9`）、`PluginTracer`（`kubo/plugin/tracer.go:8`）。
- `PluginFx.Options(core.FXNodeInfo) ([]fx.Option, error)` —— 文档明确的做法是对已有接口用 `fx.Replace()` 或 `fx.Decorate()` —— `plugin/fx.go:10-17`。
- `PluginDaemon.Start(coreiface.CoreAPI) error` —— 节点已构建完毕，这是"读取并驱动"的入口 —— `plugin/daemon.go:9-13`。
- **外部插件只支持 Linux、FreeBSD、macOS。** Go 在 `$GOROOT/src/plugin/plugin.go` 的警告段写明；Kubo 在 `kubo/docs/plugins.md:136-137` 写明。build tag 证实：`load_unix.go` 与 `load_nocgo.go` 都限定 `(linux || darwin || freebsd)`，**没有 Windows 加载器**。
- 因此 Windows 只能走**预加载**插件：加进 `plugin/loader/preload_list` 再重新编译 —— `plugin/loader/preload.sh`，以及 `kubo/docs/plugins.md` 的「Preloaded Plugins」。
- Go 的插件 ABI 要求插件与宿主使用**完全相同**的工具链版本、相同的 build tag、相同的共享依赖，否则容易崩溃 —— `$GOROOT/src/plugin/plugin.go` 警告段。
- Kubo 不承诺 fx 稳定：**"Kubo cannot guarantee backwards compatibility for `fx` customizations"** —— `kubo/docs/plugins.md`。

### 不联网也能测节点

- `boxo/exchange/offline.Exchange(bs)` 只读本地 blockstore，未命中返回 "block was not found locally (offline)"。零网络 —— `boxo/exchange/offline/offline.go`。
- `kubo/core.BuildCfg{Online: false, Routing: libp2p.NilRouterOption}` 构建不联网节点 —— `kubo/core/node/builder.go`，以及 `kubo/docs/examples/kubo-as-a-library/main.go`。
- 但**对本项目**，测试基线是发行的二进制，不是库导入。见[测试基线](design/harness.zh-CN.md)。

### Helia 与 Kubo 在同一个网络上

这条在有人问"JavaScript 面板会不会自成一张网"时很关键。答案是不会。

- Helia 的 interop 套件用 `ipfsd-ctl` 启动真实 Kubo 并 dial 它：`await helia.libp2p.dial((await (await kubo.api.id())).addresses)` —— `reference/helia/packages/interop/src/car.spec.ts:27-30`。
- fixture 跑的是 `kubo` npm 包里的二进制（`bin: kuboPath()`），并固定导入 profile —— `packages/interop/src/fixtures/create-kubo.ts`。
- README 原文：*"An interop suite ensures everything is compatible"* —— `helia/README.md:223`。

所以 Helia、Kubo、以及任何连它们之一的面板，都在同一条 libp2p 网络上。**"接入网络"不构成选择进程内设计的理由**——网络是靠 peering 接入的，两种模型都能做到。

## 写之前怎么验证

1. **在克隆里找到它。** `grep -rn "那个东西" reference/kubo reference/boxo`。搜不到，就是你没有这条结论。
2. **读上下文代码**，不只是命中的那一行。函数名不等于语义。
3. **确认克隆是不是发布版。** 对 kubo 跑 `git ls-remote --tags`。`master` 带 `-dev` 版本号。
4. **线上格式以 spec 为准。** `reference/specs/` 的权威高于任何实现，包括 Kubo。
5. **未核实就写"未核实"。** 一个标注过的未知，比一个自信的错误更值钱——本仓库已经纠正过三条后者。

机械性的那一半由两个脚本兜住：

| 脚本 | 在哪跑 | 检查什么 |
| --- | --- | --- |
| `.github/scripts/check-docs.sh` | CI，任何 `docs/**` 变更 | 每份文档都有 `.zh-CN` 配对、语言切换存在、相对链接可解析 |
| `.github/scripts/check-doc-citations.sh` | 本地，需要 `reference/` | 本目录文档里每个 `file:line` 都能解析且不越界 |

改完文档、或改了 `fetch-reference.sh` 里的钉定 commit 之后，跑第二个。引用指向不存在的文件、或越过文件末尾时它以非零退出——那比没有引用更糟。

## 不要从本文得出

- 它不是 IPFS 的综述，只是"去哪看"的地图。
- 固定的 commit 会变老。引用行号前先刷新、先复核。
- 关于 `master` 的事实不等于关于发布版的事实。心里要把两者分开，写明你说的是哪个。