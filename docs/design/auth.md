# Auth

**English** · [中文](auth.zh-CN.md)

**Status: implemented.** This file is the spec for the first real login. If the code and this file disagree, this file wins until a pull request changes it. This is roadmap slice 1 (the shell and the languages were already here). Node start and stop stay as in [Node lifecycle](../DESIGN-node-lifecycle.md), and now require a session.

## What this is for

The manager can start and stop Kubo. Later slices let a browser do that from another machine. That only stays safe if a stranger who can open the port cannot. This slice adds one admin, a session, and a setup that a stranger cannot finish.

The browser still never talks to Kubo. It talks to the manager. `GET /health` stays a check of the manager process and stays open, so a process monitor can use it. Every other `/api` route needs a session.

## Threat model

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

## Decisions

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

## API

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

## UI

No admin: a setup screen, not the shell. Username, password, confirm, a strength meter, and the setup token field only when the status says it is required. Submit creates the account and opens the panel.

Admin, no session: a login screen. A failed login says the username or password is not right. It does not say which. The shell is not shown.

A session: the existing shell. Settings gains an Account tab with change-password. Log out sits at the bottom of the sidebar. A **401** from any panel request returns the browser to login and remembers the path.

Copy is in `en` and `zh`. The screens follow [Design system](../DESIGN-SYSTEM.md): the WinX mark, one primary button, tokens instead of raw colors, and a calm page rather than a bare form.

## Out of scope

- More than one user, roles, OIDC, SSO, WebAuthn, TOTP.
- Sessions that survive a manager restart.
- Timeouts that an operator can set from the page.
- A network "forgot password" flow. The escape hatch is `reset-admin` on the machine.
- Changing the username.
- Turning on a public bind. Auth makes a published manager less open. It does not make publishing it the default, and it does not put Kubo RPC on a public address.
- Locking the account forever. The backoff cap is 15 minutes.
- A global queue in front of argon2id. The rate limit is the control. A few parallel guesses can still use CPU. That is accepted for this slice.
