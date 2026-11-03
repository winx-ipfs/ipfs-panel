# 节点启停

[English](DESIGN-node-lifecycle.md) · **中文**

**状态：已实现。** 本文仍然是规格。以后代码和本文不一致时，以本文为准，直到有 pull request 改这份说明。manager 可以启动、停止和重启本机上的一个 Kubo。登录仍然不属于这一片。

这一片只做本机上的一个 Kubo：启动、停止、重启，以及新手能用一句话读懂的健康概览。[路线图](ROADMAP.zh-CN.md)里这是第 2 片。登录仍是第 1 片，本文不设计登录。

## 用词

- **Kubo** 是 IPFS 的程序。命令叫 `ipfs`。
- **manager**（管理服务）是 `services/manager` 里的 Go 进程。浏览器只和它说话。
- **回环** 指 `127.0.0.1`（或 `::1`）。这台机器上的程序可以连。别的机器不行。
- **仓库（repo）** 是 Kubo 存放身份和数据的文件夹。变量 `IPFS_PATH` 指向它。
- **子进程** 是 manager 启动、并且由 manager 负责的程序。这一片里就是 `ipfs daemon`。

## 这个画面要回答什么

新手不用翻 Kubo 文档，也能回答三件事：

1. 节点在不在跑？
2. 怎么启动、停止、重启？
3. 失败了，下一步是什么？

第 1 问的答案是一句话。节点在跑时，再加上 Kubo 版本和 Peer ID。Peer ID 是这个节点的身份号码。答案不是把 Kubo RPC 的响应倒在屏幕上。

## 请在实现之前确认的决定

实现按这些做。要改，先在改这份文档的 pull request 里提出来，再写代码。

1. `GET /health` 仍然只表示 **manager 这个进程** 活着。它现在返回 `{"status":"ok","service":"manager"}`。节点到底怎么样，看新的 `GET /api/node`。
2. 面板用自己的仓库目录，不用 `~/.ipfs`。
3. manager 已经听在 `127.0.0.1:8080`。Kubo 默认的网关也用这个端口。面板创建仓库时，把 API 设为 `127.0.0.1:5001`，把网关设为 `127.0.0.1:8081`。
4. 以后每次启动只检查这两个地址。API 或网关不是回环，就拒绝启动，不再改写配置。
5. Swarm（Kubo 连别的节点的端口，通常是 4001）保持 Kubo 的默认。安静地跑是场景，属于后面的切片。
6. 状态只有 `down`、`starting`、`up`、`stopping`、`failed`。重启就是先停再启动，不是第六个状态。
7. 登录是另一份规格。这些路由需要会话。其他防护不变：不发 CORS 头，动作类请求必须带 JSON 的 Content-Type。没有会话是 **401** `{"error":"unauthorized"}`。`GET /health` 仍然开放。
8. 界面请求相对路径（`/api/node`、`/health`）。开发时由 Vite 代理到 manager。打包后的页面里不出现 Kubo 的地址。

## 谁来跑 Kubo

```text
浏览器
  |  GET /health、GET /api/node、POST /api/node/start|stop|restart
  v
manager   127.0.0.1:8080          services/manager
  |
  |  子进程，IPFS_PATH = 面板自己的仓库
  v
ipfs daemon
  API      127.0.0.1:5001         只有 manager 来叫
  网关     127.0.0.1:8081         这一片里没有人用
  swarm    Kubo 默认，通常是机器网卡上的 4001
```

manager 最多当一个 `ipfs daemon` 的父进程。启动时用参数数组，不经过 shell（不用 `sh -c`，也不用 `bash -c`）。daemon 的参数只有二进制路径和 `daemon`。manager 不会加上 `--unrestricted-api`。

manager 里由一个 goroutine 独占节点状态。HTTP 处理函数把命令交给它，再等一份快照。处理函数自己不启动 `ipfs`。连点两次也不会起两个 daemon。

manager 收到 SIGINT 或 SIGTERM 时，先停掉自己拥有的子进程，再退出。

## 路径

下面这些由 manager 读取。请求体不能设置它们。

| 变量 | 默认 | 作用 |
| --- | --- | --- |
| `MANAGER_ADDR` | `127.0.0.1:8080` | 已经实现。manager 的监听地址。 |
| `IPFS_PANEL_REPO` | `$XDG_DATA_HOME/ipfs-panel/kubo`；没设 `XDG_DATA_HOME` 时用 `~/.local/share/ipfs-panel/kubo` | 面板的 Kubo 仓库。 |
| `IPFS_PANEL_KUBO_BIN` | `PATH` 里找到的 `ipfs` | Kubo 二进制。 |

manager 把子进程的 `IPFS_PATH` 设成 `IPFS_PANEL_REPO`。就算 manager 自己的环境里已经有 `IPFS_PATH`，子进程仍然用面板仓库。有人在 shell 里 `export IPFS_PATH=~/.ipfs`，不应当因此让面板把那个节点接管走。

`~/.ipfs` 是人手动跑 Kubo 时得到的目录。面板不去动它。停掉一个不是我们启动的节点，不属于这一片。

`IPFS_PANEL_REPO` 不存在时，manager 创建它。创建不了或不能写，状态是 `failed`，代码 `repo_not_writable`。

这一片不负责安装 Kubo。找不到二进制就是 `failed`，代码 `binary_not_found`。

## 端口

| 端口 | 程序 | 这一片里 |
| --- | --- | --- |
| 8080 | manager | 保持。默认 `127.0.0.1:8080`。 |
| 5001 | Kubo API（控制） | 只在回环。调用方是 manager。 |
| 8081 | Kubo 网关（内容的 HTTP） | 只在回环。界面不链到它。把它放在这里，是为了别占 8080。 |
| 4001 | Kubo swarm（别的 IPFS 节点） | 保持 Kubo 默认。 |

只有当 `MANAGER_ADDR` 已经占用 8081 时，网关才改到 8082。网关不使用 manager 的端口。

Swarm 是节点加入 IPFS 网络的方式。默认的 Kubo 会在机器的网卡上监听。这和控制用的 API 是两件事。这一片不会为了“跟 swarm 一致”把 API 改到 `0.0.0.0`，也不会写文档教人转发 5001、8080 或 8081。

## manager 实际执行的命令

下面每条都是参数数组，并且 `IPFS_PATH` 指向面板仓库。时间限制是规格的一部分。

| 步骤 | 命令 | 时限 | 何时 |
| --- | --- | --- | --- |
| 创建仓库 | `ipfs init` | 60 秒 | 仓库里还没有 `config` |
| 写下私有地址 | `ipfs config` | 10 秒 | 仅紧接在这次 init 之后 |
| 检查地址 | `ipfs config --json Addresses` | 10 秒 | 每次启动 daemon 之前 |
| 跑节点 | `ipfs daemon` | 长期 | 检查通过之后 |
| 确认它起来了 | `ipfs id` | 每次 2 秒，总预算 30 秒 | `starting` 期间，以及 `up` 时每次刷新概览 |
| 请它退出 | `ipfs shutdown` | 5 秒 | 停止的第一步，且进程必须是我们的 |

`ipfs id` 打出的 JSON 里有 Peer ID、版本，还有节点的监听地址。manager 只留下 `ID` 和 `AgentVersion`。编 HTTP 响应之前丢掉地址和协议，避免监听地址偶然进到浏览器。

**第一次启动**，仓库里还没有 `config`：

1. 用 Kubo 的默认 profile 执行 `ipfs init`。`server`、`lowpower` 这类场景 profile 以后再说。
2. 把 `Addresses` 读成 JSON。API 写成回环上的 5001，JSON 类型跟 Kubo 刚生成的一致（一个字符串，或只有一个字符串的数组）。网关写成只含一个回环地址的数组，端口 8081；manager 若已占用 8081，则用 8082。
3. 再读一次 `Addresses`，然后做下面的检查。
4. 启动 `ipfs daemon`。

**每一次启动**，包括第一次：

- 每一个 API 地址、每一个网关地址都是回环（`127.0.0.1` 或 `::1`）。
- 它们都不用 manager 的端口。
- 做这个判断时不读 Swarm，也不写 Swarm。

检查失败就不启动 daemon。

manager 不删除一个创建到一半的仓库。下一次点「启动」会再试。没有单独的「初始化」按钮，也没有删除仓库的路由。

## manager 自己重启的时候

成功拉起子进程后，manager 把 pid 写到 `<仓库>/ipfs-panel.pid`，权限 `0600`。子进程退出后删掉这个文件。

manager 启动时：

- 在 Linux 上：这个 pid 还活着，并且 `/proc/<pid>/cmdline` 是配置好的 Kubo 二进制加上 `daemon`，并且 `/proc/<pid>/environ` 里的 `IPFS_PATH` 等于面板仓库，manager 就收养它。`ipfs id` 成功后状态变为 `up`。
- Kubo 的 `repo.lock` 被别的进程占着：状态是 `failed`，代码 `repo_busy`。manager 不杀掉那个进程，也不删锁。
- pid 已经死了：删掉 pid 文件，状态报 `down`。过期的锁由 Kubo 自己清。manager 不删 `repo.lock`。

收养是为了 manager 被 SIGKILL 之后，还能找回自己已经启动的 daemon。不是为了接上操作者在终端里自己跑起来的 Kubo。

## REST

概览的正文：

```json
{
  "state": "up",
  "peerId": "12D3KooW...",
  "version": "kubo/0.32.1/",
  "repoPath": "/home/me/.local/share/ipfs-panel/kubo",
  "error": null
}
```

`state` 是 `down`、`starting`、`up`、`stopping`、`failed` 之一。

只有 `state` 为 `up` 时，`peerId` 和 `version` 才有字符串，否则是 JSON 的 `null`。它们来自 `ipfs id`。

`repoPath` 是面板仓库，方便新手找到文件夹。它是文件系统路径，不是 URL。

`error` 是 `null`，或者：

```json
{ "code": "binary_not_found", "detail": "ipfs not found on PATH" }
```

`detail` 是 stderr 的末尾或一句很短的系统信息，最多 2000 字节。界面按 `code` 翻译。`detail` 不是那句给新手看的话。

正文里没有 API URL、没有网关 URL、没有 multiaddr、也没有端口字段。不该出现名叫 `api`、`gateway` 或 `multiaddr` 的字段。

| 方法与路径 | 作用 |
| --- | --- |
| `GET /health` | 不变。只说明 manager 进程在。不看 Kubo。 |
| `GET /api/node` | 当前概览。 |
| `POST /api/node/start` | 启动。若已经在启动，或已经是 `up`，则不改变。 |
| `POST /api/node/stop` | 停止我们拥有的子进程。若已经在停止，则不改变。 |
| `POST /api/node/restart` | 没有过渡进行中时：先停，再启动。 |

POST 的规则：

- `Content-Type` 必须是 `application/json`。没有这个头，或是别的类型，返回 **415**。空正文可以。`{}` 可以。JSON 里出现任何键，返回 **400**。浏览器不能顺便传路径、端口或参数。
- 成功是 **200** 加概览。新状态是 `starting` 或 `stopping` 时也一样。不用 202。处理函数在状态变成下一个值之后就返回，不等 daemon 真正起来，也不等停止做完。页面靠轮询 `GET /api/node` 知道后面的变化。
- 没有会话是 **401** `{"error":"unauthorized"}`。见[认证](design/auth.zh-CN.md)。`GET /health` 不需要会话。
- 未知路径仍是 **404**。方法不对仍是 **405**，和现在的 `/health` 一样。
- manager 不设置 `Access-Control-Allow-Origin`。

下面这张表保证连点是安全的：

| 当前状态 | 启动 | 停止 | 重启 |
| --- | --- | --- | --- |
| `down` | 开始启动 | 200，保持 `down` | 开始启动 |
| `starting` | 200，保持，不再起进程 | 200，保持，不排队 | 200，保持，不排队 |
| `up` | 200，保持 `up` | 开始停止 | 先停再启动 |
| `stopping` | 200，保持，不排队 | 200，保持 | 200，保持，不排队 |
| `failed`，进程已经没了 | 开始启动，一进入 `starting` 就清掉旧错误 | 200，保持 `failed`，句子还在 | 开始启动 |
| `failed`，`repo_busy` | 200，保持 `failed` | 200，保持 `failed` | 200，保持 `failed` |

停止时，只向我们启动或收养过的 pid 发信号。

成功进入 `up` 时，`error` 设为 `null`。

## 界面

句子、事实、管理动作和轮询，仍以本文为准。周围的界面在[设计系统](DESIGN-SYSTEM.zh-CN.md)：侧栏、主题、复制按钮，以及面板自己的 404 页。

外壳保留名字和标语，以及英文 / 中文开关。首页主区域是节点卡片。导航里真正能用的一项是「节点」。文件、固定、场景是占位，不调用 manager。设置里放的是这个浏览器的语言和主题。

版本、Peer ID、文件夹上可以有复制按钮。它们只复制页面上已经显示的文字，不调用 manager。

卡片按这个顺序：

1. 一句说明当前情况的话。
2. `up` 时：「连接」卡片显示版本和 Peer ID。宽屏时它在状态卡片旁边，窄屏时在下面。屏幕上的 Peer ID 是缩短的。复制按钮写下的、以及辅助技术读到的，是完整值。这张卡片不显示网关地址，也不显示 API 地址。概览里没有这两项。
3. 知道 `repoPath` 时，显示节点文件夹。
4. `error.detail` 非空时，在句子下面放一块预格式文本，标题是「详情」。
5. 下面表格里的管理动作。某一行事实上可以有复制按钮。不要别的管理动作。

页面上不出现端口、网关链接、Kubo RPC 链接，也没有填写路径的输入框。

| 情况 | 中文句子 | 按钮 |
| --- | --- | --- |
| `down`，没有错误 | 节点已停止。 | 启动 |
| `down`，`stop_timeout` | 节点已停止。它没有自己退出，面板强制把它停掉了。 | 启动 |
| `starting` | 节点正在启动。 | 无 |
| `up` | 节点正在运行。 | 停止、重启 |
| `stopping` | 节点正在停止。 | 无 |
| `failed`，见下面的代码 | 对应该代码的一句 | 启动；`repo_busy` 没有启动按钮；`binary_not_found` 显示的是重试，不是启动 |
| 请求 manager 失败 | 面板连不上这台机器上的管理服务。 | 重试 |

`repo_busy` 不放「启动」。文件夹已经被另一个 Kubo 占用，再点一次还是同样的失败。

`binary_not_found` 的主按钮是「重试」，旁边是次要链接「如何安装 Kubo」，指向 `https://docs.ipfs.tech/install/command-line/`（新标签页）。重试发出的是启动，这样刚装好 Kubo 的人可以再试一次。按钮不写「启动」：在有 `ipfs` 这条命令之前，启动不会成功。

句子里的一对反引号表示行内代码。面板把里面的词画成代码芯片。反引号本身不显示。`error.detail` 不按这个规则解析。

「重启」是「停止」旁边较轻的那个按钮。只有节点处于 `up` 时才出现。

文案写在 `apps/web/src/locales/en.ts` 和 `apps/web/src/locales/zh.ts`。服务器只给 `state` 和 `error.code`，不给句子。这一片的键：

| 键 | 中文 |
| --- | --- |
| `navNode` | 节点 |
| `nodeStopped` | 节点已停止。 |
| `nodeStopForced` | 节点已停止。它没有自己退出，面板强制把它停掉了。 |
| `nodeStarting` | 节点正在启动。 |
| `nodeRunning` | 节点正在运行。 |
| `nodeStopping` | 节点正在停止。 |
| `nodeUnreachable` | 面板连不上这台机器上的管理服务。 |
| `nodeStart` | 启动 |
| `nodeStop` | 停止 |
| `nodeRestart` | 重启 |
| `nodeRetry` | 重试 |
| `nodeVersion` | 版本 |
| `nodePeerId` | Peer ID |
| `nodeFolder` | 节点文件夹 |
| `nodeDetails` | 详情 |
| `nodeErrorBinary` | 没有装 Kubo，或面板找不到 `ipfs` 这条命令。 |
| `nodeErrorRepoWrite` | 面板写不了自己的节点文件夹。 |
| `nodeErrorInit` | 节点文件夹没能建好。 |
| `nodeErrorApiPublic` | 节点的控制端口不限于这台机器。面板不会启动它。 |
| `nodeErrorGatewayPublic` | 网关不限于这台机器。面板不会启动它。 |
| `nodeErrorPanelPort` | 节点被设成了占用面板的端口。面板不会启动它。 |
| `nodeErrorPortInUse` | 节点需要的端口已经被占用。 |
| `nodeErrorRepoBusy` | 已经有另一个 Kubo 在用这个文件夹。面板不会去停它。 |
| `nodeErrorDaemonExited` | Kubo 自己停了。 |
| `nodeErrorHealthTimeout` | 节点进程起来了，但一直没有就绪。面板已经把它停掉。 |
| `nodeErrorUnknown` | 启动或停止节点时出了问题。 |

`stop_timeout` 只用 `nodeStopForced`，而且只在 `state` 为 `down` 时用。其他代码在 `state` 为 `failed` 时用 `nodeError*` 这些键。

上一次概览是 `starting` 或 `stopping` 时，页面每 1 秒刷新。其余时候，只要 manager 有响应，每 5 秒刷新。请求失败就显示连不上管理服务的那句，并每 2 秒重试。请求失败不能把卡片改成「节点已停止。」保留上一份快照，在请求重新成功之前只换掉那一句说明。

开发服务器把 `/api` 和 `/health` 代理到 `http://127.0.0.1:8080`（实现时改 `apps/web/vite.config.ts`）。界面请求这些相对路径。如果改了 `MANAGER_ADDR`，同一次 pull request 里一起改代理目标。

## 失败情况

| 代码 | 何时 | manager 做什么 |
| --- | --- | --- |
| `binary_not_found` | `IPFS_PANEL_KUBO_BIN` 或 `PATH` 里的二进制执行不了。 | `failed`。没有进程。 |
| `repo_not_writable` | 面板仓库建不出来或不能写。 | `failed`。没有进程。 |
| `init_failed` | `ipfs init` 非零退出，或超过 60 秒。 | `failed`。目录留在原地。 |
| `api_not_loopback` | 有 API 地址不是回环。 | 不启动。`failed`。 |
| `gateway_not_loopback` | 有网关地址不是回环。 | 不启动。`failed`。 |
| `panel_port_conflict` | API 或网关用了 manager 的端口。 | 不启动。`failed`。 |
| `port_in_use` | daemon 退出，输出说明监听失败是因为地址已被占用。 | `failed`。 |
| `repo_busy` | 仓库锁被一个我们不拥有的进程拿着。 | `failed`。不向它发信号。 |
| `daemon_exited` | 子进程在 `ipfs id` 成功前退出，或在 `up` 期间退出，且没有更具体的代码。 | `failed`。 |
| `health_timeout` | 拉起 30 秒后子进程还在，但 `ipfs id` 从未成功。 | 杀掉子进程。`failed`。 |
| `stop_timeout` | 停止开始 10 秒后子进程还在，manager 发了 SIGKILL。 | `down`，并带上这个错误。节点已经停了。 |
| `unknown` | 其他情况。 | `failed`，`detail` 截断后附上。 |

我们拥有该进程时，停止的顺序：

1. 状态变为 `stopping`。
2. 用同一个 `IPFS_PATH` 执行 `ipfs shutdown`，限时 5 秒。
3. 进程还在，就向它的进程组发 SIGTERM。
4. 停止开始满 10 秒仍在，就向进程组发 SIGKILL，然后进入 `down`，错误为 `stop_timeout`。
5. 在那之前退出了，则是 `down`，`error` 为 `null`。

`health_timeout` 不能把 daemon 留在那里继续跑。

`up` 的时候进程自己崩了，下一次探测变成 `failed` / `daemon_exited`。每 5 秒刷新，就是为了卡片说实话，而不必先做日志页。

## 安全

manager 的默认监听地址仍是 `127.0.0.1:8080`。把 manager 公布出去仍然不是默认：`/api` 现在需要会话，而且 `MANAGER_ADDR` 仍然可以被指到公网网卡。除非你就是要这么做，否则不要那样设置，并且先读[认证](design/auth.zh-CN.md)。

Kubo API 留在回环。网关留在回环。任一检查失败，manager 就拒绝启动 daemon。只有在 `ipfs init` 刚刚创建仓库的那一刻，实现才去写这两个地址。之后若有人把它们改成对外，这是拒绝启动的理由，不是“先跑起来再说”的理由。

浏览器只跟 manager 说话。`ipfs id` 在 manager 里面跑。监听地址在编码概览之前丢掉。界面没有指向 5001 的链接，也不会去请求 5001。

同一台浏览器里打开的网站，可能在 manager 运行时去访问 `127.0.0.1:8080`。动作类路由要求 `Content-Type: application/json`，跨源调用会先发预检。manager 不用允许源站的头去应答预检，于是浏览器不会发出那个 POST。开发时的界面请求打到 Vite 的源站，再由 Vite 转到 manager，从而避开这个问题。不要为了让界面能用就加上 `Access-Control-Allow-Origin: *`。

子进程使用参数数组。仓库路径和二进制来自 manager 的环境，不来自请求。

留给 `detail` 的 stderr 放在内存里，并且有上限。这一片不写人人可读的日志文件，也不做日志页。

## 这一片不做的

留给后面的切片：

- 登录和会话。有登录之后，这些路由走同一套会话检查。本文的 JSON 保持不变。
- 添加、读取、列出文件。MFS。pin 和取消 pin。
- 安装或升级 Kubo 二进制。
- 场景 profile、修改 swarm、bootstrap、对等连接、原始配置编辑器。
- 日志页、仓库大小、连接上的节点数、带宽。
- 第二个节点，或另一台机器上的 Kubo。
- 删除仓库。
- 接上面板没有启动的 daemon，包括占用 `~/.ipfs` 的那个。
- 修改 `GET /health` 的正文。
- 公共网关、端口转发教程，或任何把浏览器带到 Kubo RPC 的画面。

## 验收清单

实现那次 pull request，下面每一条都成立才算做完。

- [x] `GET /health` 仍是 `{"status":"ok","service":"manager"}`，Content-Type 为 `application/json; charset=utf-8`。
- [x] 未设置 `MANAGER_ADDR` 时，manager 听在 `127.0.0.1:8080`。
- [x] 子进程的 `IPFS_PATH` 是面板仓库。manager 上已经存在的 `IPFS_PATH` 不能把子进程指走。
- [x] 对还没有 `config` 的仓库，第一次启动会跑 `ipfs init`，把 API 设为回环 5001，把网关设为回环 8081（仅当 manager 占用 8081 时用 8082），然后不带额外参数跑 `ipfs daemon`。
- [x] 状态已是 `up` 或 `starting` 时再启动，不会拉起第二个 daemon。
- [x] 任一 API 或网关地址不是回环，或其中之一占用了 manager 的端口，就拒绝启动。配置里的 Swarm 行保持原样。
- [x] 停止时只向 manager 启动或收养的子进程发信号。别人的锁是 `repo_busy`，那个进程之后仍在运行。
- [x] 从 `up` 重启，会经过停止再进入启动，最后停在 `up` 或 `failed`。
- [x] 拉起后 30 秒内 `ipfs id` 没有成功，子进程已经被杀掉，状态是 `failed`，代码 `health_timeout`。
- [x] `GET /api/node` 和三个 POST 返回概览，其中没有 API URL、网关 URL、multiaddr，也没有端口 5001。
- [x] POST 没有 `Content-Type: application/json` 时返回 415。响应里没有 `Access-Control-Allow-Origin`。
- [x] 首页卡片用英文和中文显示每种情况的那一句，并且只有上面列出的管理动作。版本、Peer ID、文件夹上的复制按钮可以有。页面不链到 Kubo。
- [x] 界面请求相对路径 `/api/node` 和 `/health`。Vite 开发服务器把两者代理到 manager。
- [x] `go test ./...` 用一个假的 `ipfs` 二进制覆盖监督逻辑：拒绝不安全地址，以及两次重叠的启动只产生一个子进程。
- [x] 不增加 manager 的文件路由、pin 路由或第二节点路由。不调用 manager 的占位页可以有。
- [x] 两个语言文件都包含每一个新键。

手工检查，不放进 CI：`PATH` 上有真的 `ipfs` 时，启动最后显示「节点正在运行。」和一个 Peer ID，停止最后显示「节点已停止。」两种语言都能看到这些句子。节点在跑时，对 `127.0.0.1:5001` 的 `curl` 可以成功。页面既不写这个地址，也不自己去请求它。

## 代码放哪

| 路径 | 职责 |
| --- | --- |
| `services/manager/internal/node` | 监督进程、状态、超时、pid 文件。 |
| `services/manager/internal/api` | `/api/node` 路由，放在现有健康检查旁边。 |
| `services/manager/cmd/manager/main.go` | 建好监督器。收到 SIGINT/SIGTERM 时先停子进程再退出。 |
| `apps/web/src/views/NodeView.vue` | 卡片。替换现在空的首页。 |
| `apps/web/src/stores/node.ts` | 轮询和最近一次快照。 |
| `apps/web/src/locales/en.ts`、`zh.ts` | 上表里的键。 |
| `apps/web/vite.config.ts` | 开发时代理 `/api` 和 `/health`。 |

测试写法跟 `services/manager/internal/api/health_test.go` 一样：用 `httptest`，不连真的网络。监督逻辑的测试在 `PATH` 上放一个假的二进制。
