# ipfs-panel

English below. 中文在后半部分。

IPFS is worth using. The usual way to run Kubo is not. You get a CLI, a long JSON config, and a local RPC port that must stay private. A lot of people give up before the node feels real.

This repo is a panel for Kubo. Open it on the machine that runs the node, or open it in a browser from somewhere else after you log in. Beginners get a few clear actions. Experts still get the deep settings later, on purpose. This is not a small host panel. No websites, no databases, no "manage the whole server". Kubo is the job.

**winx.run** is the portal. **This repository is the product.**

The browser never talks to Kubo RPC (usually `127.0.0.1:5001`). That port stays on the machine. Our own manager is the API. It will check who you are before it does anything real. Today it only answers a health check, and it binds to loopback unless you override it. Do not publish it. There is no auth yet.

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

## Run the empty shell

UI:

```bash
cd apps/web
npm install
npm run dev
```

Manager:

```bash
cd services/manager
go run ./cmd/manager
curl -s http://127.0.0.1:8080/health
```

You should see `{"status":"ok","service":"manager"}`. This does not start Kubo. The page does not call the manager yet either. Connecting them is a later slice.

Checks CI runs for the UI:

```bash
cd apps/web
npm run typecheck
npm run build
```

Manager tests:

```bash
cd services/manager
go test ./...
```

Node 22 is enough for the UI (22.18 or newer, or Node 24.12+). Go 1.22 is enough for the manager.

## Read next

- [Vision](docs/VISION.md)
- [Roadmap](docs/ROADMAP.md)
- [Contributing](CONTRIBUTING.md)
- License: [MIT](LICENSE)

---

# ipfs-panel（中文）

IPFS 值得用。Kubo 默认的用法不太值得折腾。你拿到的是命令行、一份很长的 JSON，还有一个必须留在本机的 RPC 端口。很多人在「节点到底算不算跑起来了」之前就放弃了。

这个仓库是 Kubo 的面板。可以开在跑节点的那台机器上，也可以在别的设备的浏览器里打开，但远程要先登录。新手看到的是几件说得清的事。熟手要的深层设置会晚一点出现，而且是故意晚一点。这不是一个小号的主机面板。不管网站，不管数据库，也不管整台服务器。我们只管 Kubo。

**winx.run** 是门户。**这个仓库才是产品。**

浏览器不和 Kubo RPC 说话（一般是 `127.0.0.1:5001`）。那个端口留在机器上。对外的是我们自己的 manager。它以后会先确认你是谁，再去做真正的事。现在它只会回答健康检查，而且默认只听本机回环地址。先别把它暴露出去，认证还没做。

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

## 把空壳跑起来

界面：

```bash
cd apps/web
npm install
npm run dev
```

管理服务：

```bash
cd services/manager
go run ./cmd/manager
curl -s http://127.0.0.1:8080/health
```

你应该看到 `{"status":"ok","service":"manager"}`。这不会启动 Kubo。页面现在也不会去请求 manager。把两边接上是后面的切片。

和 CI 一样的界面检查：

```bash
cd apps/web
npm run typecheck
npm run build
```

管理服务的测试：

```bash
cd services/manager
go test ./...
```

界面用 Node 22 就行（22.18 或更新，或者 Node 24.12+）。管理服务用 Go 1.22 就行。

## 接着读

- [愿景](docs/VISION.md)
- [路线图](docs/ROADMAP.md)
- [参与方式](CONTRIBUTING.md)
- 许可证：[MIT](LICENSE)
