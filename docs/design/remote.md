# Remote control of the manager

**English** · [中文](remote.zh-CN.md)

**Status: implemented.** This file is the spec for opening the same panel from a browser on another machine. If the code and this file disagree, this file wins until a pull request changes it.

The browser still never talks to Kubo. Kubo's API stays on loopback. What moves off the machine is the manager, and only when the operator asks. The web UI is the same one a later desktop shell will reuse. There is no second skin.

[Auth](auth.md) still owns the password, the session cookie, and CSRF. This file owns the listen address, TLS, proxies, and the checks that only matter once a packet can arrive from somewhere else. Where the two files both speak, this file is the later rule.

## What a person can do

On the machine that runs the manager, nothing changes if the flags are left alone. The panel listens on `127.0.0.1:8080`. A browser on that machine opens it. Another machine cannot.

When the manager runs on a home server or a VPS, the operator chooses one of three ways to open it from another browser:

1. **TLS on the manager.** `-tls auto` makes a certificate and prints its fingerprint. Or `-tls-cert` and `-tls-key` point at a certificate the operator already has. The browser opens `https://` on the manager's port.
2. **A reverse proxy the operator runs.** The manager stays on loopback. The proxy terminates TLS and is the only thing that faces the network. `-trust-proxy` is on, and the proxy's address is trusted. This is off unless the flag is set.
3. **Cleartext on a network the operator trusts.** `-insecure` allows a listen address that is not loopback, without TLS. The password and the session travel in the clear. The process says so at startup, and Settings says so on the page.

Settings shows the listen address, the certificate fingerprint when there is one, and the sessions that can be revoked. It does not change the listen address. That is a flag and a restart, so a page on the network cannot rebind the port.

## Threat model

What we protect:

- The admin password while it crosses the network.
- The session cookie that stands in for that password.
- The actions behind `/api`: start, stop, files, pins, config, logs.
- First-run setup, which still creates the only admin.
- Kubo's API (usually port 5001) and the decision to keep it on loopback. Publishing the manager must not publish that port.
- The operator's ability to see a session they do not recognise and drop it.

Who we assume is trying:

- Anyone who can send a packet to the manager's port. A neighbour on the LAN, a scanner on a VPS, a browser on a hostile network.
- A website the operator already has open. It will try to submit the panel's forms, frame the page, or guess the password.
- A DNS rebinding attacker. Their name resolves to the manager's address after the browser has treated that name as their site.
- Someone on the path who can read or change cleartext HTTP.
- A client who sends `X-Forwarded-For` or `X-Forwarded-Proto` and hopes the manager believes it.
- Someone with a copy of the data directory. They get the password hash, as [Auth](auth.md) already says, and the manager's self-signed private key if `-tls auto` was used. They do not get a live session. Sessions are not on disk.

What a same-user shell can already do is unchanged. `reset-admin` is still the escape hatch. The session does not protect the operator from themselves.

What this does not try to stop:

- A bug that runs an attacker's script on the panel page. The session cookie stays `HttpOnly`. A script that is already on the page can still call `/api`.
- A correct password typed by someone who stole it. The rate limit slows guessing. It does not replace the password.
- A reverse proxy the operator configured on purpose and then pointed at an attacker. `-trust-proxy` means that proxy is believed.

## Decisions

1. **The default listen address stays loopback.** `-addr` / `MANAGER_ADDR` default to `127.0.0.1:8080`. Unset means another machine cannot open the manager. A hostname of `localhost` is loopback for this rule. `127.0.0.1` and `::1` are loopback. Anything else is remote, including `0.0.0.0`, `::`, an empty host (`:8080`), a LAN address, a public address, and any other name.

2. **A remote listen without TLS does not start, unless `-insecure` is set.** The flag is `-insecure` / `IPFS_PANEL_INSECURE` (`1`, `true`, `yes`, `on`). The password and the session are the keys to the node. On loopback those bytes never leave the machine. The moment the socket accepts a packet from another machine, anyone on the path can read them unless TLS covers them. A trusted LAN is a real setup, so the flag exists. It is a second decision, not a consequence of typing `0.0.0.0`. The flag does not check that the address is a private prefix. A lab, a VPN, or a cloud network is not always RFC1918, and a private prefix is not a friendly network. The flag's name, the startup line, and the Settings banner are the control. With the flag, startup prints `listening without TLS` and says the password travels in cleartext. If TLS is also configured, the flag does nothing and the log says it was ignored.

3. **TLS is one of three states.** They are mutually exclusive.

    | State | How it is selected | What the manager does |
    | --- | --- | --- |
    | `off` | `-tls off` or unset. This is the default. | Plain HTTP. Legal for a loopback listen. Legal for a remote listen only with `-insecure`. |
    | `auto` | `-tls auto` / `IPFS_PANEL_TLS=auto` | A self-signed ECDSA P-256 certificate, stored at `<data>/tls/cert.pem` and `<data>/tls/key.pem`, mode `0600`, directory mode `0700`. The same files are reused so the fingerprint stays put. |
    | `file` | both `-tls-cert` / `IPFS_PANEL_TLS_CERT` and `-tls-key` / `IPFS_PANEL_TLS_KEY` | The operator's PEM certificate and key. The manager does not copy them into the data directory. |

    Setting `-tls auto` and a certificate path together is an error, and the process does not start. Setting only one of the two paths is an error. Any other `-tls` value is an error.

    The automatic certificate lasts 825 days. It is replaced on the next start when it will expire within 14 days, when it no longer parses, or when it is missing a name or address the manager now wants on it. Replacement prints a new fingerprint. The names on it are `localhost`, `127.0.0.1`, `::1`, the listen host when that host is a specific address or name, every entry in `-tls-hosts` / `IPFS_PANEL_TLS_HOSTS` (comma-separated), and this machine's unicast addresses at start, skipping loopback and link-local, up to 16. A DHCP change is picked up on the next start, which changes the fingerprint. A DNS name the browser will send belongs in `-tls-hosts`.

    The fingerprint is the SHA-256 of the raw certificate, lowercase hex with colons. It is printed to stdout as `ipfs-panel tls fingerprint:` and logged as `fingerprint`. It is not a secret. The private key is never logged and never sent to the browser.

    A certificate the operator provided is used as-is when it parses and the key matches. An expired certificate still starts, with a warning, because the operator handed it over on purpose. A certificate that does not parse does not start.

    The listener allows TLS 1.2 and newer. The minimum is not 1.0.

4. **A reverse proxy is opt-in, and only its own connection is believed.** `-trust-proxy` / `IPFS_PANEL_TRUST_PROXY` stays off by default. When it is on, `X-Forwarded-For` and `X-Forwarded-Proto` are read only if the direct TCP peer is a trusted proxy. Trusted peers are loopback (`127.0.0.0/8` and `::1`) plus `-trusted-proxies` / `IPFS_PANEL_TRUSTED_PROXIES`, a comma-separated list of IPs or CIDRs. A bare IP is one address. A bad entry refuses to start.

    The client address is the rightmost `X-Forwarded-For` entry that parses as an IP. That is the entry a single proxy appends. Entries further left were supplied by the client. If the peer is not trusted, or the header is missing, the connection peer is the client address. A remote client cannot rotate the header to escape the rate limit.

    `Secure` on the cookie follows the same rule. The manager's own TLS sets it. Otherwise it is set only when the peer is trusted and the first `X-Forwarded-Proto` value is `https`. A client-supplied header does not set it.

    `X-Forwarded-Host` is not used for any decision. The `Host` header is what the browser sent, and that is what the allowlist checks. A proxy should preserve `Host`.

    The recommended proxy setup keeps the manager on `127.0.0.1`. The proxy faces the network. Kubo's port is not part of that proxy.

5. **First-run setup does not treat the proxy as the operator.** [Auth](auth.md) still refuses to trust `X-Forwarded-For` as proof that the client is on loopback when the flag is off. When the flag is on and the direct peer is a trusted proxy, setup requires the token unless the rightmost forwarded address is itself loopback. A missing forwarded address requires the token. The proxy must send `X-Forwarded-For`. A direct loopback connection with the flag off still does not need the token. A direct connection that is not loopback still needs it, and the header is ignored.

6. **The Host header is checked when the manager is reachable under a name the operator did not bind.** The check is on when the listen address is remote, or when `-trust-proxy` is on. It is off for a plain loopback listen, so a local script can send any `Host` and `curl` to `127.0.0.1` keeps working.

    A request whose hostname is not allowed gets **421** `{"error":"invalid_host"}`. The body is not read. The response does not echo the header.

    Allowed hostnames:

    - `localhost`
    - any host that is an IP address (a literal does not rebind; the browser's `Host` is the address in the URL)
    - the listen host, when it is a name
    - every DNS name on the certificate
    - every name in `-hosts` / `IPFS_PANEL_HOSTS` and `-tls-hosts`

    The port in `Host` is ignored. DNS rebinding is a name, not a port. An empty `Host` is rejected when the check is on. This is what stops a name the attacker controls from being treated as the panel after it resolves to the manager.

7. **Cookies stay as in Auth, with the proxy rule above.** `ipfs_panel_session` is `HttpOnly`, `SameSite=Strict`, path `/`. `ipfs_panel_csrf` is not `HttpOnly`, so the page can read it, and it is `SameSite=Strict`, path `/`. `Secure` is set when the browser is on HTTPS, by the rules in decision 4. A cleartext remote listen cannot set `Secure`, because the browser would drop the cookie. That is why decision 2 exists.

    Idle and absolute lifetimes stay 12 hours and 7 days on a loopback listen with `-trust-proxy` off. When the listen is remote, or `-trust-proxy` is on, idle is 4 hours and absolute is 24 hours. A cookie that can leave the machine should not live for a week. The browser `Max-Age` matches the absolute lifetime in force. Both clocks are still enforced when the cookie is presented. A restart still drops every session.

8. **CSRF is unchanged, and it sits behind the host check.** State-changing requests need `X-CSRF-Token`. When `Origin` is present, its host must match `Host`. Browsers send `Origin` on `POST`. The host check runs first, so a rebinding name is **421** before the CSRF secret is considered. A cross-site form still cannot set the header. `GET`, `HEAD`, and `OPTIONS` still do not need the token.

9. **Login rate limiting is the lockout.** The first four failures have no delay. The next waits 1 second, then 2, 4, and so on, capped at 15 minutes, per client IP and per username. The client gets **429** `rate_limited` and `Retry-After`. The clock does not sleep inside the handler. A success clears both buckets. The IP is the one from decision 4, so a forged forwarded header does not create a fresh bucket. The username bucket is the attempted name, lowercased, so many source addresses do not multiply guesses against one account.

    There is no permanent lock. The only admin, locked out until someone can reach a shell, is a denial of service, and a remote operator may not have that shell to hand. `reset-admin` remains the way to clear the account. Fifteen minutes is the cap [Auth](auth.md) already chose.

10. **Sessions can be listed and revoked.** A session stores an id that is not the cookie, the username, the CSRF secret, the created time, the last-seen time, and the client IP of the latest request. The id is 32 random bytes, encoded. Guessing it is not the threat. The list is how the operator recognises a browser.

    `GET /api/access/sessions` needs a session. It returns every live session for this process. `current` is true for the cookie that asked. The body has no cookie, no hash, and no CSRF secret.

    `POST /api/access/sessions/revoke` with `{ "id" }` needs a session and the CSRF header. **204** on success. The current session clears its cookies. An unknown id is **404** `not_found` and writes no audit line.

    `POST /api/access/sessions/revoke-others` with `{}` drops every session except the current one. **204**.

    Sessions are still memory only. A restart drops them. Changing the password still drops the others and rotates the CSRF secret on the current one.

11. **Audit lines carry the client IP, and whether it is remote.** Each of login, logout, setup, password change, and session revoke logs `msg=audit` with `event`, `result`, `username`, `ip`, and `remote`. `remote` is false when `ip` is a loopback address, and true otherwise, including when `ip` is empty. `ip` is the client address from decision 4. The line never includes the password, the hash, the session cookie, the CSRF secret, the setup token, or the TLS private key. Revoking one session uses `event=session_revoke` and `current` true or false. Revoking the rest uses `event=session_revoke_others` and `dropped` set to how many were removed.

    Startup is not an audit event. The process logs `manager listening` with the bound address, the TLS state, and whether the listen is remote. The fingerprint line is separate.

12. **The manager can serve the same web UI.** `-web` / `IPFS_PANEL_WEB` is a directory that contains `index.html`, normally `apps/web/dist` after `npm run build`. The manager serves those files on the same listener as `/api` and `/health`. A client-side route that is not a file falls back to `index.html`. A missing file that has an extension is **404**. Paths are confined to that directory. `index.html` is not cached. Files under `assets/` may be cached for a long time because their names are hashed.

    Unset, the manager serves no pages. The dev server and its proxy stay the way they are. Cookies stay same-site because the page and the API share an origin. There is no CORS header.

    Responses from the manager, including the pages, send `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, and `Cross-Origin-Resource-Policy: same-origin`. The frame rule is so another site cannot put the panel behind a click. A content security policy is not set. The page has an inline script that paints the theme before the bundle loads, and a policy that blocked it would flash the wrong theme.

13. **Kubo's API stays loopback, in the program.** The listen address of the manager is not written into Kubo's config. `Addresses.API` is still rejected when it is not loopback, on start and in the config editor, as [Node lifecycle](../DESIGN-node-lifecycle.md) and [Config editor](config-editor.md) already require. The manager's Kubo clients still refuse a base URL that is not loopback. `GET /api/access` does not include a Kubo address, a port 5001, or a gateway address. The public gateway scene is a separate decision and is not this feature.

14. **Settings shows the facts and does not pretend to edit them.** The Access section is a grouped list: a label, then rows on one card. It shows a warning when the listen is remote, a stronger warning when that listen has no TLS, the bound address, the addresses on this machine when the bind is a wildcard, the TLS state, the fingerprint with a copy control, whether a proxy is trusted, and the sessions with revoke. A revoke asks first. Revoking this browser signs out. The page says the listen address changes only by restarting the manager with different flags, and that Kubo's control port stays on this computer.

## API

`GET /api/access` needs a session. **200**:

```json
{
  "loopback": false,
  "remote": true,
  "scheme": "https",
  "tls": "auto",
  "insecure": false,
  "trustProxy": false,
  "fingerprint": "ab:cd:…",
  "listen": ["0.0.0.0:8443"],
  "reachable": ["192.0.2.10:8443"],
  "hosts": ["panel.example"]
}
```

`scheme` is `https` when the manager itself speaks TLS, and `http` otherwise. A proxy in front of a loopback manager leaves `scheme` as `http`; `trustProxy` is what tells the page that a proxy is allowed to claim HTTPS. `insecure` is true only when the listen is remote and TLS is off. `fingerprint` is `""` when there is no certificate. `listen` is the address the socket bound. `reachable` is this machine's unicast addresses with that port, excluding loopback and link-local, and empty when the bind is already a single loopback address. It is not the address the public internet uses to reach a NAT. `hosts` is the extra names from `-hosts` and `-tls-hosts`, not every IP. The body has no private key, no certificate PEM, no Kubo address, and no file path.

`GET /api/access/sessions` needs a session. **200**:

```json
{
  "sessions": [
    {
      "id": "…",
      "current": true,
      "created": "2026-11-27T00:00:00Z",
      "seen": "2026-11-27T00:00:00Z",
      "ip": "198.51.100.20"
    }
  ]
}
```

Times are UTC RFC3339. `sessions` is `[]` when there are none, never `null`.

`POST /api/access/sessions/revoke` and `POST /api/access/sessions/revoke-others` need a session and the CSRF header. The content type is `application/json`. Unknown fields are **400**. Success is **204**.

`GET /health` stays open and stays a check of this process. When the host check is on, it still has to pass. A monitor on another machine uses an IP literal or a name in `-hosts`.

## UI

Settings gains an Access section beside General, Account, About, and Advanced. The strings are in `en` and `zh`. The section uses the grouped rows already used for a settings list: a caption, a card, a hairline between rows, the label on the left, the value on the right. It does not introduce a control library. Tokens stay the ones in [Design system](../DESIGN-SYSTEM.md). The warning uses the warning tone. The cleartext warning uses the danger tone. Neither is a green banner.

Loopback with no proxy is a quiet sentence: only this computer can open the manager, and Kubo's control port stays here. There is no alarm for the default.

## How to run it

Dev, unchanged:

```bash
cd services/manager && go run ./cmd/manager
cd apps/web && npm run dev
```

The dev server proxies `/api` and `/health` to `127.0.0.1:8080`. The host check is off in that mode.

Another machine, TLS on the manager:

```bash
cd apps/web && npm run build
cd ../services/manager && go run ./cmd/manager \
  -addr 0.0.0.0:8443 -tls auto -web ../../apps/web/dist
```

Read the fingerprint from stdout. Open `https://` on that port and compare it before signing in. The first visit warns because the certificate is self-signed. That warning is the browser's. The fingerprint is how the operator checks it.

Another machine, a proxy on this computer. The manager stays on loopback. Caddy or nginx listens on 443, proxies to `127.0.0.1:8080`, sets `X-Forwarded-Proto` and appends `X-Forwarded-For`, and preserves `Host`.

```bash
go run ./cmd/manager -addr 127.0.0.1:8080 -trust-proxy \
  -hosts panel.example -web ../../apps/web/dist
```

A trusted LAN, cleartext, only if that is really what is wanted:

```bash
go run ./cmd/manager -addr 0.0.0.0:8080 -insecure -web ../../apps/web/dist
```

An example unit an operator can copy. This repository does not ship it:

```ini
[Service]
ExecStart=/usr/local/bin/manager -addr 0.0.0.0:8443 -tls auto -web /usr/local/share/ipfs-panel/dist
```

The firewall should allow the manager's port and should not allow Kubo's API port. The gateway port is the scene decision, not this one.

## Verification

- With the flags unset, the manager listens on `127.0.0.1:8080` and a remote bind is not attempted.
- `-addr 0.0.0.0:8080` without TLS and without `-insecure` exits before it listens. The error names the flag.
- `-addr 0.0.0.0:0 -tls auto` listens, the certificate file is mode `0600` where the platform has that bit, a second start keeps the fingerprint, and a TLS client that checks the fingerprint can read `/health`.
- A request with `Host: evil.example` is **421** when the check is on, and the handler is not called. `Host: 203.0.113.5` and `Host: panel.example` are allowed when that name was configured.
- `Origin: https://evil.example` on a login whose `Host` is the panel is **403** `csrf`.
- The session cookie is `HttpOnly`, `SameSite=Strict`, path `/`. `Secure` is set on the manager's TLS, and on a trusted proxy's `X-Forwarded-Proto: https` when the peer is loopback. It is not set for that header from a peer that is not trusted, and it is not set for plain HTTP on loopback.
- The fifth failed login from one address is **429** with `Retry-After`. A changing `X-Forwarded-For` from a peer that is not trusted does not reset the wait. The audit line for a remote failure contains that peer's IP and `remote=true`.
- Setup through a trusted proxy without `X-Forwarded-For` requires the token. Setup from loopback with the flag off does not.
- `GET /api/access` after a session does not contain `5001` or a Kubo multiaddr. A public `Addresses.API` is still rejected by the start check.
- Revoking another session makes that cookie **401**. Revoking the current session clears the cookie.

## Out of scope

- More than one admin, SSO, WebAuthn, or TOTP.
- Sessions that survive a restart.
- Editing the listen address, the certificate, or the proxy flag from the page.
- A public CA, ACME, or a certificate the manager fetches.
- Trusting a chain of proxies by counting from the right. One proxy is the rule.
- Opening Kubo's API, or proxying it.
- A second UI for remote use.
- A permanent account lock.
