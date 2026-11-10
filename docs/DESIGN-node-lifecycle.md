# Node lifecycle

**English** · [中文](DESIGN-node-lifecycle.zh-CN.md)

**Status: implemented.** This file stays the spec. If the code and this file disagree, this file wins until a pull request changes it. The manager can start, stop, and restart one local Kubo. Those routes now require a session. Login is specified in [Auth](design/auth.md).

This slice is one local Kubo: start, stop, restart, and a health overview a beginner can read in one sentence. [Roadmap](ROADMAP.md) calls that slice 2. Login stays slice 1 and is not designed here.

## Words

- **Kubo** is the IPFS program. The command is `ipfs`.
- The **manager** is our Go process in `services/manager`. The browser talks only to it.
- **Loopback** means `127.0.0.1` (or `::1`). Programs on this machine can connect. Other machines cannot.
- The **repo** is the folder where Kubo keeps its identity and data. The variable `IPFS_PATH` points at that folder.
- A **subprocess** is a program the manager starts and is responsible for. In this slice that program is `ipfs daemon`.

## What the screen is for

A beginner should be able to answer three questions without Kubo docs:

1. Is the node running?
2. How do I start it, stop it, or restart it?
3. If that failed, what do I do next?

The answer to (1) is one sentence, plus the Kubo version and the peer id when the node is up. The peer id is the node's identity number. The answer is not a dump of the Kubo RPC response.

## Decisions to review

These are the choices an implementation should follow. Object to them in the pull request that changes this file, before writing the code.

1. `GET /health` stays a check of the **manager process**. It already returns `{"status":"ok","service":"manager"}`. Node truth is a new route, `GET /api/node`.
2. The panel keeps its own repo. It does not use `~/.ipfs`.
3. The manager already listens on `127.0.0.1:8080`. Kubo's default gateway uses that same port. When the panel creates a repo, it sets the API to `127.0.0.1:5001` and the gateway to `127.0.0.1:8081`.
4. On later starts, the manager checks those addresses. The API must be loopback. The gateway must be loopback, except the read-only public gateway in [Scenes](design/scenes.md): the unspecified listeners on the panel gateway port, and only when `Gateway.NoFetch` and `Gateway.NoDNSLink` are true and `Gateway.ExposeRoutingAPI` is false. Any other public address is refused. The manager does not rewrite the addresses on its own.
5. Swarm (Kubo's network port, usually 4001) stays at Kubo's default. A quiet node is a scene, which is a later slice.
6. The states are `down`, `starting`, `up`, `stopping`, and `failed`. Restart is a stop followed by a start. It is not a sixth state.
7. Login is a separate spec. These routes require a session. The other guards stay: no CORS headers, and a JSON content type on the action routes. A missing session is **401** `{"error":"unauthorized"}`. `GET /health` stays open.
8. The UI calls relative URLs (`/api/node`, `/health`). Vite proxies them to the manager in development. The page bundle does not contain a Kubo address.

## Who runs Kubo

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

## Paths

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

## Ports

| Port | Program | This slice |
| --- | --- | --- |
| 8080 | Manager | Stays. Default `127.0.0.1:8080`. |
| 5001 | Kubo API (control) | Loopback only. The manager is the client. |
| 8081 | Kubo gateway (content HTTP) | Loopback only. The UI does not link to it. It exists so the gateway is not on 8080. |
| 4001 | Kubo swarm (other IPFS nodes) | Left at Kubo's default. |

8081 moves to 8082 only when `MANAGER_ADDR` is already using port 8081. The gateway never uses the manager's port.

Swarm is how a node joins IPFS. On a default Kubo it listens on the machine's network interfaces. That is a different job from the control API. This slice does not move the API to `0.0.0.0` to match swarm, and it does not document a port forward for 5001, 8080, or 8081.

## What the manager runs

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

The readiness probe does not call `ipfs id` until the repo's `api` file exists and is non-empty. Kubo writes that file when the HTTP API is listening, and removes it on shutdown. Before the file exists, `ipfs id` opens the repo and takes `repo.lock`, and the daemon exits because the lock is held. The node is still not `up` until `ipfs id` itself succeeds.

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

## If the manager restarts

The manager writes `<repo>/ipfs-panel.pid` after it spawns the child, mode `0600`, and removes that file after the child has exited.

On boot:

- Linux: if that pid is alive, and `/proc/<pid>/cmdline` is the configured Kubo binary with `daemon`, and `/proc/<pid>/environ` has `IPFS_PATH` equal to the panel repo, the manager adopts it. State becomes `up` once `ipfs id` succeeds.
- If Kubo's `repo.lock` is held by some other process, the state is `failed` with `repo_busy`. The manager does not kill that process and does not delete the lock.
- If the pid is dead, the manager removes the pid file and reports `down`. Kubo itself clears a stale lock. The manager does not remove `repo.lock`.

Adopting is how a manager that was killed with SIGKILL finds the daemon it already started. It is not a way to attach to a Kubo the operator started in a terminal.

## REST

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
- A missing session is **401** `{"error":"unauthorized"}`. See [Auth](design/auth.md). `GET /health` does not require a session.
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

## UI

The sentences, the facts, the manager actions, and the polling in this file stay the contract. The chrome around them is in [Design system](DESIGN-SYSTEM.md): sidebar, theme, copy buttons, and the panel's own 404 page.

The shell shows the brand and the tagline, and an English / 中文 switch. The home route's main area is the node card. The nav's working item is Node. Files, Pins, and Scenes are placeholders that do not call the manager. Settings holds the language and the theme for this browser.

Copy buttons may sit on the version, the peer id, and the folder. They copy text the page already shows. They do not call the manager.

The card shows, in order:

1. One sentence for the situation.
2. When `up`: a Connection card shows Version and Peer ID. On a wide screen it sits beside the status card. On a narrow screen it sits under it. The peer id is shortened on screen. The full value is what the copy button writes, and what assistive tech reads. When `activity` is present, the card also shows the peer count and, when the rates are present, traffic in and out. Both are hidden while the node is not `up`. The card does not show a gateway address or an API address. The overview does not include them. `activity` is omitted when the node is not `up`, or when neither call answered. A missing rate means bandwidth was not available (`Swarm.DisableBandwidthMetrics`, or the reporter error in `core/commands/stat.go:104-106`). The peer count is the number of distinct peer ids from `swarm/peers` (`core/commands/swarm.go:234`, `core/commands/swarm.go:447`). Addresses on those rows (`core/commands/swarm.go:433`) are discarded. The count links to the Peers page. The number itself is unchanged. Rates are `RateIn` and `RateOut` from `stats/bw` (`core/commands/stat.go:46-50`, `core/commands/stat.go:158`).
3. The node folder, when `repoPath` is known.
4. When `error.detail` is non-empty, a preformatted block under the sentence, labeled "Details".
5. The manager actions in the table below. A copy button may sit on a fact. No other manager actions.

The page does not show ports, a gateway link, a Kubo RPC link, or a text field for a path.

| Situation | English sentence | Buttons |
| --- | --- | --- |
| `down`, no error | The node is stopped. | Start |
| `down`, `stop_timeout` | The node is stopped. It did not exit on its own, so the panel forced it to stop. | Start |
| `starting` | The node is starting. | none |
| `up` | The node is running. | Stop, Restart |
| `stopping` | The node is stopping. | none |
| `failed`, see codes below | one sentence for that code | Start, except `repo_busy` which has no Start, and `binary_not_found` which shows Retry instead of Start |
| manager request failed | The panel cannot reach the manager on this machine. | Retry |

`repo_busy` has no Start button because another Kubo already holds the folder. Start would fail the same way again.

`binary_not_found` shows Retry as the primary button, and a secondary link, "How to install Kubo", to `https://docs.ipfs.tech/install/command-line/` (a new tab). Retry posts start, so a person who just installed Kubo can try again. Start is not the label: starting cannot succeed until the `ipfs` command exists.

A backtick pair in a sentence marks inline code. The panel renders the inside as a code chip. The backtick characters are not shown. `error.detail` is never parsed this way.

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

## Failure modes

| Code | When | What the manager does |
| --- | --- | --- |
| `binary_not_found` | The binary from `IPFS_PANEL_KUBO_BIN` or `PATH` cannot be executed. | `failed`. No process. |
| `repo_not_writable` | The panel repo cannot be created or written. | `failed`. No process. |
| `init_failed` | `ipfs init` exits non-zero, or exceeds 60s. | `failed`. Leave the directory in place. |
| `api_not_loopback` | Any API address is not loopback. | Do not start. `failed`. |
| `gateway_not_loopback` | A gateway address is not loopback and is not the read-only public shape in [Scenes](design/scenes.md). | Do not start. `failed`. |
| `gateway_not_readonly` | The gateway uses the public shape, but it is not limited to content already on the node. | Do not start. `failed`. |
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

## Security

The manager's default listen address stays `127.0.0.1:8080`. Publishing the manager is still a decision, not the default: `/api` now requires a session, and `MANAGER_ADDR` can still be pointed at a public interface. Do not do that unless you mean to, and read [Auth](design/auth.md) first.

The Kubo API stays on loopback. The gateway stays on loopback. The manager refuses to start the daemon when either check fails. The implementation writes those addresses only in the moment after `ipfs init` created the repo. A later edit that opens them up is a reason to refuse, not a reason to start and hope.

The browser talks to the manager. `ipfs id` runs inside the manager. Its listen addresses are dropped before the overview is encoded. The UI has no link and no fetch aimed at port 5001.

A website open in the same browser can try to call `127.0.0.1:8080` while the manager is running. Action routes require `Content-Type: application/json`, which makes a cross-origin call send a preflight. The manager does not answer that preflight with an allow-origin header, so the browser does not send the POST. The dev UI avoids the problem by calling the Vite origin, which proxies to the manager. Do not add `Access-Control-Allow-Origin: *` to make the UI work.

The subprocess is an argument vector. The repo path and the binary come from the manager's environment, not from the request.

Stderr kept for `detail` stays in memory, capped. This slice does not write a world-readable log file and does not add a log page.

## Out of scope

Later slices, not this one:

- Login and sessions. When login exists, these routes take the same session check. The JSON in this document stays.
- Adding, reading, and listing files. MFS. Pins and unpin.
- Installing or upgrading the Kubo binary.
- Swarm listen edits, bootstrap, peering, a raw config editor. Scenes are specified in [Scenes](design/scenes.md).
- A log page, repo size. Peer count and bandwidth rates are on the connection card, above.
- A second node, or a Kubo on another machine.
- Deleting the repo.
- Attaching to a daemon the panel did not start, including a daemon on `~/.ipfs`.
- Changing the `GET /health` body.
- A public gateway, a port-forward guide, or any screen that sends the browser to Kubo RPC.

## Acceptance criteria

The implementation pull request is done when every box is true.

- [x] `GET /health` is still `{"status":"ok","service":"manager"}` with content type `application/json; charset=utf-8`.
- [x] With `MANAGER_ADDR` unset, the manager listens on `127.0.0.1:8080`.
- [x] The child `IPFS_PATH` is the panel repo. An `IPFS_PATH` already set on the manager does not redirect the child.
- [x] The first start on a repo with no `config` runs `ipfs init`, sets the API to loopback port 5001, sets the gateway to loopback port 8081 (8082 only if the manager is using 8081), then runs `ipfs daemon` with no extra flags.
- [x] A start while `up` or `starting` does not spawn a second daemon.
- [x] Start is refused when any API address is not loopback, when the gateway is public but not the read-only shape in [Scenes](design/scenes.md), or when either uses the manager's port. Swarm lines in the config are unchanged.
- [x] Stop signals only a child the manager spawned or adopted. A foreign lock is `repo_busy`, and that process is still running afterward.
- [x] Restart from `up` goes through stopping and then starting, and ends at `up` or `failed`.
- [x] If `ipfs id` does not succeed within 30s of spawn, the child is dead and the state is `failed` with `health_timeout`.
- [x] `GET /api/node` and the three POST routes return the overview and do not include an API URL, a gateway URL, a multiaddr, or port 5001.
- [x] A POST without `Content-Type: application/json` returns 415. Responses do not include `Access-Control-Allow-Origin`.
- [x] The home card shows the sentence for each situation in English and in 中文, and only the manager actions listed above. Copy buttons on the version, the peer id, and the folder are allowed. The page does not link to Kubo.
- [x] The UI fetches relative `/api/node` and `/health`. The Vite dev server proxies both to the manager.
- [x] `go test ./...` covers the supervisor with a fake `ipfs` binary: the unsafe-address refusal, and two overlapping starts producing one child.
- [x] No manager file route, pin route, or second-node route is added. Placeholder pages that do not call the manager are allowed.
- [x] Both locale files contain every new key.

Manual check, not required in CI: with a real `ipfs` on `PATH`, Start ends on "The node is running." and a peer id, Stop ends on "The node is stopped.", and both languages show those sentences. `curl` to `127.0.0.1:5001` can succeed while the node is up. The page does not document or perform that curl.

## Where the code lives

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
