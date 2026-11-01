# Roadmap

English below. 中文在后半部分。

The UI stack is Vue 3, Vite, TypeScript, Vue Router, Pinia, and vue-i18n, in `apps/web`. The API is Go in `services/manager`, standard library for now. Kubo is not vendored here. The manager will call a local Kubo later. It will not publish Kubo's RPC.

## English

### How a slice works

Each slice is vertical. The screen, the manager route, and the Kubo call (when that slice has one) land together. A slice is done when a person can do that job in English and in 中文 without reading Kubo docs for it.

We do not build the whole backend, then the whole UI.

### Where this repo is

The tree you can clone today is the foundation, not slice 1.

What is here:

- an empty panel shell
- English and 中文, with the language switch saved in the browser
- `GET /health` on the manager, loopback by default

What is not here:

- auth
- starting or stopping Kubo
- files, pins, or scenes
- more than one node

### 1. Shell, auth, and languages

The shell and the language switch already exist. This slice adds a real login on the manager, and a UI that stays signed out until that login works.

An anonymous browser cannot call the manager. The browser still never receives a Kubo RPC address to call itself.

Done when you can sign in, the shell renders in both languages, and a request without a session is rejected.

### 2. Start, stop, and health

Show whether Kubo is up. Start it. Stop it. Restart it. The status is a sentence (up, down, starting, stopping, failed), not a dump of the RPC response.

The spec is [Node lifecycle](DESIGN-node-lifecycle.md). That file is design only. The behavior is not in the code yet.

Done when you can change whether the local node is running from the panel, and the panel tells the truth when it fails.

### 3. Files: add, pin, and MFS

Add a file, see the CID, find it again in the mutable file system (MFS), and pin it as part of that flow.

Done when a beginner can put a file on the node and open it again from the panel.

### 4. Pins

A page for what the node is holding, and a way to unpin. Pin bookkeeping is not hidden inside the file browser.

Done when you can answer "what is pinned?" and remove one pin without the CLI.

### 5. Scene settings

Pick a scene. The panel writes the Kubo config for that scene and says, in one sentence per language, what it changed.

Done when a common setup does not mean opening the JSON file.

### 6. Expert mode, then more than one node

Raw config, logs, swarm and peering. After that, more than one Kubo behind the same panel.

This waits until slices 1–5 feel dull. Dull is the goal.

### Not in the early slices

- Host-panel work: sites, databases, containers, OS admin
- A public Kubo RPC, or a tunnel to it
- Rewriting Kubo
- A hosted public gateway as a product
- Multi-node before one node works

---

## 中文

界面在 `apps/web`：Vue 3、Vite、TypeScript、Vue Router、Pinia、vue-i18n。API 在 `services/manager`，现在用 Go 标准库。仓库里不带 Kubo。以后由 manager 去叫本机上的 Kubo，不会把 Kubo 的 RPC 公布出去。

### 一片是怎么算完的

每一片都是竖着切的。画面、manager 的路由、以及这一片要打的 Kubo 调用（如果有），一起落地。一个人用英文和中文都能把这件事做完，而且不用为这件事再去翻 Kubo 文档，这片才算完。

不要先把整个后端做完，再回头做整个界面。

### 这个仓库现在在哪

现在能克隆到的是地基，不是第 1 片。

已经有的：

- 一个空的面板壳
- 英文和中文，语言开关会记在浏览器里
- manager 上的 `GET /health`，默认只听本机

还没有的：

- 登录
- 启动或停止 Kubo
- 文件、pin、场景
- 多个节点

### 1. 壳、登录、语言

壳和语言开关已经在了。这一片要补的是 manager 上真正的登录，以及没登录成功之前界面都保持未登录。

没会话的浏览器不能调用 manager。浏览器仍然拿不到一个可以自己去叫的 Kubo RPC 地址。

做完的样子：能登录，壳用两种语言都能画出来，没有会话的请求会被拒绝。

### 2. 启动、停止、健康

看出 Kubo 在不在。启动它。停止它。重启它。状态是一句话（在跑、停了、正在启动、正在停止、失败了），不是把 RPC 响应倒出来。

规格在 [节点启停](DESIGN-node-lifecycle.md)。那份文件只有设计，行为还没进代码。

做完的样子：你可以在面板上改变本机节点是否在跑，失败时面板说的是实话。

### 3. 文件：添加、固定、MFS

添加一个文件，看到 CID，在可变文件系统（MFS）里再找到它，并在这条流程里把它 pin 上。

做完的样子：新手可以把一个文件放到节点上，再从面板里打开它。

### 4. 固定列表（pins）

单独一页看节点拿着什么，以及怎么取消 pin。pin 的账不要藏在文件浏览器里面。

做完的样子：你能回答「pin 了什么」，并且不用命令行就能去掉一个。

### 5. 场景配置

选一个场景。面板把这个场景对应的 Kubo 配置写进去，并用每种语言的一句话说明改了什么。

做完的样子：常见设置不用去打开那份 JSON。

### 6. 专家模式，然后才是多个节点

原始配置、日志、swarm 和对等连接。在那之后，同一个面板后面可以有不止一个 Kubo。

这一片要等 1 到 5 变得没劲。没劲就是目标。

### 早期切片里不做的

- 主机面板的活：网站、数据库、容器、操作系统管理
- 公开的 Kubo RPC，或者一条通向它的隧道
- 重写 Kubo
- 把托管公共网关当成产品来做
- 单节点还没做好就做多节点
