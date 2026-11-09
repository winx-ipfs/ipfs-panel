# Kubo integration: process boundary or plugin

**English** · [中文](kubo-integration.zh-CN.md)

**Status: decision required.** No code. This file compares the two ways the panel can sit next to Kubo, states what each costs, and recommends one. The recommendation is not yet applied; the current tree implements the first model.

Every claim carries its evidence. Sources are cloned under `reference/` at the commits listed in [REFERENCE-ipfs-stack](../REFERENCE-ipfs-stack.md).

## The question

The panel manages a Kubo node. There are two places its code can run:

1. **Outside the Kubo process.** The panel is its own binary. It talks to Kubo over `127.0.0.1:5001` RPC. This is what `services/manager` does today.
2. **Inside the Kubo process.** The panel is a Kubo plugin. Its code runs in the daemon, shares the node's libp2p host, blockstore, bitswap, and DHT. No RPC hop, no second process.

The stated goal for the second model is to join the *existing* IPFS network rather than stand beside it with a private one. That goal deserves a precise answer, because both models join the same network today. Model 1 talks to a local Kubo that is itself a normal IPFS node, peering over the public network like any other. The difference is not *whether* the panel is on the network. The difference is *how much of the node's internals it can see and change*, and how much machinery it has to run to do it.

## What model 1 is, precisely

`services/manager` binds loopback, holds the admin session, and proxies a fixed set of Kubo RPC calls. Kubo runs as a child process the manager starts and stops.

Evidence in this repo:

- `services/manager/internal/node/lifecycle.go` spawns `ipfs init`/`daemon`/`shutdown` and pins the API address to loopback.
- `services/manager/internal/kuborpc/kuborpc.go` refuses any base URL that is not an http loopback origin, and its dialer re-checks the target on every connection.
- `internal/files` and `internal/pins` each own a client for a slice of `/api/v0`.

What it gives up: anything the RPC surface does not expose. What it keeps: a hard security boundary the panel has already invested in. The manager authenticates, and the browser never receives a Kubo address (`docs/VISION.md`).

## What model 2 is, precisely

Kubo has a plugin system with six interfaces (`plugin/*.go`):

| Interface | File | What it receives |
| --- | --- | --- |
| `PluginDaemon` | `plugin/daemon.go:9` | `Start(coreiface.CoreAPI)` — the node already built |
| `PluginDaemonInternal` | `plugin/daemoninternal.go:10` | internal state, unstable by definition |
| `PluginFx` | `plugin/fx.go:17` | `Options(core.FXNodeInfo) ([]fx.Option, error)` — rewires the dependency graph |
| `PluginDatastore` | `plugin/datastore.go:9` | a datastore backend |
| `PluginIPLD` | `plugin/ipld.go:9` | an IPLD codec |
| `PluginTracer` | `plugin/tracer.go:8` | a tracing backend |

`PluginFx` is the one that matters for "low intrusion": it appends fx options, and the documented pattern is `fx.Replace()` or `fx.Decorate()` on an interface already in the graph (`plugin/fx.go:10-15`). Both `exchange.Interface` (`core/node/core.go:38`) and `pin.Pinner` (`core/node/core.go:62`) are produced in that graph, so they can be replaced without touching Kubo's source.

`PluginDaemon` is the simpler entry: `Start(coreiface.CoreAPI)` runs after the node is up and gets the real CoreAPI. A panel that only *reads and drives* the node needs this and nothing more.

### The platform split is real and unavoidable

External plugins load through Go's `plugin` package, which is built on `dlopen`. Kubo's loader selects an implementation by build tag:

- `plugin/loader/load_unix.go` — `//go:build (linux || darwin || freebsd) && cgo && !noplugin`, calls `plugin.Open`.
- `plugin/loader/load_nocgo.go` — `//go:build (linux || darwin || freebsd) && !cgo && !noplugin`, returns "not built with cgo support".
- `plugin/loader/load_noplugin.go` — `//go:build noplugin`, returns "not built with plugin support".

There is no Windows file in that set. Go states the constraint directly: *"Plugins are currently supported only on Linux, FreeBSD, and macOS"* (`$GOROOT/src/plugin/plugin.go`, Warnings section). Kubo's own docs agree for the external case: *"this method is only supported on Linux and MacOS at the moment"* (`docs/plugins.md:136-137`).

So on Windows the only route is a **preloaded** plugin: add it to `plugin/loader/preload_list` and rebuild Kubo. `plugin/loader/preload.sh` generates `preload.go` from that list, and the Makefile target is `make build`. This is a real fork of the Kubo build for Windows users, which is a distribution problem, not a code problem.

## Tradeoffs, stated as tradeoffs

| | Model 1: process boundary | Model 2: plugin |
| --- | --- | --- |
| Reach into node internals | only what `/api/v0` exposes | any fx-decorated interface |
| Shares the libp2p host | no (separate process) | yes, same process |
| Extra process | yes | no |
| Works on Windows | yes, unchanged | only as a preloaded fork of Kubo |
| Distribution on Linux/macOS | one binary + Kubo | Kubo binary + `.so` per platform, matching toolchain |
| Matching toolchain required | no | **yes** — Go plugin ABI demands identical toolchain, build tags, and dependency sources |
| Release cadence coupling | the RPC contract | Kubo's fx graph, which Kubo explicitly does not keep stable |
| Security boundary | the panel authenticates; browser never gets a RPC address | plugin runs with full node privileges |
| Test surface | the released binary (see [Harness](harness.md)) | must build Kubo from source |

Two of those rows deserve emphasis because they are easy to skip:

- **Go plugin ABI.** Go's own documentation warns that runtime crashes are likely unless the application and every plugin are built with *exactly* the same toolchain version, the same build tags, and the same values of certain flags and environment variables, and unless all shared dependencies come from the same source (`$GOROOT/src/plugin/plugin.go`, Warnings). For a project whose users install Kubo from a release, this means each panel release must ship a `.so` built against exactly that Kubo release. That is a support matrix, not a feature.
- **fx stability.** Kubo says the opposite of a promise here: *"Kubo cannot guarantee backwards compatibility for `fx` customizations"* (`docs/plugins.md`, fx section), and `plugin/fx.go:10-13` calls it invasive and warns that breaking changes may occur between releases. Simple interface replacement is the safe subset; anything deeper will break.

## The stated goal, answered

"Join the IPFS network rather than run a separate one" is already true under model 1, because the local Kubo is a full network node. If the goal is instead *"the panel's own logic should participate in the node's networking"* — for example serving content over the node's libp2p host, or answering requests through the node's bitswap — then model 2 is the only way to do it, and the Windows fork is the price.

That distinction is the decision to make, and it should be made explicitly rather than by accident.

## What I recommend

Keep model 1 as the product, and add a small `PluginDaemon` adapter later only if a concrete feature needs in-process access. Reasons, in order:

1. The panel's job as documented is to make a *local* node usable, and that is done over RPC. Nothing in slices 1–4 wants node internals.
2. Model 2's Windows cost is a fork of Kubo, which conflicts with "the user installs Kubo however they install it".
3. Model 2 replaces a versioned HTTP contract with an fx graph Kubo declines to keep stable.
4. The security model in `docs/design/auth.md` assumes the browser never reaches Kubo. A plugin puts panel code inside the process that *is* Kubo.

If a future feature genuinely needs in-process access, the narrowest form is a `PluginDaemon` that hands the panel a handle, not an `PluginFx` that rewires the graph, and not a `PluginDaemonInternal`.

## Open questions

- **Does the panel need the node's bitswap for anything a gateway call cannot do?** If not, model 2 has no feature behind it yet.
- **Would a Windows preloaded plugin mean shipping our own Kubo build?** Yes, and that needs an answer on who updates it and how it is verified.
- **Does a plugin change the auth model?** A plugin cannot be reached over HTTP by the browser directly, so `docs/design/auth.md` still applies to whatever the panel exposes; but the plugin itself runs unsandboxed. That needs writing down before any plugin code lands.