# Node lifecycle

English below. 中文在后半部分。Same decisions, two languages.

**Status: implemented.** This file stays the spec. If the code and this file disagree, this file wins until a pull request changes it. The manager can start, stop, and restart one local Kubo. Login is still not part of this slice.

This slice is one local Kubo: start, stop, restart, and a health overview a beginner can read in one sentence. [Roadmap](ROADMAP.md) calls that slice 2. Login stays slice 1 and is not designed here.

## English

### Words

- **Kubo** is the IPFS program. The command is `ipfs`.
- The **manager** is our Go process in `services/manager`. The browser talks only to it.
- **Loopback** means `127.0.0.1` (or `::1`). Programs on this machine can connect. Other machines cannot.
- The **repo** is the folder where Kubo keeps its identity and data. The variable `IPFS_PATH` points at that folder.
- A **subprocess** is a program the manager starts and is responsible for. In this slice that program is `ipfs daemon`.

### What the screen is for

A beginner should be able to answer three questions without Kubo docs:

1. Is the node running?
2. How do I start it, stop it, or restart it?
3. If that failed, what do I do next?

The answer to (1) is one sentence, plus the Kubo version and the peer id when the node is up. The peer id is the node's identity number. The answer is not a dump of the Kubo RPC response.

### Decisions to review

These are the choices an implementation should follow. Object to them in the pull request that changes this file, before writing the code.

1. `GET /health` stays a check of the **manager process**. It already returns `{"status":"ok","service":"manager"}`. Node truth is a new route, `GET /api/node`.
2. The panel keeps its own repo. It does not use `~/.ipfs`.
3. The manager already listens on `127.0.0.1:8080`. Kubo's default gateway uses that same port. When the panel creates a repo, it sets the API to `127.0.0.1:5001` and the gateway to `127.0.0.1:8081`.
4. On later starts, the manager checks those addresses. If the API or the gateway is not loopback, it refuses to start. It does not rewrite the config again.
5. Swarm (Kubo's network port, usually 4001) stays at Kubo's default. A quiet node is a scene, which is a later slice.
6. The states are `down`, `starting`, `up`, `stopping`, and `failed`. Restart is a stop followed by a start. It is not a sixth state.
7. This slice does not add login. The guards are loopback, no CORS headers, and a JSON content type on the action routes.
8. The UI calls relative URLs (`/api/node`, `/health`). Vite proxies them to the manager in development. The page bundle does not contain a Kubo address.

### Who runs Kubo

```text
browser
  |  GET /health, GET /api/node, POST /api/node/start|stop|restart
  v
manager   127.0.0.1:8080          services/manager
  |
  |  subprocess, IPFS_PATH = the panel repo
  v
ipfs daemon
  API      127.0.0.1:5001         only the manager calls this
  gateway  127.0.0.1:8081         nobody in this slice
  swarm    Kubo default, often 4001 on the machine's network interfaces
```

The manager is the parent of at most one `ipfs daemon`. It starts that process with an argument list. It does not go through a shell (`sh -c` and `bash -c` are out). The child argv for the daemon is the binary, then `daemon`, and nothing else. In particular the manager never passes `--unrestricted-api`.

One goroutine inside the manager owns the node state. HTTP handlers send that goroutine a command and wait for a snapshot. They do not spawn `ipfs` themselves. Two clicks cannot create two daemons.

When the manager receives SIGINT or SIGTERM, it stops the child it owns, then exits.

### Paths

The manager reads these. A request body cannot set them.

| Variable | Default | Role |
| --- | --- | --- |
| `MANAGER_ADDR` | `127.0.0.1:8080` | Already implemented. The manager's listen address. |
| `IPFS_PANEL_REPO` | `$XDG_DATA_HOME/ipfs-panel/kubo`, or `~/.local/share/ipfs-panel/kubo` when `XDG_DATA_HOME` is unset | The panel's Kubo repo. |
| `IPFS_PANEL_KUBO_BIN` | The `ipfs` found on `PATH` | The Kubo binary. |

The manager sets the child's `IPFS_PATH` to `IPFS_PANEL_REPO`. If the manager's own environment already has `IPFS_PATH`, the child still gets the panel repo. An operator who exports `IPFS_PATH=~/.ipfs` in a shell must not make the panel take over that node.

`~/.ipfs` is the folder a person gets from running Kubo by hand. The panel leaves it alone. Stopping a node we did not start is out of this slice.

The manager creates `IPFS_PANEL_REPO` if it is missing. If the directory cannot be created or is not writable, the state is `failed` with code `repo_not_writable`.

This slice does not install Kubo. A missing binary is `failed` with code `binary_not_found`.

### Ports

| Port | Program | This slice |
| --- | --- | --- |
| 8080 | Manager | Stays. Default `127.0.0.1:8080`. |
| 5001 | Kubo API (control) | Loopback only. The manager is the client. |
| 8081 | Kubo gateway (content HTTP) | Loopback only. The UI does not link to it. It exists so the gateway is not on 8080. |
| 4001 | Kubo swarm (other IPFS nodes) | Left at Kubo's default. |

8081 moves to 8082 only when `MANAGER_ADDR` is already using port 8081. The gateway never uses the manager's port.

Swarm is how a node joins IPFS. On a default Kubo it listens on the machine's network interfaces. That is a different job from the control API. This slice does not move the API to `0.0.0.0` to match swarm, and it does not document a port forward for 5001, 8080, or 8081.

### What the manager runs

Every command below is an argument list with `IPFS_PATH` set to the panel repo. Time limits are part of the spec.

| Step | Command | Limit | When |
| --- | --- | --- | --- |
| Create the repo | `ipfs init` | 60s | `config` is not in the repo yet |
| Record private addresses | `ipfs config` | 10s | Only immediately after that init |
| Check addresses | `ipfs config --json Addresses` | 10s | Before every daemon start |
| Run the node | `ipfs daemon` | long-running | After the check passes |
| Prove it is up | `ipfs id` | 2s per try, 30s budget | While `starting`, and on each overview refresh while `up` |
| Ask it to exit | `ipfs shutdown` | 5s | First step of stop, when we own the process |

`ipfs id` prints JSON that includes the peer id, the version, and the node's listen addresses. The manager keeps `ID` and `AgentVersion`. It discards addresses and protocols before it builds the HTTP body, so a listen address cannot leak into the browser by accident.

**First start**, when the repo has no `config`:

1. `ipfs init` with Kubo's default profile. Scene profiles such as `server` and `lowpower` wait.
2. Read `Addresses` as JSON. Write the API as loopback port 5001, keeping the JSON type Kubo used (a string, or a list of one string). Write the gateway as a list of one loopback address on 8081, or 8082 if the manager is on 8081.
3. Read `Addresses` again. Then apply the check below.
4. Start `ipfs daemon`.

**Every start**, including the first:

- Every API address and every gateway address is loopback (`127.0.0.1` or `::1`).
- None of them uses the manager's port.
- Swarm is not read for this decision and is not written.

If the check fails, the daemon is not started.

The manager does not delete a half-created repo. The next Start tries again. There is no separate Init button, and there is no route that deletes the repo.

### If the manager restarts

The manager writes `<repo>/ipfs-panel.pid` after it spawns the child, mode `0600`, and removes that file after the child has exited.

On boot:

- Linux: if that pid is alive, and `/proc/<pid>/cmdline` is the configured Kubo binary with `daemon`, and `/proc/<pid>/environ` has `IPFS_PATH` equal to the panel repo, the manager adopts it. State becomes `up` once `ipfs id` succeeds.
- If Kubo's `repo.lock` is held by some other process, the state is `failed` with `repo_busy`. The manager does not kill that process and does not delete the lock.
- If the pid is dead, the manager removes the pid file and reports `down`. Kubo itself clears a stale lock. The manager does not remove `repo.lock`.

Adopting is how a manager that was killed with SIGKILL finds the daemon it already started. It is not a way to attach to a Kubo the operator started in a terminal.

### REST

The overview body:

```json
{
  "state": "up",
  "peerId": "12D3KooW...",
  "version": "kubo/0.32.1/",
  "repoPath": "/home/me/.local/share/ipfs-panel/kubo",
  "error": null
}
```

`state` is one of `down`, `starting`, `up`, `stopping`, `failed`.

`peerId` and `version` are JSON `null` unless `state` is `up`. They come from `ipfs id`.

`repoPath` is the panel repo, so a beginner can find the folder. It is a filesystem path, not a URL.

`error` is `null` or:

```json
{ "code": "binary_not_found", "detail": "ipfs not found on PATH" }
```

`detail` is a tail of stderr or a short OS message, capped at 2000 bytes. The UI translates `code`. It does not treat `detail` as the sentence.

The body has no API URL, no gateway URL, no multiaddr, and no port field. A field named like `api`, `gateway`, or `multiaddr` does not belong here.

| Method and path | Effect |
| --- | --- |
| `GET /health` | Unchanged. Manager process is up. Does not look at Kubo. |
| `GET /api/node` | Current overview. |
| `POST /api/node/start` | Start, or no change if a start is already underway or the node is `up`. |
| `POST /api/node/stop` | Stop the child we own, or no change if we are already stopping. |
| `POST /api/node/restart` | Stop, then start, when no transition is underway. |

POST rules:

- `Content-Type` is `application/json`. A missing or other type is **415**. An empty body is valid. `{}` is valid. Any JSON key is **400**. The browser cannot pass a path, a port, or a flag.
- Success is **200** and the overview, including when the new state is `starting` or `stopping`. There is no 202. The handler returns once the state has moved to that next value. It does not wait until the daemon is up, or until a stop has finished. The page learns the rest by polling `GET /api/node`.
- There is no 401. Login is not this slice.
- Unknown paths stay **404**. The wrong method stays **405**, the same as today's `/health` behavior.
- The manager does not set `Access-Control-Allow-Origin`.

Single-flight, so a double click is safe:

| Current state | Start | Stop | Restart |
| --- | --- | --- | --- |
| `down` | begin start | 200, stay `down` | begin start |
| `starting` | 200, stay, do not spawn again | 200, stay, do not queue | 200, stay, do not queue |
| `up` | 200, stay `up` | begin stop | begin stop, then start |
| `stopping` | 200, stay, do not queue | 200, stay | 200, stay, do not queue |
| `failed`, process gone | begin start, clear the old error once `starting` | 200, stay `failed` so the sentence remains | begin start |
| `failed`, `repo_busy` | 200, stay `failed` | 200, stay `failed` | 200, stay `failed` |

Stop never signals a pid we did not spawn or adopt.

A successful arrival at `up` sets `error` to `null`.

### UI

The shell stays: brand, tagline, English / 中文 switch. The home route's main area becomes the node card. The nav's only item is "Node" / "节点".

The card shows, in order:

1. One sentence for the situation.
2. When `up`: a "Version" line and a "Peer ID" line. The peer id is the full value, as text the person can select.
3. The node folder, when `repoPath` is known.
4. When `error.detail` is non-empty, a preformatted block under the sentence, labeled "Details".
5. The buttons in the table below, and no other controls.

The page does not show ports, a gateway link, a Kubo RPC link, or a text field for a path.

| Situation | English sentence | Buttons |
| --- | --- | --- |
| `down`, no error | The node is stopped. | Start |
| `down`, `stop_timeout` | The node is stopped. It did not exit on its own, so the panel forced it to stop. | Start |
| `starting` | The node is starting. | none |
| `up` | The node is running. | Stop, Restart |
| `stopping` | The node is stopping. | none |
| `failed`, see codes below | one sentence for that code | Start, except `repo_busy` which has no Start |
| manager request failed | The panel cannot reach the manager on this machine. | Retry |

`repo_busy` has no Start button because another Kubo already holds the folder. Start would fail the same way again.

Restart is the quieter button next to Stop. It is on screen only while the node is `up`.

Copy lives in `apps/web/src/locales/en.ts` and `apps/web/src/locales/zh.ts`. The server sends `state` and `error.code`. It does not send a sentence. Keys for this slice:

| Key | English |
| --- | --- |
| `navNode` | Node |
| `nodeStopped` | The node is stopped. |
| `nodeStopForced` | The node is stopped. It did not exit on its own, so the panel forced it to stop. |
| `nodeStarting` | The node is starting. |
| `nodeRunning` | The node is running. |
| `nodeStopping` | The node is stopping. |
| `nodeUnreachable` | The panel cannot reach the manager on this machine. |
| `nodeStart` | Start |
| `nodeStop` | Stop |
| `nodeRestart` | Restart |
| `nodeRetry` | Retry |
| `nodeVersion` | Version |
| `nodePeerId` | Peer ID |
| `nodeFolder` | Node folder |
| `nodeDetails` | Details |
| `nodeErrorBinary` | Kubo is not installed, or the panel cannot find the `ipfs` command. |
| `nodeErrorRepoWrite` | The panel cannot write its node folder. |
| `nodeErrorInit` | The node folder could not be created. |
| `nodeErrorApiPublic` | The node control port is not limited to this machine. The panel will not start it. |
| `nodeErrorGatewayPublic` | The gateway is not limited to this machine. The panel will not start it. |
| `nodeErrorPanelPort` | The node is set to use the panel's port. The panel will not start it. |
| `nodeErrorPortInUse` | A port the node needs is already in use. |
| `nodeErrorRepoBusy` | Another Kubo is already using this folder. The panel will not stop that one. |
| `nodeErrorDaemonExited` | Kubo stopped on its own. |
| `nodeErrorHealthTimeout` | The node process started, but it never became ready. The panel stopped it. |
| `nodeErrorUnknown` | Something went wrong starting or stopping the node. |

`stop_timeout` uses `nodeStopForced`, and only while `state` is `down`. The other codes use the `nodeError*` keys while `state` is `failed`.

While the last overview said `starting` or `stopping`, the page refreshes every second. Otherwise, if the manager is answering, it refreshes every 5 seconds. If the request fails, the card shows the unreachable sentence and retries every 2 seconds. A failed request must not flip the card to "The node is stopped." Keep the last snapshot, and replace the sentence until a request succeeds.

The dev server proxies `/api` and `/health` to `http://127.0.0.1:8080` (see `apps/web/vite.config.ts` when this is built). The UI fetches those relative paths. If `MANAGER_ADDR` changes, the proxy target changes in the same pull request.

### Failure modes

| Code | When | What the manager does |
| --- | --- | --- |
| `binary_not_found` | The binary from `IPFS_PANEL_KUBO_BIN` or `PATH` cannot be executed. | `failed`. No process. |
| `repo_not_writable` | The panel repo cannot be created or written. | `failed`. No process. |
| `init_failed` | `ipfs init` exits non-zero, or exceeds 60s. | `failed`. Leave the directory in place. |
| `api_not_loopback` | Any API address is not loopback. | Do not start. `failed`. |
| `gateway_not_loopback` | Any gateway address is not loopback. | Do not start. `failed`. |
| `panel_port_conflict` | The API or the gateway uses the manager's port. | Do not start. `failed`. |
| `port_in_use` | The daemon exits and its output says the listen failed because the address is taken. | `failed`. |
| `repo_busy` | The repo lock is held by a process we do not own. | `failed`. Do not signal it. |
| `daemon_exited` | The child exits before `ipfs id` succeeds, or it exits while `up`, and no more specific code fits. | `failed`. |
| `health_timeout` | The child is alive 30s after spawn and `ipfs id` has not succeeded. | Kill the child. `failed`. |
| `stop_timeout` | The child is still alive 10s after stop began, so the manager sent SIGKILL. | `down`, with this error set. The node is stopped. |
| `unknown` | Anything else. | `failed`, with a capped `detail`. |

Stop sequence, when we own the process:

1. State becomes `stopping`.
2. `ipfs shutdown` with the same `IPFS_PATH`, 5s limit.
3. If the process is still alive, SIGTERM to its process group.
4. 10s after stop began, if it is still alive, SIGKILL to the process group, then `down` with `stop_timeout`.
5. If it exits before that, `down` and `error` is `null`.

`health_timeout` must not leave the daemon running.

A crash while `up` becomes `failed` / `daemon_exited` on the next probe. The 5s refresh is what makes the card tell the truth without a log page.

### Security

The manager's default listen address stays `127.0.0.1:8080`. Publishing the manager is still a mistake: this slice does not add login, and `MANAGER_ADDR` can still be pointed at a public interface. Do not do that.

The Kubo API stays on loopback. The gateway stays on loopback. The manager refuses to start the daemon when either check fails. The implementation writes those addresses only in the moment after `ipfs init` created the repo. A later edit that opens them up is a reason to refuse, not a reason to start and hope.

The browser talks to the manager. `ipfs id` runs inside the manager. Its listen addresses are dropped before the overview is encoded. The UI has no link and no fetch aimed at port 5001.

A website open in the same browser can try to call `127.0.0.1:8080` while the manager is running. Action routes require `Content-Type: application/json`, which makes a cross-origin call send a preflight. The manager does not answer that preflight with an allow-origin header, so the browser does not send the POST. The dev UI avoids the problem by calling the Vite origin, which proxies to the manager. Do not add `Access-Control-Allow-Origin: *` to make the UI work.

The subprocess is an argument vector. The repo path and the binary come from the manager's environment, not from the request.

Stderr kept for `detail` stays in memory, capped. This slice does not write a world-readable log file and does not add a log page.

### Out of scope

Later slices, not this one:

- Login and sessions. When login exists, these routes take the same session check. The JSON in this document stays.
- Adding, reading, and listing files. MFS. Pins and unpin.
- Installing or upgrading the Kubo binary.
- Scene profiles, swarm edits, bootstrap, peering, a raw config editor.
- A log page, repo size, peer count, bandwidth.
- A second node, or a Kubo on another machine.
- Deleting the repo.
- Attaching to a daemon the panel did not start, including a daemon on `~/.ipfs`.
- Changing the `GET /health` body.
- A public gateway, a port-forward guide, or any screen that sends the browser to Kubo RPC.

### Acceptance criteria

The implementation pull request is done when every box is true.

- [x] `GET /health` is still `{"status":"ok","service":"manager"}` with content type `application/json; charset=utf-8`.
- [x] With `MANAGER_ADDR` unset, the manager listens on `127.0.0.1:8080`.
- [x] The child `IPFS_PATH` is the panel repo. An `IPFS_PATH` already set on the manager does not redirect the child.
- [x] The first start on a repo with no `config` runs `ipfs init`, sets the API to loopback port 5001, sets the gateway to loopback port 8081 (8082 only if the manager is using 8081), then runs `ipfs daemon` with no extra flags.
- [x] A start while `up` or `starting` does not spawn a second daemon.
- [x] Start is refused when any API or gateway address is not loopback, or when either uses the manager's port. Swarm lines in the config are unchanged.
- [x] Stop signals only a child the manager spawned or adopted. A foreign lock is `repo_busy`, and that process is still running afterward.
- [x] Restart from `up` goes through stopping and then starting, and ends at `up` or `failed`.
- [x] If `ipfs id` does not succeed within 30s of spawn, the child is dead and the state is `failed` with `health_timeout`.
- [x] `GET /api/node` and the three POST routes return the overview and do not include an API URL, a gateway URL, a multiaddr, or port 5001.
- [x] A POST without `Content-Type: application/json` returns 415. Responses do not include `Access-Control-Allow-Origin`.
- [x] The home card shows the sentence for each situation in English and in 中文, and only the buttons listed above. The page does not link to Kubo.
- [x] The UI fetches relative `/api/node` and `/health`. The Vite dev server proxies both to the manager.
- [x] `go test ./...` covers the supervisor with a fake `ipfs` binary: the unsafe-address refusal, and two overlapping starts producing one child.
- [x] No file route, pin route, or second-node route or page is added.
- [x] Both locale files contain every new key.

Manual check, not required in CI: with a real `ipfs` on `PATH`, Start ends on "The node is running." and a peer id, Stop ends on "The node is stopped.", and both languages show those sentences. `curl` to `127.0.0.1:5001` can succeed while the node is up. The page does not document or perform that curl.

### Where the code lives

| Path | Responsibility |
| --- | --- |
| `services/manager/internal/node` | Supervisor, states, timeouts, pid file. |
| `services/manager/internal/api` | `/api/node` routes beside the current health handler. |
| `services/manager/cmd/manager/main.go` | Build the supervisor. On SIGINT/SIGTERM, stop the child, then exit. |
| `apps/web/src/views/NodeView.vue` | The card. Replaces the empty home view. |
| `apps/web/src/stores/node.ts` | Polling and the last snapshot. |
| `apps/web/src/locales/en.ts`, `zh.ts` | The keys in the table above. |
| `apps/web/vite.config.ts` | Dev proxy for `/api` and `/health`. |

Tests follow the style of `services/manager/internal/api/health_test.go`: `httptest`, no real network. The supervisor tests use a fake binary on `PATH`.

---

## 中文

**状态：已实现。** 本文仍然是规格。以后代码和本文不一致时，以本文为准，直到有 pull request 改这份说明。manager 可以启动、停止和重启本机上的一个 Kubo。登录仍然不属于这一片。

这一片只做本机上的一个 Kubo：启动、停止、重启，以及新手能用一句话读懂的健康概览。[路线图](ROADMAP.md)里这是第 2 片。登录仍是第 1 片，本文不设计登录。

### 用词

- **Kubo** 是 IPFS 的程序。命令叫 `ipfs`。
- **manager**（管理服务）是 `services/manager` 里的 Go 进程。浏览器只和它说话。
- **回环** 指 `127.0.0.1`（或 `::1`）。这台机器上的程序可以连。别的机器不行。
- **仓库（repo）** 是 Kubo 存放身份和数据的文件夹。变量 `IPFS_PATH` 指向它。
- **子进程** 是 manager 启动、并且由 manager 负责的程序。这一片里就是 `ipfs daemon`。

### 这个画面要回答什么

新手不用翻 Kubo 文档，也能回答三件事：

1. 节点在不在跑？
2. 怎么启动、停止、重启？
3. 失败了，下一步是什么？

第 1 问的答案是一句话。节点在跑时，再加上 Kubo 版本和 Peer ID。Peer ID 是这个节点的身份号码。答案不是把 Kubo RPC 的响应倒在屏幕上。

### 请在实现之前确认的决定

实现按这些做。要改，先在改这份文档的 pull request 里提出来，再写代码。

1. `GET /health` 仍然只表示 **manager 这个进程** 活着。它现在返回 `{"status":"ok","service":"manager"}`。节点到底怎么样，看新的 `GET /api/node`。
2. 面板用自己的仓库目录，不用 `~/.ipfs`。
3. manager 已经听在 `127.0.0.1:8080`。Kubo 默认的网关也用这个端口。面板创建仓库时，把 API 设为 `127.0.0.1:5001`，把网关设为 `127.0.0.1:8081`。
4. 以后每次启动只检查这两个地址。API 或网关不是回环，就拒绝启动，不再改写配置。
5. Swarm（Kubo 连别的节点的端口，通常是 4001）保持 Kubo 的默认。安静地跑是场景，属于后面的切片。
6. 状态只有 `down`、`starting`、`up`、`stopping`、`failed`。重启就是先停再启动，不是第六个状态。
7. 这一片不加登录。防护是：只听回环、不发 CORS 头、动作类请求必须带 JSON 的 Content-Type。
8. 界面请求相对路径（`/api/node`、`/health`）。开发时由 Vite 代理到 manager。打包后的页面里不出现 Kubo 的地址。

### 谁来跑 Kubo

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

### 路径

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

### 端口

| 端口 | 程序 | 这一片里 |
| --- | --- | --- |
| 8080 | manager | 保持。默认 `127.0.0.1:8080`。 |
| 5001 | Kubo API（控制） | 只在回环。调用方是 manager。 |
| 8081 | Kubo 网关（内容的 HTTP） | 只在回环。界面不链到它。把它放在这里，是为了别占 8080。 |
| 4001 | Kubo swarm（别的 IPFS 节点） | 保持 Kubo 默认。 |

只有当 `MANAGER_ADDR` 已经占用 8081 时，网关才改到 8082。网关不使用 manager 的端口。

Swarm 是节点加入 IPFS 网络的方式。默认的 Kubo 会在机器的网卡上监听。这和控制用的 API 是两件事。这一片不会为了“跟 swarm 一致”把 API 改到 `0.0.0.0`，也不会写文档教人转发 5001、8080 或 8081。

### manager 实际执行的命令

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

### manager 自己重启的时候

成功拉起子进程后，manager 把 pid 写到 `<仓库>/ipfs-panel.pid`，权限 `0600`。子进程退出后删掉这个文件。

manager 启动时：

- 在 Linux 上：这个 pid 还活着，并且 `/proc/<pid>/cmdline` 是配置好的 Kubo 二进制加上 `daemon`，并且 `/proc/<pid>/environ` 里的 `IPFS_PATH` 等于面板仓库，manager 就收养它。`ipfs id` 成功后状态变为 `up`。
- Kubo 的 `repo.lock` 被别的进程占着：状态是 `failed`，代码 `repo_busy`。manager 不杀掉那个进程，也不删锁。
- pid 已经死了：删掉 pid 文件，状态报 `down`。过期的锁由 Kubo 自己清。manager 不删 `repo.lock`。

收养是为了 manager 被 SIGKILL 之后，还能找回自己已经启动的 daemon。不是为了接上操作者在终端里自己跑起来的 Kubo。

### REST

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
- 没有 401。登录不是这一片。
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

### 界面

外壳保留：名字、标语、英文 / 中文开关。首页主区域变成节点卡片。导航里只有一项：「节点」。

卡片按这个顺序：

1. 一句说明当前情况的话。
2. `up` 时：一行「版本」，一行「Peer ID」。Peer ID 用完整字符串，让人可以选中复制。
3. 知道 `repoPath` 时，显示节点文件夹。
4. `error.detail` 非空时，在句子下面放一块预格式文本，标题是「详情」。
5. 下面表格里的按钮。不要别的控件。

页面上不出现端口、网关链接、Kubo RPC 链接，也没有填写路径的输入框。

| 情况 | 中文句子 | 按钮 |
| --- | --- | --- |
| `down`，没有错误 | 节点已停止。 | 启动 |
| `down`，`stop_timeout` | 节点已停止。它没有自己退出，面板强制把它停掉了。 | 启动 |
| `starting` | 节点正在启动。 | 无 |
| `up` | 节点正在运行。 | 停止、重启 |
| `stopping` | 节点正在停止。 | 无 |
| `failed`，见下面的代码 | 对应该代码的一句 | 启动；`repo_busy` 没有启动按钮 |
| 请求 manager 失败 | 面板连不上这台机器上的管理服务。 | 重试 |

`repo_busy` 不放「启动」。文件夹已经被另一个 Kubo 占用，再点一次还是同样的失败。

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

### 失败情况

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

### 安全

manager 的默认监听地址仍是 `127.0.0.1:8080`。把 manager 公布出去仍然是错的：这一片没有登录，而且 `MANAGER_ADDR` 仍然可以被指到公网网卡。不要那样设置。

Kubo API 留在回环。网关留在回环。任一检查失败，manager 就拒绝启动 daemon。只有在 `ipfs init` 刚刚创建仓库的那一刻，实现才去写这两个地址。之后若有人把它们改成对外，这是拒绝启动的理由，不是“先跑起来再说”的理由。

浏览器只跟 manager 说话。`ipfs id` 在 manager 里面跑。监听地址在编码概览之前丢掉。界面没有指向 5001 的链接，也不会去请求 5001。

同一台浏览器里打开的网站，可能在 manager 运行时去访问 `127.0.0.1:8080`。动作类路由要求 `Content-Type: application/json`，跨源调用会先发预检。manager 不用允许源站的头去应答预检，于是浏览器不会发出那个 POST。开发时的界面请求打到 Vite 的源站，再由 Vite 转到 manager，从而避开这个问题。不要为了让界面能用就加上 `Access-Control-Allow-Origin: *`。

子进程使用参数数组。仓库路径和二进制来自 manager 的环境，不来自请求。

留给 `detail` 的 stderr 放在内存里，并且有上限。这一片不写人人可读的日志文件，也不做日志页。

### 这一片不做的

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

### 验收清单

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
- [x] 首页卡片用英文和中文显示每种情况的那一句，并且只有上面列出的按钮。页面不链到 Kubo。
- [x] 界面请求相对路径 `/api/node` 和 `/health`。Vite 开发服务器把两者代理到 manager。
- [x] `go test ./...` 用一个假的 `ipfs` 二进制覆盖监督逻辑：拒绝不安全地址，以及两次重叠的启动只产生一个子进程。
- [x] 不增加文件路由、pin 路由、第二节点的路由或页面。
- [x] 两个语言文件都包含每一个新键。

手工检查，不放进 CI：`PATH` 上有真的 `ipfs` 时，启动最后显示「节点正在运行。」和一个 Peer ID，停止最后显示「节点已停止。」两种语言都能看到这些句子。节点在跑时，对 `127.0.0.1:5001` 的 `curl` 可以成功。页面既不写这个地址，也不自己去请求它。

### 代码放哪

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
