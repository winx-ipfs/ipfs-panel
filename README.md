# ipfs-panel

[![CI](https://github.com/winx-ipfs/ipfs-panel/actions/workflows/ci.yml/badge.svg)](https://github.com/winx-ipfs/ipfs-panel/actions/workflows/ci.yml)

English below. 中文在后半部分。

IPFS is worth using. The usual way to run Kubo is not. You get a CLI, a long JSON config, and a local RPC port that must stay private. A lot of people give up before the node feels real.

This repo is a panel for Kubo. Open it on the machine that runs the node, or open it in a browser from somewhere else after you log in. Beginners get a few clear actions. Experts still get the deep settings later, on purpose. This is not a small host panel. No websites, no databases, no "manage the whole server". Kubo is the job.

**winx.run** is the portal. **This repository is the product.**

The browser never talks to Kubo RPC (usually `127.0.0.1:5001`). That port stays on the machine. Our own manager is the API. It checks who you are before it starts or stops Kubo. It binds to loopback unless you override it. Do not publish it until you have read [Auth](docs/design/auth.md).

## Layout

```text
apps/web            the panel UI
services/manager    the API gateway (Go)
docs                why we are building this, and in what order
```

| Path | What lives there |
| --- | --- |
| `apps/web` | Vue 3, Vite, TypeScript, Vue Router, Pinia, vue-i18n. English and 中文 from the first screen. |
| `services/manager` | Go, standard library. The only process that should call Kubo. |
| `docs/VISION.md` | What this is, and what we will not turn it into. |
| `docs/ROADMAP.md` | Vertical slices. Finish one before starting the next. |

## Run the panel

The manager starts and stops one local Kubo. The browser talks to the manager. Kubo (`ipfs`) needs to be on `PATH`, or set `IPFS_PANEL_KUBO_BIN` to the binary. The panel keeps its own repo. The default is `$XDG_DATA_HOME/ipfs-panel/kubo`, or `~/.local/share/ipfs-panel/kubo` when `XDG_DATA_HOME` is unset. It does not use `~/.ipfs`.

Manager:

```bash
cd services/manager
go run ./cmd/manager
curl -s http://127.0.0.1:8080/health
```

You should see `{"status":"ok","service":"manager"}`. That check is the manager process, not Kubo. With `MANAGER_ADDR` unset, the manager listens on `127.0.0.1:8080`. Optional flags: `-addr`, `-repo`, `-data`, `-kubo-bin`, and `-trust-proxy`. Each one overrides the matching environment variable when you pass it.

The first time you open the panel, it asks you to create the admin account. The password is hashed with argon2id and stored in `<data>/auth.json` (mode `0600`), next to the Kubo repo, not inside it. The default data directory is `$XDG_DATA_HOME/ipfs-panel`, or `~/.local/share/ipfs-panel`. After that, the panel asks you to sign in. A request to `/api` without a session is `401`. `GET /health` stays open.

If you forget the password, stop the manager and run `go run ./cmd/manager reset-admin`. It deletes the account. It does not take a password on the command line. Start the manager again and create the account. From another machine, setup also needs the one-time token the new process prints. The reason, and the rest of the rules, are in [Auth](docs/design/auth.md).

UI, in another terminal:

```bash
cd apps/web
npm install
npm run dev
```

Open the printed local URL. The Node card starts, stops, and restarts the node. The dev server proxies `/api` and `/health` to `http://127.0.0.1:8080`.

Checks CI runs for the UI:

```bash
cd apps/web
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```

Manager checks:

```bash
cd services/manager
gofmt -l .
go vet ./...
go test -race ./...
go build -o /dev/null ./cmd/manager
```

Node 22 is enough for the UI (22.18 or newer, or Node 24.12+). The manager module needs Go 1.26, which is what `services/manager/go.mod` asks for. CI installs that version.

## Look

The shell, the tokens, and the rules for new screens are in [Design system](docs/DESIGN-SYSTEM.md).

![Node running, light theme, English](docs/screenshots/node-running-light-en.png)

![Node running, dark theme, 中文](docs/screenshots/node-running-dark-zh.png)

![Node stopped, light theme, 中文, narrow](docs/screenshots/node-stopped-light-zh-mobile.png)

## Read next

- [Vision](docs/VISION.md)
- [Roadmap](docs/ROADMAP.md)
- [Design system](docs/DESIGN-SYSTEM.md)
- [Auth](docs/design/auth.md)
- [Node lifecycle](docs/DESIGN-node-lifecycle.md)
- [Contributing](CONTRIBUTING.md) (Git Flow, CI, hotfixes)
- [Org setup](docs/ORG-SETUP.md) (repository settings checklist)
- [Governance](GOVERNANCE.md)
- [Security](SECURITY.md)
- [Support](SUPPORT.md)
- [Code of conduct](CODE_OF_CONDUCT.md)
- License: [MIT](LICENSE)

---

# ipfs-panel（中文）

IPFS 值得用。Kubo 默认的用法不太值得折腾。你拿到的是命令行、一份很长的 JSON，还有一个必须留在本机的 RPC 端口。很多人在「节点到底算不算跑起来了」之前就放弃了。

这个仓库是 Kubo 的面板。可以开在跑节点的那台机器上，也可以在别的设备的浏览器里打开，但远程要先登录。新手看到的是几件说得清的事。熟手要的深层设置会晚一点出现，而且是故意晚一点。这不是一个小号的主机面板。不管网站，不管数据库，也不管整台服务器。我们只管 Kubo。

**winx.run** 是门户。**这个仓库才是产品。**

浏览器不和 Kubo RPC 说话（一般是 `127.0.0.1:5001`）。那个端口留在机器上。对外的是我们自己的 manager。它会先确认你是谁，再去启动或停止 Kubo。默认只听本机回环地址。在读过[认证](docs/design/auth.md)之前，别把它暴露出去。

## 目录

```text
apps/web            面板界面
services/manager    API 网关（Go）
docs                为什么做、按什么顺序做
```

| 路径 | 里面是什么 |
| --- | --- |
| `apps/web` | Vue 3、Vite、TypeScript、Vue Router、Pinia、vue-i18n。第一屏就有英文和中文。 |
| `services/manager` | Go，标准库。以后只有这个进程应该去叫 Kubo。 |
| `docs/VISION.md` | 这是什么，以及我们不会把它做成什么。 |
| `docs/ROADMAP.md` | 垂直切片。做完一片再开始下一片。 |

## 把面板跑起来

manager 负责启动和停止本机上的一个 Kubo。浏览器只跟 manager 说话。Kubo（`ipfs`）要在 `PATH` 上，或者用 `IPFS_PANEL_KUBO_BIN` 指向那个二进制。面板用自己的仓库。默认是 `$XDG_DATA_HOME/ipfs-panel/kubo`；没设 `XDG_DATA_HOME` 时是 `~/.local/share/ipfs-panel/kubo`。它不会去用 `~/.ipfs`。

管理服务：

```bash
cd services/manager
go run ./cmd/manager
curl -s http://127.0.0.1:8080/health
```

你应该看到 `{"status":"ok","service":"manager"}`。这个检查只说明 manager 这个进程在，不说明 Kubo 在不在。没设 `MANAGER_ADDR` 时，manager 听在 `127.0.0.1:8080`。可选参数：`-addr`、`-repo`、`-data`、`-kubo-bin`、`-trust-proxy`。传了参数就覆盖对应的环境变量。

第一次打开面板时，它会让你创建管理员账号。密码用 argon2id 哈希，存在 `<data>/auth.json`（权限 `0600`），和 Kubo 仓库并列，不在仓库里面。默认数据目录是 `$XDG_DATA_HOME/ipfs-panel`，没设 `XDG_DATA_HOME` 时是 `~/.local/share/ipfs-panel`。之后面板会要求登录。没有会话的 `/api` 请求是 `401`。`GET /health` 仍然开放。

如果忘了密码，先停掉 manager，再运行 `go run ./cmd/manager reset-admin`。它删掉账号。它不在命令行上接受密码。然后再启动 manager，重新创建账号。从另一台机器设置时，还需要新进程打印的一次性口令。原因和其余规则在[认证](docs/design/auth.md)。

界面，另开一个终端：

```bash
cd apps/web
npm install
npm run dev
```

打开它打印出来的本机地址。节点卡片可以启动、停止和重启节点。开发服务器把 `/api` 和 `/health` 代理到 `http://127.0.0.1:8080`。

和 CI 一样的界面检查：

```bash
cd apps/web
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```

管理服务的检查：

```bash
cd services/manager
gofmt -l .
go vet ./...
go test -race ./...
go build -o /dev/null ./cmd/manager
```

界面用 Node 22 就行（22.18 或更新，或者 Node 24.12+）。管理服务模块需要 Go 1.26，`services/manager/go.mod` 里写的就是这个版本。CI 会装它。

## 看一眼

外壳、令牌，以及新画面要遵守的规矩，在[设计系统](docs/DESIGN-SYSTEM.md)。

![节点正在运行，浅色，英文](docs/screenshots/node-running-light-en.png)

![节点正在运行，深色，中文](docs/screenshots/node-running-dark-zh.png)

![节点已停止，浅色，中文，窄屏](docs/screenshots/node-stopped-light-zh-mobile.png)

## 接着读

- [愿景](docs/VISION.md)
- [路线图](docs/ROADMAP.md)
- [设计系统](docs/DESIGN-SYSTEM.md)
- [认证](docs/design/auth.md)
- [节点启停](docs/DESIGN-node-lifecycle.md)
- [参与方式](CONTRIBUTING.md)（Git Flow、CI、热修）
- [组织设置](docs/ORG-SETUP.md)（仓库设置清单）
- [治理](GOVERNANCE.md)
- [安全](SECURITY.md)
- [支持](SUPPORT.md)
- [行为准则](CODE_OF_CONDUCT.md)
- 许可证：[MIT](LICENSE)
