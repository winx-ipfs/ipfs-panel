# Auth

English below. 中文在后半部分。Same decisions, two languages.

**Status: implemented.** This file is the spec for the first real login. If the code and this file disagree, this file wins until a pull request changes it. This is roadmap slice 1 (the shell and the languages were already here). Node start and stop stay as in [Node lifecycle](../DESIGN-node-lifecycle.md), and now require a session.

## English

### What this is for

The manager can start and stop Kubo. Later slices let a browser do that from another machine. That only stays safe if a stranger who can open the port cannot. This slice adds one admin, a session, and a setup that a stranger cannot finish.

The browser still never talks to Kubo. It talks to the manager. `GET /health` stays a check of the manager process and stays open, so a process monitor can use it. Every other `/api` route needs a session.

### Threat model

What we protect:

- The admin password, and the file it is stored in.
- The session that stands in for that password.
- The actions behind `/api`, including start, stop, and restart of the local Kubo.
- The first-run moment, when there is no admin yet and whoever finishes setup becomes the admin.

Who we assume is trying:

- Another person, or another process, that can reach the manager's port. The default bind is still `127.0.0.1`. A later remote slice, or a mistaken `-addr`, changes that.
- A website the operator already has open. It will try to submit the panel's forms (CSRF) or to guess the password.
- Someone who can read a backup of the data directory, but not the manager's memory while it is running.
- Someone who can read the manager's stdout once. The setup token is printed there on purpose. After it is used, or after the process exits, that line is not a key anymore.

What a same-user shell already can do: read the data directory, stop the process, and run `reset-admin`. We do not pretend the session protects the operator from themselves. The escape hatch is that shell.

What this does not try to stop: a bug that lets the page run an attacker's script (XSS). The session cookie is `HttpOnly` so a script cannot read it. A script that can run on the page can still call `/api` from the page. Keeping untrusted HTML out of the panel is a separate rule.

### Decisions

1. **One admin.** There are no accounts, no roles, and no invitation. The first successful setup creates the only user. A second setup is **409** `already_setup`.

2. **Passwords are hashed with argon2id** from `golang.org/x/crypto` (the current patched release; older tags fail dependency review). The stored string is PHC format (`$argon2id$v=19$m=19456,t=2,p=1$...`). Parameters for new hashes: memory 19 MiB (`m=19456`), time 2, parallelism 1, 32-byte tag, 16-byte salt. That is the OWASP interactive setting. Verify uses the parameters inside the string, and refuses a string that asks for more than 256 MiB, more than 10 passes, or more than 4 lanes, so a edited file cannot make the process allocate a huge region. A missing user still runs a verify against a dummy hash created at startup with the same parameters, so the response time does not answer "is there such a user?".

3. **The hash lives in the manager data directory, never in the Kubo repo.** The file is `<data>/auth.json`, mode `0600`. The directory is mode `0700`. Writes go to a temp file in the same directory, then `fsync`, then rename. The default data directory is `$XDG_DATA_HOME/ipfs-panel`, or `~/.local/share/ipfs-panel`. The Kubo repo stays `<data>/kubo`. Flags: `-data` / `IPFS_PANEL_DATA`. If that file would be inside the Kubo repo, the manager refuses to start. The repo is given to the `ipfs` child; the password hash must not go with it.

4. **Setup is allowed from loopback, or with a one-time token.** Loopback means the TCP peer is `127.0.0.1` or `::1`. The manager reads that from the connection, not from `X-Forwarded-For`. A client who can reach the port can send any forwarded-for header they like, and a proxy can be told to send `127.0.0.1`. Trusting that header would let a remote client skip the token. The token exists for the other case: the operator published the manager on purpose and is sitting in front of its log, not on the machine's loopback. The manager prints one token to stdout and to the log when it starts and no admin exists. It keeps only a SHA-256 of the token. The token is not written to disk. Setup from anywhere that is not loopback must send it (JSON field `setupToken`). A wrong token does not burn it. A successful setup clears it. A restart before setup prints a new token and the old one dies. Loopback setup does not need the token. The UI asks for the token only when `setupTokenRequired` is true, which is "no admin yet, and this TCP peer is not loopback".

5. **The session is an opaque random value stored on the server.** The cookie `ipfs_panel_session` holds 32 random bytes, encoded. The server stores SHA-256 of that value, the username, a CSRF secret, and two times: created, and last seen. The cookie is `HttpOnly`, `SameSite=Strict`, path `/`. `Secure` is set when the connection is TLS, or when `-trust-proxy` / `IPFS_PANEL_TRUST_PROXY` is on and `X-Forwarded-Proto` is `https`. It is not set just because a client sent that header. Idle lifetime is 12 hours. Absolute lifetime is 7 days. Both are enforced when the cookie is presented. The browser `Max-Age` matches the absolute lifetime. A restart drops every session; they are not written to disk. Twelve hours covers a workday away from the keyboard. Seven days means a stolen cookie cannot be renewed forever. Both numbers live in one place (`IdleTimeout`, `AbsoluteTimeout`) so a later remote slice can shorten them without a redesign.

6. **CSRF uses a secret the HTML form cannot invent.** `GET /api/auth/setup-status` sets a cookie `ipfs_panel_csrf` (`SameSite=Strict`, not `HttpOnly`, so the page can read it) and returns the same value as `csrfToken`. Login and setup must send that value in the `X-CSRF-Token` header. A cross-site form cannot set that header. After login, the session has its own CSRF secret. The server compares the header to that secret with a constant-time compare. Safe methods (`GET`, `HEAD`, `OPTIONS`) do not need it. If the `Origin` header is present on a state-changing request, its host must match the request `Host`. Browsers send `Origin` on `POST`. A non-browser client may omit it. The dev proxy leaves `Host` as the page host, so the two match.

7. **Login is rate limited per IP and per username.** The first four failures have no delay. The next failure waits 1 second, then 2, 4, and so on, capped at 15 minutes. The client gets **429** `rate_limited` and `Retry-After`. The clock does not run inside the handler, so a flood does not pile up sleeping goroutines. A success clears both buckets. The IP is the connection peer, unless `-trust-proxy` is set, in which case it is the rightmost `X-Forwarded-For` entry (the one a single proxy appends). The username bucket is the attempted name, lowercased, so `Ada` and `ada` share a lock. The password is not a key.

8. **The failure message is generic.** Wrong username, wrong password, and login before setup all return **401** `{"error":"invalid_credentials"}`. The body does not say which part was wrong. Rate limiting is a separate status because the operator needs to wait, not to guess. Setup and password rules return their own **400** codes (`invalid_username`, `invalid_password`, `password_unchanged`). Those do not reveal the stored password.

9. **Comparisons of secrets are constant-time.** Password tags, session CSRF secrets, the setup token, and the pre-session CSRF cookie use `crypto/subtle`. Usernames are padded to a fixed width and compared the same way. The session lookup is a map keyed by the hash of a 256-bit value; guessing a key is not the threat.

10. **A forgotten password is cleared from the machine, not emailed.** `manager reset-admin` deletes `<data>/auth.json` and prints what to do next. It does not accept a password. A password on the command line shows up in the process list and in shell history. Stop the manager first. The running process keeps the old hash in memory until it exits, and it keeps the sessions. Start it again. There is no admin, so it prints a new setup token, and the UI shows setup. From loopback the token is optional. From anywhere else it is required.

11. **Audit lines are structured slog, and they are not the setup-token line.** Each of login, logout, setup, and password change logs `msg=audit` with `event`, `result`, `username`, and `ip`. Results include `success`, `failure`, `rate_limited`, `forbidden`, `invalid`, and `exists`. The line never includes the password, the hash, the session cookie, the CSRF secret, or the setup token. The setup token is printed once, on its own, at startup. That line is the delivery mechanism for decision 4. It is not an audit event. Do not ship that stream to a log host that other people can read, unless you also treat it as a secret store.

12. **Password rules.** At least 12 Unicode characters, at most 128, no NUL. A string of the same character repeated, and a short denylist (`password1234`, `123456789012`, and a few more of the same kind), are rejected. A phrase of ordinary words is accepted. The UI meter is a hint. The server is the rule. The meter may say "weak" for a 12-character password that the server still accepts.

13. **Public routes.** These do not need a session:

    | Method and path | Why it is open |
    | --- | --- |
    | `GET /health` | Process check. Not under `/api`. |
    | `GET /api/auth/setup-status` | The page has to know whether to show setup or login, and has to receive a CSRF token. |
    | `POST /api/auth/setup` | Creates the admin. Gated by loopback or the setup token, plus CSRF. |
    | `POST /api/auth/login` | Creates a session. Gated by the password, the rate limit, and CSRF. |

    Every other `/api` path without a session is **401** `{"error":"unauthorized"}`, including paths that do not exist. A valid session on an unknown path is still **404**. State-changing routes without the CSRF header are **403** `{"error":"csrf"}`. The UI sends the browser to `/login` and keeps the path it wanted in `redirect`. After a good login it goes back, but only to a same-site path (starts with `/`, does not start with `//`, and is not the login or setup page).

### API

`Content-Type` on the posts is `application/json`. Anything else is **415**. Unknown JSON fields are **400**. Bodies are limited to 8 KiB.

`GET /api/auth/setup-status`

```json
{ "setupRequired": true, "setupTokenRequired": false, "csrfToken": "..." }
```

`POST /api/auth/setup` with `{ "username", "password", "setupToken?" }`. Success is **201** and `{ "username", "csrfToken" }`, plus the session cookie. The browser is signed in.

`POST /api/auth/login` with `{ "username", "password" }`. Success is **200** and the same shape.

`GET /api/auth/session` needs a session. **200** `{ "username", "csrfToken" }`. This is how a refresh learns the CSRF secret again.

`POST /api/auth/logout` needs a session and the CSRF header. **204**. Cookies are cleared.

`POST /api/auth/password` with `{ "currentPassword", "newPassword" }` needs a session and the CSRF header. Success is **200** `{ "username", "csrfToken" }`. The current session stays; its CSRF secret is rotated. Other sessions for that admin are dropped. The current password is checked with the same generic **401** and the same rate limit as login.

Username: 1 to 64 characters, ASCII letters, digits, `.`, `_`, `-`, starting with a letter or digit. Case-sensitive at login. The rate-limit bucket is not.

Cookie names: `ipfs_panel_session`, `ipfs_panel_csrf`.

### UI

No admin: a setup screen, not the shell. Username, password, confirm, a strength meter, and the setup token field only when the status says it is required. Submit creates the account and opens the panel.

Admin, no session: a login screen. A failed login says the username or password is not right. It does not say which. The shell is not shown.

A session: the existing shell. Settings gains an Account tab with change-password. Log out sits at the bottom of the sidebar. A **401** from any panel request returns the browser to login and remembers the path.

Copy is in `en` and `zh`. The screens follow [Design system](../DESIGN-SYSTEM.md): the WinX mark, one primary button, tokens instead of raw colors, and a calm page rather than a bare form.

### Out of scope

- More than one user, roles, OIDC, SSO, WebAuthn, TOTP.
- Sessions that survive a manager restart.
- Timeouts that an operator can set from the page.
- A network "forgot password" flow. The escape hatch is `reset-admin` on the machine.
- Changing the username.
- Turning on a public bind. Auth makes a published manager less open. It does not make publishing it the default, and it does not put Kubo RPC on a public address.
- Locking the account forever. The backoff cap is 15 minutes.
- A global queue in front of argon2id. The rate limit is the control. A few parallel guesses can still use CPU. That is accepted for this slice.

---

## 中文

**状态：已实现。** 本文是第一次真正登录的规格。代码和本文不一致时，在修改本文的 pull request 合进来之前，以本文为准。这是路线图第 1 片（壳和语言已经在了）。节点的启动和停止仍按[节点启停](../DESIGN-node-lifecycle.md)，并且现在需要会话。

### 这片做什么

manager 可以启动和停止 Kubo。后面的切片会让别的机器上的浏览器也能做这件事。陌生人只要能打开端口就不能做，这才安全。这一片加上一个管理员、一个会话，以及一个陌生人完成不了的首次设置。

浏览器仍然不和 Kubo 说话。它只和 manager 说话。`GET /health` 仍是 manager 进程的检查，并且保持开放，方便进程监控。其他所有 `/api` 都要有会话。

### 威胁模型

要保护的：

- 管理员密码，以及存它的那个文件。
- 代替密码的会话。
- `/api` 后面的动作，包括本机 Kubo 的启动、停止和重启。
- 第一次启动、还没有管理员的那一刻。谁完成设置，谁就是管理员。

假设的对手：

- 能连上 manager 端口的另一个人或另一个进程。默认仍是 `127.0.0.1`。以后的远程切片，或者误设的 `-addr`，会改变这一点。
- 操作者已经打开的网站。它会尝试提交面板的表单（CSRF），或者猜密码。
- 能读到数据目录备份、但读不到正在运行的 manager 内存的人。
- 能读一次 manager 标准输出的人。设置口令故意打在那里。用过之后，或者进程退出之后，那一行就不再是钥匙。

同一个用户的 shell 本来就能做的事：读数据目录、停掉进程、运行 `reset-admin`。会话不假装能防住操作者自己。逃生口就是这个 shell。

这一片不打算挡住的：让页面执行攻击者脚本的缺陷（XSS）。会话 cookie 是 `HttpOnly`，脚本读不到它。但脚本如果已经在页面上跑，它仍然可以从页面调用 `/api`。面板里不渲染不受信任的 HTML，是另一条规矩。

### 决定

1. **只有一个管理员。** 没有多账号，没有角色，没有邀请。第一次成功的设置创建唯一的用户。第二次设置是 **409** `already_setup`。

2. **密码用 argon2id**（`golang.org/x/crypto` 当前已修补的版本；更旧的标签过不了依赖审查）。存下来的是 PHC 字符串（`$argon2id$v=19$m=19456,t=2,p=1$...`）。新哈希的参数：内存 19 MiB（`m=19456`），时间 2，并行度 1，标签 32 字节，盐 16 字节。这是 OWASP 的交互式设置。校验时用字符串里的参数，并拒绝超过 256 MiB、超过 10 轮或超过 4 条并行的字符串，避免有人改文件让进程分配一块巨大的内存。用户不存在时，仍用启动时按同样参数生成的假哈希做一次校验，这样响应时间不会回答「有没有这个用户」。

3. **哈希放在 manager 的数据目录里，绝不放进 Kubo 仓库。** 文件是 `<data>/auth.json`，权限 `0600`。目录权限 `0700`。先写到同目录的临时文件，`fsync`，再改名。默认数据目录是 `$XDG_DATA_HOME/ipfs-panel`，没设 `XDG_DATA_HOME` 时是 `~/.local/share/ipfs-panel`。Kubo 仓库仍是 `<data>/kubo`。参数：`-data` / `IPFS_PANEL_DATA`。如果这个文件会落在 Kubo 仓库里面，manager 拒绝启动。仓库会交给 `ipfs` 子进程，密码哈希不能跟着进去。

4. **设置只允许来自本机回环，或者带一次性口令。** 回环指 TCP 对端是 `127.0.0.1` 或 `::1`。manager 看的是这条连接，不是 `X-Forwarded-For`。能连上端口的客户端可以随便写这个头，代理也可以被要求送上 `127.0.0.1`。相信这个头，远程客户端就能跳过口令。口令是给另一种情况的：操作者有意把 manager 公布出去，人站在日志前面，而不是站在这台机器的回环上。没有管理员时，manager 启动会把一个口令打到标准输出和日志。内存里只留口令的 SHA-256。口令不落盘。不是回环的设置必须带上它（JSON 字段 `setupToken`）。口令错了不会作废。设置成功才会清掉。设置完成前重启，会打一个新口令，旧的失效。回环上的设置不需要口令。只有状态里 `setupTokenRequired` 为真时，界面才要口令。这个字段的意思是「还没有管理员，并且这条 TCP 连接不是回环」。

5. **会话是存在服务器上的不透明随机值。** cookie `ipfs_panel_session` 装着 32 个随机字节的编码。服务器存的是这个值的 SHA-256、用户名、一个 CSRF 秘密，以及两个时间：创建时间和最后一次见到的时间。cookie 是 `HttpOnly`、`SameSite=Strict`、路径 `/`。连接是 TLS 时带 `Secure`；或者打开了 `-trust-proxy` / `IPFS_PANEL_TRUST_PROXY` 且 `X-Forwarded-Proto` 为 `https` 时也带。客户端自己送这个头，不算。空闲 12 小时过期。绝对寿命 7 天。出示 cookie 时两条都检查。浏览器的 `Max-Age` 跟绝对寿命一致。重启会丢掉所有会话；会话不写盘。12 小时盖得住一个离开键盘的工作日。7 天意味着偷到的 cookie 不能无限续期。两个数字放在一处（`IdleTimeout`、`AbsoluteTimeout`），以后的远程切片要缩短时不用重做设计。

6. **CSRF 用一个 HTML 表单编造不出来的秘密。** `GET /api/auth/setup-status` 设置 cookie `ipfs_panel_csrf`（`SameSite=Strict`，不是 `HttpOnly`，这样页面读得到），并在 `csrfToken` 里返回同一个值。登录和设置必须在 `X-CSRF-Token` 头里送回这个值。跨站表单设不了这个头。登录之后，会话有自己的 CSRF 秘密。服务器用常数时间比较把头和这个秘密比对。安全方法（`GET`、`HEAD`、`OPTIONS`）不需要它。会改变状态的请求如果带了 `Origin`，它的主机必须和请求的 `Host` 一致。浏览器在 `POST` 上会送 `Origin`。非浏览器客户端可以不送。开发代理保持 `Host` 为页面的主机，所以两者一致。

7. **登录按 IP 和按用户名限速。** 前四次失败不延迟。下一次等 1 秒，然后 2 秒、4 秒，如此类推，最多 15 分钟。客户端得到 **429** `rate_limited` 和 `Retry-After`。等待不发生在处理函数里面，所以一波请求不会堆出一堆睡着的 goroutine。成功会清掉两个桶。IP 是连接对端；打开了 `-trust-proxy` 时，用 `X-Forwarded-For` 最右边的一项（单个代理追加的那一项）。用户名桶用尝试的名字的小写，所以 `Ada` 和 `ada` 共用一把锁。密码不是键。

8. **失败提示是笼统的。** 用户名错、密码错、还没设置就登录，都是 **401** `{"error":"invalid_credentials"}`。正文不说错在哪一段。限速是另一个状态，因为操作者需要等，而不是继续猜。设置和密码规则有自己的 **400**（`invalid_username`、`invalid_password`、`password_unchanged`）。它们不泄露存着的密码。

9. **秘密的比较是常数时间。** 密码标签、会话的 CSRF 秘密、设置口令、登录前的 CSRF cookie，都用 `crypto/subtle`。用户名补到固定宽度后同样比较。会话查找用 256 位值的哈希当 map 的键；猜中一个键不是这里的威胁。

10. **忘记密码是在这台机器上清掉，不是发邮件。** `manager reset-admin` 删除 `<data>/auth.json`，并打印接下来怎么做。它不接受密码。密码出现在命令行上，会进进程列表和 shell 历史。先停掉 manager。正在跑的进程会把旧哈希留在内存里直到退出，会话也还在。然后再启动。没有管理员，于是它打印一个新的设置口令，界面进入设置。回环上口令可选。其他地方必须有口令。

11. **审计行是结构化的 slog，而且不是设置口令那一行。** 登录、退出、设置、改密码都会打 `msg=audit`，带 `event`、`result`、`username`、`ip`。结果包括 `success`、`failure`、`rate_limited`、`forbidden`、`invalid`、`exists`。这一行绝不包含密码、哈希、会话 cookie、CSRF 秘密或设置口令。设置口令在启动时单独打一次。那一行是决定 4 的送达方式，不是审计事件。不要把那条流送到别人能读的日志主机，除非你也把它当成秘密库。

12. **密码规则。** 至少 12 个 Unicode 字符，至多 128 个，不能有 NUL。同一个字符重复，以及一个很短的拒绝表（`password1234`、`123456789012`，以及少数同类），会被拒绝。普通单词组成的短语可以通过。界面上的强度计是提示。服务器才是规则。12 个字符、服务器仍会接受的密码，强度计可以说「偏弱」。

13. **公开的路由。** 这些不需要会话：

    | 方法和路径 | 为什么开放 |
    | --- | --- |
    | `GET /health` | 进程检查。不在 `/api` 下。 |
    | `GET /api/auth/setup-status` | 页面得知道该显示设置还是登录，还得拿到 CSRF 令牌。 |
    | `POST /api/auth/setup` | 创建管理员。由回环或设置口令把关，再加上 CSRF。 |
    | `POST /api/auth/login` | 创建会话。由密码、限速和 CSRF 把关。 |

    其他没有会话的 `/api` 路径都是 **401** `{"error":"unauthorized"}`，包括不存在的路径。有有效会话的未知路径仍是 **404**。会改变状态的路由没有 CSRF 头时是 **403** `{"error":"csrf"}`。界面把浏览器带到 `/login`，并把原本要去的路径放在 `redirect` 里。登录成功后再回去，但只回到本站路径（以 `/` 开头、不以 `//` 开头、且不是登录页或设置页）。

### API

这些 POST 的 `Content-Type` 必须是 `application/json`。其他类型是 **415**。未知的 JSON 字段是 **400**。正文限制 8 KiB。

`GET /api/auth/setup-status`

```json
{ "setupRequired": true, "setupTokenRequired": false, "csrfToken": "..." }
```

`POST /api/auth/setup`，正文 `{ "username", "password", "setupToken?" }`。成功是 **201** 和 `{ "username", "csrfToken" }`，加上会话 cookie。浏览器就此处于登录状态。

`POST /api/auth/login`，正文 `{ "username", "password" }`。成功是 **200**，形状相同。

`GET /api/auth/session` 需要会话。**200** `{ "username", "csrfToken" }`。刷新页面靠它重新拿到 CSRF 秘密。

`POST /api/auth/logout` 需要会话和 CSRF 头。**204**。cookie 被清掉。

`POST /api/auth/password`，正文 `{ "currentPassword", "newPassword" }`，需要会话和 CSRF 头。成功是 **200** `{ "username", "csrfToken" }`。当前会话保留；它的 CSRF 秘密会换掉。这个管理员的其他会话被丢掉。当前密码的检查和登录用同一个笼统的 **401**，以及同一套限速。

用户名：1 到 64 个字符，ASCII 字母、数字、`.`、`_`、`-`，以字母或数字开头。登录时区分大小写。限速桶不区分。

cookie 名：`ipfs_panel_session`、`ipfs_panel_csrf`。

### 界面

没有管理员：显示设置页，不显示外壳。用户名、密码、再输入一次、强度计，并且只有状态要求时才出现设置口令。提交后创建账号并进入面板。

有管理员、没有会话：显示登录页。登录失败时说用户名或密码不对，不说是哪一个。不显示外壳。

有会话：现有的外壳。设置里多一个「账号」页，用来改密码。退出登录在侧栏底部。面板的任何请求如果得到 **401**，浏览器回到登录页，并记住原来的路径。

文案在 `en` 和 `zh`。画面遵循[设计系统](../DESIGN-SYSTEM.md)：WinX 标志、一个主按钮、用令牌而不是写死的颜色，以及一张安静的页面，而不是一张光秃的表单。

### 不在这一片里

- 多个用户、角色、OIDC、SSO、WebAuthn、TOTP。
- 熬过 manager 重启的会话。
- 操作者能在页面上改的超时时间。
- 走网络的「忘记密码」。逃生口是这台机器上的 `reset-admin`。
- 修改用户名。
- 打开公网监听。认证让一个已经公布的 manager 不那么裸露。它不把公布变成默认，也不把 Kubo RPC 放到公网地址上。
- 永久锁住账号。退避最长 15 分钟。
- argon2id 前面的全局队列。限速就是控制手段。少数几次并行猜测仍然会占用 CPU。这一片接受这一点。
