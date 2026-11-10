# The IPFS Go stack: what to read, and what is true

**English** · [中文](REFERENCE-ipfs-stack.zh-CN.md)

This file is for whoever works on this repository next, especially an AI agent. It answers one question: **when you need to know how the IPFS Go stack behaves, where is the truth, and how do you check it yourself?**

The manager never guesses about Kubo. Every claim in a design doc or a pull request should be traceable to one of the sources here. If you cannot point at a file and a line, say you are unsure instead of writing it down as fact.

## Why this file exists

Three wrong things were said and then corrected while building slice 1–4 and the v0.1.0 release. Each one would have been prevented by reading the source instead of recalling the answer:

1. "Extensions replace a fake CLI stand-in" — wrong; the fake CLI tests process lifecycle only, and never speaks the RPC.
2. "boxo alone can stand in for Kubo in tests" — wrong; boxo has `mfs`, the pinner, and an offline exchange, but **no HTTP RPC server**. The `/api/v0` surface lives in Kubo.
3. "An in-process library node is the right test baseline" — wrong for this project; the contract being tested is made about *released binaries* users install.

The pattern is the same every time: a plausible answer from memory, when the source was one `grep` away. This file is the fix.

## The sources, pinned

Cloned to `reference/` (gitignored, never committed). These exact commits are what every claim in this repository's docs was checked against.

| Repo | Commit | Date | License |
| --- | --- | --- | --- |
| `ipfs/kubo` | `833e04a240d8c533509ddaaa3c202b39b0bb3024` | 2026-09-27 | Apache-2.0 + MIT |
| `ipfs/boxo` | `3d6ac39a0f76c824ffbb90e3a729e78f62bc2d12` | 2026-09-21 | Apache-2.0 + MIT |
| `ipfs/helia` | `cba1ed69b4369924790d9eccdb8c9089435f4f20` | 2026-09-29 | Apache-2.0 + MIT |
| `ipfs/ipfs-docs` | `1261c016f2b6c700e9354e46cedcdf38c9ad7b91` | 2026-09-27 | — |
| `ipfs/specs` | `3000be69497e47c399cdb719440d0809f085388b` | 2026-09-17 | — |
| `ipfs/ipfs-webui` | `c51d980920d60e9d41f53f461886d6e1ea046298` | 2026-07-16 | MIT |

To build or refresh the clones at exactly these commits:

```bash
.github/scripts/fetch-reference.sh
```

It is idempotent: a repo already at its pinned commit is skipped, a missing one is cloned, a stale one is moved to the pin. Do **not** run `git reset --hard origin/HEAD` in those directories — that moves to `master` and every line number in this file stops matching.

Bumping a pin is a deliberate change. Move the commit in the script, update the table above, and re-check every citation (see [How to verify a claim](#how-to-verify-a-claim-before-writing-it)).

`git ls-remote --tags https://github.com/ipfs/kubo` gives the released versions. At the time of writing the latest is `v0.43.1` (2026-09-15). `kubo/version.go:31` on the clone says `0.44.0-dev`, so **the clone is ahead of the release** — never use `master` as a test baseline.

## Division of labor: kubo or boxo

Kubo's own `AGENTS.md` states the rule, and it is worth internalizing because it tells you where to look:

> Boxo is the Go SDK for IPFS … Kubo is one consumer of boxo, not the only one, so reusable building blocks live in boxo … **Goes in kubo:** the daemon and product, for example the config schema, CLI commands (`core/commands/`), the `/api/v0/` RPC surface, node construction and lifecycle, the on-disk repo, plugins, and migrations.
> — `reference/kubo/AGENTS.md`

So:

| You want to know about | Read |
| --- | --- |
| `/api/v0/files/*`, `/api/v0/pin/*` request and response shapes | `kubo/core/commands/` |
| How the RPC handler is mounted and served | `kubo/core/corehttp/`, `kubo/core/corehttp/commands.go` |
| The config file schema, defaults, `docs/config.md` | `kubo/config/`, `kubo/docs/config.md` |
| Node construction, the fx graph | `kubo/core/node/`, `kubo/core/core.go` |
| Plugin interfaces and loading | `kubo/plugin/`, `kubo/plugin/loader/` |
| MFS semantics, CID shaping for directories | `boxo/mfs/` |
| Pinning internals (`pin.Pinner`) | `boxo/pinning/pinner/` |
| UnixFS import: chunking, DAG layout | `boxo/ipld/unixfs/`, `boxo/chunker/` |
| Offline block access for tests | `boxo/exchange/offline/` |
| Block and blockstore layers | `boxo/blockstore/`, `boxo/blockservice/` |
| Protocol definitions (wire formats) | `specs/` — this is the source of truth, not any implementation |

## Facts worth having ready

Each one is checked against the clone. Follow the citation if you need to confirm it.

### The `/api/v0` contract

- The handler is mounted by `corehttp.CommandsOption(cctx)`, which calls `commandsOption(cctx, corecommands.Root)` — `kubo/core/corehttp/commands.go:124,202`.
- `commands.Root`'s subcommands are registered statically in `rootSubcommands`, including `"files": FilesCmd` and `"pin": PinCmd` — `kubo/core/commands/root.go`. **A command being present does not require plugins.**
- The compatibility rule is a kubo project rule, not a code detail: never remove an endpoint, rename a field, change a default, or alter a response shape. Adding an endpoint or an optional argument is allowed — `kubo/AGENTS.md`.
- The API version string is `/kubo/<version>/` — `kubo/version.go:33`.

### MFS and files

- `files/write` is multipart. The server-side reader is Kubo's; the client side in this repo is hand-written. That asymmetry is exactly why the multipart path needs a real-binary test.
- **`Type` is encoded differently by `files/ls` and `files/stat`, and this is easy to get wrong.** `files/ls` emits an integer, `int(fsn.Type())` — `kubo/core/commands/files.go:761`. `files/stat` emits a string, `"directory"` or `"file"` — `kubo/core/commands/files.go:412-414`. The shapes are `lsEntry.Type int` and `statOutput.Type string` in Kubo; this repo mirrors them as `lsEntry.Type int` (`internal/files/client.go`) and a string on the stat path. A client that uses one type for both will fail on one of them, and a hand-written stub will agree with whichever the author picked.
- `files/ls` with `long=true` returns entries with `Name`, `Type`, `Size`, `Hash`, `Mtime`, `MtimeNsecs` — `kubo/core/commands/files.go:168-180`.
- MFS root operations go through `boxo/mfs`, whose HAMT parameters decide a directory's CID — `boxo/AGENTS.md`, "mfs/, files/, blockservice/".

### Pinning

- `pin.Pinner` is produced in the fx graph — `kubo/core/node/core.go:62`. It is therefore replaceable with `fx.Decorate`.
- `pin/ls` accepts `names` and `stream`. This repo retries without `names` when Kubo rejects it; the rejection message shape is a guess and is listed as such in [Harness](design/harness.md).

### Plugins

- Six interfaces, all in `kubo/plugin/`: `PluginDaemon` (`kubo/plugin/daemon.go:9`), `PluginDaemonInternal` (`kubo/plugin/daemoninternal.go:10`), `PluginFx` (`kubo/plugin/fx.go:17`), `PluginDatastore` (`kubo/plugin/datastore.go:9`), `PluginIPLD` (`kubo/plugin/ipld.go:9`), `PluginTracer` (`kubo/plugin/tracer.go:8`).
- `PluginFx.Options(core.FXNodeInfo) ([]fx.Option, error)` — the documented pattern is `fx.Replace()` or `fx.Decorate()` on an existing interface — `plugin/fx.go:10-17`.
- `PluginDaemon.Start(coreiface.CoreAPI) error` — the node is already built; this is the read-and-drive entry point — `plugin/daemon.go:9-13`.
- **External plugins are Linux, FreeBSD, and macOS only.** Go says so in `$GOROOT/src/plugin/plugin.go` (Warnings), and Kubo says so in `kubo/docs/plugins.md:136-137`. The build tags confirm it: `load_unix.go` and `load_nocgo.go` are both gated on `(linux || darwin || freebsd)`; there is no Windows loader.
- Windows therefore needs a **preloaded** plugin: add it to `plugin/loader/preload_list` and rebuild — `plugin/loader/preload.sh`, and `kubo/docs/plugins.md`, "Preloaded Plugins".
- Go's plugin ABI requires the plugin and the host to be built with the *same* toolchain version, the same build tags, and identical shared dependencies. Crashes otherwise — `$GOROOT/src/plugin/plugin.go`, Warnings.
- Kubo does not promise fx stability: *"Kubo cannot guarantee backwards compatibility for `fx` customizations"* — `kubo/docs/plugins.md`.

### Testing a node without the network

- `boxo/exchange/offline.Exchange(bs)` reads only the local blockstore and answers "block was not found locally (offline)" on a miss. No network — `boxo/exchange/offline/offline.go`.
- `kubo/core.BuildCfg{Online: false, Routing: libp2p.NilRouterOption}` builds a non-networked node — `kubo/core/node/builder.go`, and `kubo/docs/examples/kubo-as-a-library/main.go`.
- But for **this project** the test baseline is a released binary, not a library import. See [Harness](design/harness.md).

### Helia and Kubo share one network

This matters when someone asks whether a JavaScript panel would be "a separate network". It would not.

- Helia's interop suite starts a real Kubo node through `ipfsd-ctl` and dials it: `await helia.libp2p.dial((await (await kubo.api.id())).addresses)` — `reference/helia/packages/interop/src/car.spec.ts:27-30`.
- The fixture runs the `kubo` npm package's binary (`bin: kuboPath()`) with pinned import profiles — `packages/interop/src/fixtures/create-kubo.ts`.
- README: *"An interop suite ensures everything is compatible"* — `helia/README.md:223`.

So Helia, Kubo, and any panel talking to either are all on the same libp2p network. "Joining the network" is not a reason to choose an in-process design; the network is joined by peering, which both models do.

## How to verify a claim before writing it

1. **Find it in the clone.** `grep -rn "the thing" reference/kubo reference/boxo`. A claim with no hit is a claim you do not have.
2. **Read the surrounding code**, not just the matching line. A function name does not establish its semantics.
3. **Check whether the clone is the release.** `git ls-remote --tags` for kubo. `master` carries `-dev` versions.
4. **Prefer the spec** for anything on the wire. `reference/specs/` outranks any implementation, including Kubo.
5. **Say "unverified" when it is unverified.** A labelled unknown is worth more than a confident error; three of those were already corrected in this repository.

Two scripts enforce the mechanical half:

| Script | Runs where | Checks |
| --- | --- | --- |
| `.github/scripts/check-docs.sh` | CI, on any `docs/**` change | every doc has a `.zh-CN` pair, the language switch is present, relative links resolve |
| `.github/scripts/check-doc-citations.sh` | locally, needs `reference/` | every `file:line` in these docs resolves and is within the file |

Run the second one after editing a doc or moving a pin in `fetch-reference.sh`. It exits non-zero when a citation points at a file that does not exist or a line past the end, which is worse than no citation.

## What not to conclude from this file

- It is not a summary of IPFS. It is a map of where to look.
- The pinned commits age. Refresh them and re-check before quoting a line number.
- A fact about `master` is not a fact about the release. Keep the two separate in your head, and say which one you mean.