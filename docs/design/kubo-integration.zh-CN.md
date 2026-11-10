# Kubo 集成：进程边界还是插件

**中文** · [English](kubo-integration.md)

**状态：已决定。** 面板留在进程边界上。这份文件比较面板挨着 Kubo 的两种摆法。下面的 bitswap 问题已经回答。不要开始插件工作。

每条结论都带证据。来源已克隆到 `reference/`，commit 见 [技术栈参考](../REFERENCE-ipfs-stack.zh-CN.md)。

## 问题

面板管理一个 Kubo 节点。它的代码可以跑在两个地方：

1. **Kubo 进程之外。** 面板是独立二进制，通过 `127.0.0.1:5001` RPC 跟 Kubo 说话。这就是今天 `services/manager` 的做法。
2. **Kubo 进程之内。** 面板是 Kubo 插件，代码跑在守护进程里，共享节点的 libp2p host、blockstore、bitswap 与 DHT。没有 RPC 跳转，没有第二个进程。

第二种模型被赋予的目标是：加入**既有的** IPFS 网络，而不是在旁边另起一张私有网络。这个目标值得精确回答，因为**今天两种模型都已经在同一个网络上**。模型 1 对话的本地 Kubo 本身就是普通 IPFS 节点，跟任何节点一样在公网上 peering。差别不在于面板**是否**在网络里，而在于它**能看到和改动多少节点内部**，以及为此要跑多少机械。

## 模型 1 的确切形状

`services/manager` 绑 loopback，持有管理员会话，代理一组固定的 Kubo RPC 调用。Kubo 是它启动和停止的子进程。

本仓库里的证据：

- `services/manager/internal/node/lifecycle.go` 生成 `ipfs init`/`daemon`/`shutdown`，并把 API 地址钉在 loopback。
- `services/manager/internal/kuborpc/kuborpc.go` 拒绝任何非 http loopback 的 base URL，其 dialer 在每次连接时重新校验目标。
- `internal/files` 与 `internal/pins` 各自拥有 `/api/v0` 一片的客户端。

它放弃的是 RPC 面没有暴露的东西。它保住的是面板已经投入的安全边界：manager 负责认证，浏览器永远拿不到 Kubo 地址（`docs/VISION.md`）。

## 模型 2 的确切形状

Kubo 有插件系统，六个接口（`plugin/*.go`）：

| 接口 | 文件 | 拿到什么 |
| --- | --- | --- |
| `PluginDaemon` | `plugin/daemon.go:9` | `Start(coreiface.CoreAPI)` ——节点已构建完毕 |
| `PluginDaemonInternal` | `plugin/daemoninternal.go:10` | 内部状态，定义上就不稳定 |
| `PluginFx` | `plugin/fx.go:17` | `Options(core.FXNodeInfo) ([]fx.Option, error)` ——重接依赖图 |
| `PluginDatastore` | `plugin/datastore.go:9` | 一个 datastore 后端 |
| `PluginIPLD` | `plugin/ipld.go:9` | 一个 IPLD 编解码 |
| `PluginTracer` | `plugin/tracer.go:8` | 一个追踪后端 |

对"低侵入"而言，关键是 `PluginFx`：它追加 fx option，文档给出的做法是对图中已有接口用 `fx.Replace()` 或 `fx.Decorate()`（`plugin/fx.go:10-15`）。`exchange.Interface`（`core/node/core.go:38`）与 `pin.Pinner`（`core/node/core.go:62`）都产自该图，所以可以不改 Kubo 源码就替换掉它们。

`PluginDaemon` 是更简单的入口：`Start(coreiface.CoreAPI)` 在节点就绪后运行，拿到真实 CoreAPI。**一个只读取和驱动节点的面板，需要它，不需要别的。**

### 平台分裂是真的，且绕不过

外部插件通过 Go 的 `plugin` 包加载，底层是 `dlopen`。Kubo 的加载器按 build tag 选实现：

- `plugin/loader/load_unix.go` —— `//go:build (linux || darwin || freebsd) && cgo && !noplugin`，调用 `plugin.Open`。
- `plugin/loader/load_nocgo.go` —— `//go:build (linux || darwin || freebsd) && !cgo && !noplugin`，返回 "not built with cgo support"。
- `plugin/loader/load_noplugin.go` —— `//go:build noplugin`，返回 "not built with plugin support"。

**这套里没有 Windows 的文件。** Go 直接写明：*"Plugins are currently supported only on Linux, FreeBSD, and macOS"*（`$GOROOT/src/plugin/plugin.go` 警告段）。Kubo 自己的文档对外部插件也是同样说法：*"this method is only supported on Linux and MacOS at the moment"*（`docs/plugins.md:136-137`）。

所以 Windows 上唯一的路径是**预加载**插件：加进 `plugin/loader/preload_list` 再重新编译 Kubo。`plugin/loader/preload.sh` 从那个列表生成 `preload.go`，Makefile 目标是 `make build`。**这对 Windows 用户而言是对 Kubo 构建的一次真实分叉**——这是分发问题，不是代码问题。

## 权衡，摆成权衡

| | 模型 1：进程边界 | 模型 2：插件 |
| --- | --- | --- |
| 触及节点内部 | 仅 `/api/v0` 暴露的 | 任何被 fx 装饰的接口 |
| 共享 libp2p host | 否（独立进程） | 是，同进程 |
| 额外进程 | 有 | 无 |
| Windows 可用 | 可用，无需改动 | 只能作为 Kubo 的预加载分叉 |
| Linux/macOS 分发 | 一个二进制 + Kubo | Kubo 二进制 + 每平台一个 `.so`，且工具链匹配 |
| 需匹配工具链 | 否 | **是** ——Go 插件 ABI 要求工具链、build tag、依赖源码完全一致 |
| 发布节奏耦合 | RPC 契约 | Kubo 的 fx 图，Kubo 明确不保证其稳定 |
| 安全边界 | 面板负责认证；浏览器拿不到 RPC 地址 | 插件以完整节点权限运行 |
| 测试面 | 发行二进制（见[测试基线](harness.zh-CN.md)） | 必须从源码编译 Kubo |

其中两行要强调，因为它们最容易被略过：

- **Go 插件 ABI。** Go 自己的文档警告：除非应用与每个插件都用**完全相同**的工具链版本、相同的 build tag、某些 flag 与环境变量的相同取值，且所有共享依赖来自同一份源码，否则很可能运行期崩溃（`$GOROOT/src/plugin/plugin.go` 警告段）。对一个用户从 release 安装 Kubo 的项目来说，这意味每个面板版本都必须针对**恰好那个** Kubo 版本编译出一份 `.so`。那叫支持矩阵，不叫功能。
- **fx 稳定性。** Kubo 在这点上说的是相反的：*"Kubo cannot guarantee backwards compatibility for `fx` customizations"*（`docs/plugins.md` fx 一节），而 `plugin/fx.go:10-13` 称其 invasive，并警告版本间可能出现破坏性变更。简单的接口替换是安全子集；更深的改动会坏。

## 那个目标，正面回答

"接入 IPFS 网络而不是另起一张"在模型 1 下**已经成立**，因为本地 Kubo 就是完整的网络节点。如果目标其实是"*面板自己的逻辑应当参与节点的联网*"——例如用节点的 libp2p host 服务内容，或经节点的 bitswap 应答请求——那么模型 2 是唯一做法，而**Windows 分叉就是它的价钱**。

这个区分才是要做的决定，应当明明白白做出，而不是顺带发生。

## 我的建议

保留模型 1 作为产品形态；只有当某个具体功能确实需要进程内访问时，再加一个小的 `PluginDaemon` 适配层。理由按重要性：

1. 面板的职责按文档是让**本地**节点好用，而这靠 RPC 就能做到。切片 1–4 里没有任何东西需要节点内部。
2. 模型 2 的 Windows 代价是对 Kubo 的一次分叉，这与"用户按自己的方式安装 Kubo"冲突。
3. 模型 2 把一个有版本号的 HTTP 契约换成 Kubo 明确拒绝保证稳定的 fx 图。
4. `docs/design/auth.md` 的安全模型假定浏览器永远触不到 Kubo。插件把面板代码放进了**就是** Kubo 的那个进程里。

若将来确有功能需要进程内访问，最窄的形态是交出句柄的 `PluginDaemon`，不是重接图的 `PluginFx`，更不是 `PluginDaemonInternal`。

## bitswap 问题，已回答

没有。网关的一次读取由节点的 blockservice 提供（`core/node/core.go:38-40`），bitswap 开着时这个服务的 exchange 就是节点的 bitswap（`core/node/bitswap.go:208-220`）；`Gateway.NoFetch` 则换成离线 exchange（`core/corehttp/gateway.go:160-161`）。面板的工作是 Kubo RPC 或写配置，没有一件需要 bitswap 账本、wantlist，或跑在 Kubo 进程里面的代码，所以插件模型背后仍然没有功能。

场景写的是配置文件。它们不调用 bitswap。见[场景](scenes.zh-CN.md)。

## 待定问题

- **面板是否有任何事必须靠节点的 bitswap，而网关调用做不到？** 没有。上面已回答。模型 2 背后仍然没有功能。
- **Windows 预加载插件是否意味着我们要发自己的 Kubo 构建？** 是，而且要回答由谁更新、如何验证。
- **插件是否改变认证模型？** 插件不能被浏览器直接经 HTTP 触达，所以 `docs/design/auth.md` 仍适用于面板暴露的东西；但插件本身不受沙箱约束。这条在任何插件代码落地前必须写下来。