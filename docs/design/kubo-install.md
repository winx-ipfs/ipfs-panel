# Install and upgrade Kubo

**English** · [中文](kubo-install.zh-CN.md)

**Status: implemented.** This file is the spec for getting a Kubo binary onto the machine and replacing it later. If the code and this file disagree, this file wins until a pull request changes it.

The manager still starts Kubo as a child process. It does not import Kubo as a library and it does not load a plugin. The browser still never receives a Kubo RPC address. `Identity.PrivKey` still never leaves the manager.

## What a person can do

A beginner whose machine has no `ipfs` command can install the pinned release from the Node page and end with the node running. Later, the same page says when a newer Kubo inside the supported range exists, and an upgrade downloads it, checks it, stops the node, swaps the binary, runs Kubo's own repo migration, and starts again. If that fails before the repo version changes, the previous binary is put back and started.

An expert who set `-kubo-bin` or `IPFS_PANEL_KUBO_BIN`, or who already has `ipfs` on `PATH`, keeps that binary. The panel shows which one it is using and does not download over it.

## Where the binary comes from

Two origins, in this order. The browser cannot choose either one. An operator can point the process at a mirror with `IPFS_PANEL_KUBO_DIST` and `IPFS_PANEL_KUBO_GITHUB`. Unset, they are the origins below. A request body that contains a URL is `bad_request`.

| Order | Origin | Archive | Checksum |
| --- | --- | --- | --- |
| 1 | `https://dist.ipfs.tech/kubo/v<version>/<asset>` | the release archive | the same path plus `.sha512` |
| 2 | `https://github.com/ipfs/kubo/releases/download/v<version>/<asset>` | the same asset name | the same path plus `.sha512` |

The asset name is `kubo_v<version>_<os>-<arch>.tar.gz` for linux and darwin, and `kubo_v<version>_<os>-<arch>.zip` for windows. `<os>` is `linux`, `darwin`, or `windows`. `<arch>` is `amd64` or `arm64`. Anything else is `unsupported_platform` and nothing is downloaded.

The archive contains `kubo/ipfs`, or `kubo/ipfs.exe` on Windows, plus license and readme files. The manager extracts that one binary. It does not run `install.sh`. A member whose path escapes the destination, a symlink, or a second unexpected binary is `verify_failed` and the destination is removed.

Checked on 2026-11-27 against the GitHub release `v0.43.1` (published 2026-09-15). That release has thirty assets: the six archives this panel can install, the same set for freebsd, openbsd, and linux-riscv64, the source tarball, and a `.sha512` and a `.cid` beside each. There is no `.asc`, no minisign file, and no other detached signature. `.cid` is a content identifier of the bytes, not a signature, and this panel does not treat it as one. `https://dist.ipfs.tech/kubo/versions` did not answer from the network this was written on (the connection timed out). The installer still tries that origin first and uses the GitHub release when it does not answer.

## Integrity

Nothing is executed until the archive's SHA-512 matches.

The pinned version is `0.43.1`, the same pin as [the harness](harness.md). Its SHA-512 is compiled into the manager, one hash per platform, copied from the GitHub release sidecars on 2026-11-27. The linux-amd64 hash is the one in `.github/scripts/fetch-kubo.sh`.

| Platform | SHA-512 of the archive |
| --- | --- |
| linux-amd64 | `ff53b2428794fc8cca39505d28c15b4cceaef4b90a09284f291f611fcdc8c06399690575fc89cbd4d85ef71f3652581296c6f2e710386f887c8edf36b6e89d71` |
| linux-arm64 | `70f082584651ef78fb5b07448be53bb0adf141aadcb7eea15ffa59bd9ca78fed46781c2cf5915b3528c591052f97486067fedb0a7a1415abd5bfd44942921501` |
| darwin-amd64 | `efe0c1561595fbf7b5bdc3fc651863697c2bf388eed5f8c94c163f60d9575201808538abba84f652718f015ad1af6e3c01ced1321ddb2082f934ba8952e4a4ab` |
| darwin-arm64 | `bc282d781d856736051ce6d216ba47d52bec703f1a54ab5fa20fba26a92e4af3e8fb131e220cdf01f20205f1effb1a5b7bedbdeff15e8edbbe8e5b01923a7cbb` |
| windows-amd64 | `e611cecef25cddb10b7f907145383608d9b31219ad7120041aeacfe2313882da1944f2897fda29bc2d959dd357e4950089ec8f2cbd52b29f2fc472bcae4356d0` |
| windows-arm64 | `a6f2578ad3386a567c437eeebac1646293276b5fb44a26a33195f4310ed7b06cab9dfe0e02e17f33ce8ba1beb4f1ccef45375a861c8cc60192f46da7e0c02166` |

For `0.43.1`:

1. Fetch the `.sha512` sidecar from each origin that answers. A sidecar is the hash as the first field, then the file name, which is what `sha512sum` writes.
2. A sidecar that is not the compiled hash is `verify_failed`, even if the other origin matches.
3. At least one sidecar must be present and must equal the compiled hash. If every origin fails to connect, the result is `offline`.
4. Download the archive, trying dist.ipfs.tech and then GitHub. The file's SHA-512 must equal the compiled hash. A mismatch deletes the partial file and is `verify_failed`.
5. Only then extract and run `ipfs version --number` (or `ipfs version` if that flag is rejected). The printed version must be `0.43.1`. Any other output deletes the extracted binary and is `verify_failed`.

For a later version inside the supported range there is no compiled hash and no signature, so one origin is not enough:

1. Both sidecars must be fetched and their hashes must be equal. A missing sidecar, a mismatch, or a hash that is not 128 hex characters is `verify_failed`. If both connections fail, the result is `offline`.
2. The archive must hash to that shared value.
3. The extracted binary's `version --number` must equal the version that was requested.

A download larger than 256 MiB is rejected before the rest is read. The extracted binary has the same cap.

## Where it is stored

Under the manager data directory, never inside the Kubo repo and never in this git repository:

```text
<data>/kubo-bin/state.json
<data>/kubo-bin/versions/<version>/ipfs
<data>/kubo-bin/staging/          partial downloads, removed after the job
```

On Windows the file is `ipfs.exe`. `state.json` names the current version and the previous one. The previous directory is left in place so a rollback is a pointer change, not another download. `state.json` is replaced by renaming a finished temporary file. `.gitignore` ignores `kubo-bin/` so a data directory placed inside a checkout is not committed.

Mode on the binary is `0755` where the platform has that bit. The data directory stays `0700`.

## Which binary the supervisor runs

On every start, stop, and adopt, in this order:

1. `-kubo-bin` / `IPFS_PANEL_KUBO_BIN`, when it is set. The file must exist. This is `flag`.
2. Otherwise the current managed binary, when `state.json` points at a file that exists. This is `managed`.
3. Otherwise `ipfs` on `PATH`. This is `path`.
4. Otherwise `missing`.

`flag` and `path` are the expert's binary. Install and upgrade refuse them with `external_binary` and do not write under `kubo-bin/`. A missing explicit path stays `binary_not_found` from [Node lifecycle](../DESIGN-node-lifecycle.md). The page offers Retry, not Install.

## Version pin and supported range

| | |
| --- | --- |
| Pin, what Install downloads | `0.43.1` |
| Supported range | `>=0.43.1` and `<0.45.0` |

The range is a panel limit, not Kubo's. The RPC surface is tested at `0.43.1`. Kubo's own rule is that `/api/v0` is not broken between releases, so the next minor, `0.44`, may be offered. `0.45.0` and above are not downloaded until a pull request raises the cap and says why. A stable release newer than the cap is still reported, so the page can say it exists, and there is no upgrade button for it.

The catalog is `https://dist.ipfs.tech/kubo/versions`, one version per line. If that origin does not answer, the manager reads the GitHub releases list and keeps tags that are stable versions (`v0.43.1`, not a release candidate, not a draft). The newest stable version inside the range is `latest`. The newest stable version with no range cap is `upstream`. The check is remembered for 15 minutes. A failed check does not change the binary and does not stop a node that is already up.

`upgrade` is true only when the source is `managed`, `latest` parses, and `latest` is greater than the installed version.

## Install

`POST /api/kubo/install` with `{}` or an empty body. Allowed only when the source is `missing`. `managed` returns `already_installed`. `flag` and `path` return `external_binary`. A job that is already running returns `busy`.

The job runs after the response. The phases are `downloading`, `verifying`, `stopping`, `swapping`, `migrating`, `starting`.

1. Download and verify the pin, as above. The node is still untouched. A failure here leaves no binary behind.
2. Hold ordinary Start and Restart. The install's own start is allowed.
3. If the node is `up` or on its way there, stop it and wait until it is `down` or `failed`. A stop that does not finish is `stop_failed`, the hold is released, and nothing is swapped.
4. Point `state.json` at the new version.
5. If the repo has no `version` file, there is nothing to migrate. The supervisor's first start still runs `ipfs init` and still pins the API and gateway to loopback, exactly as [Node lifecycle](../DESIGN-node-lifecycle.md) already requires.
6. If the repo has a `version` file, run migration as below.
7. Start the node and wait until it is `up` or `failed`.
8. Release the hold.

There is no previous managed binary on a first install. If start fails, the verified binary stays, the job is `failed` with `start_failed`, and the Node page can start it again. That is not a rollback.

## Upgrade

`POST /api/kubo/upgrade` with `{}` or an empty body. Allowed only when the source is `managed` and `upgrade` is true. Otherwise `external_binary`, `not_installed`, or `up_to_date`.

The new archive is downloaded and verified while the running node is still the old binary. Only after the hash matches does the job stop the node. The rest matches install: hold, stop, point `state.json` at the new version with `previous` set to the old one, migrate, start.

## Migration and rollback

Migration is Kubo's command, not code in this repository. With `IPFS_PATH` set to the panel repo, the manager runs:

```text
ipfs repo migrate
```

No `--allow-downgrade`. On Kubo v0.43.1 this command migrates the repo to the version that binary supports. A repo that is already current exits successfully and prints that. The manager reads `<repo>/version` before the command and again after.

Rollback puts the previous managed binary back and starts it when all of these are true: a previous version exists, the new binary did not become a healthy `up` node, and `<repo>/version` is unchanged (still absent, or the same integer). The job state is `rolled_back`. The new directory is kept on disk but `state.json` no longer points at it, so the next start does not run it.

If `<repo>/version` changed, the old binary must not be started against that repo. Kubo v0.43.1 says a downgrade needs `--allow-downgrade` and an older binary, and it can lose data. This panel does not do that. The new binary stays selected, the job is `failed` with `migrate_failed` or `start_failed`, and the page says the node data was already updated so the previous Kubo was not started.

A rollback that cannot start the previous binary is `rollback_failed`. The hold is released in every path.

## Progress

One job runs at a time. It is the same shape as a pin job: the request returns immediately, and the page polls. `GET /api/kubo` includes the current job. `GET /api/kubo/jobs/<id>` returns that job.

While `phase` is `downloading`, `received` and `total` are byte counts. `total` is `0` when the origin sends no length. Other phases leave the counts at the download's final size. `state` is `running`, `succeeded`, `failed`, or `rolled_back`. `error` is empty, or one of the codes below. Stdout and stderr from Kubo stay in the manager log. They are not copied into the JSON.

Finished jobs are kept in memory, newest first, up to eight. They are not written to disk. A manager restart forgets them. The binary and `state.json` remain.

## Offline

`GET /api/kubo` never needs the network to report `source` and the installed version. Those come from disk, from the flag, or from `PATH`.

When the catalog cannot be fetched, `check` is `offline`, `latest` and `upstream` are empty, and `upgrade` is false. The node is not stopped.

An install or upgrade that cannot fetch a sidecar and an archive is `offline`. A partial file is deleted. A managed binary that was already current is not replaced. A `verify_failed` result is not called `offline`: the bytes arrived and did not match.

## User-supplied binary

| Source | What the page says | Install | Upgrade |
| --- | --- | --- | --- |
| `missing` | Kubo is not installed. Install is the button. | downloads the pin | `not_installed` |
| `managed` | The installed version. Upgrade when `upgrade` is true. | `already_installed` | the flow above |
| `path` | Using the Kubo on `PATH`. The panel will not replace it. | `external_binary` | `external_binary` |
| `flag` | Using the Kubo you chose. The panel will not replace it. | `external_binary` | `external_binary` |

The version string for `path` and `flag` comes from `ipfs version --number` on that binary, cached for a minute, or from the running node's agent version when the node is `up`. A failure to read it leaves the version empty and still does not download.

## API

Session and CSRF match [Auth](auth.md). GET needs a session. POST needs `Content-Type: application/json` and `X-CSRF-Token`. Unknown JSON fields are **400** `bad_request`. A second JSON value is **400** `bad_request`.

| Method and path | Body | Success |
| --- | --- | --- |
| `GET /api/kubo` | none | **200** the status |
| `POST /api/kubo/install` | `{}` or an empty body | **202** the job |
| `POST /api/kubo/upgrade` | `{}` or an empty body | **202** the job |
| `GET /api/kubo/jobs/{id}` | none | **200** the job |

```json
{
  "source": "managed",
  "version": "0.43.1",
  "pinned": "0.43.1",
  "supported": ">=0.43.1 <0.45.0",
  "latest": "0.44.0",
  "upstream": "0.44.0",
  "upgrade": true,
  "check": "ok",
  "job": {
    "id": "abc",
    "action": "upgrade",
    "state": "running",
    "phase": "downloading",
    "version": "0.44.0",
    "received": 1048576,
    "total": 30000000,
    "error": ""
  }
}
```

`source` is `missing`, `managed`, `path`, or `flag`. `check` is `ok` or `offline`. `job` is `null` when there is no job yet. `version` is `""` when no binary can be read. The body has no dist URL, no file path, no checksum, no Kubo RPC address, and no private key.

| Status | `error` | When |
| --- | --- | --- |
| 400 | `bad_request` | The JSON shape is wrong |
| 401 | `unauthorized` | No session |
| 403 | `csrf` | POST without the CSRF header |
| 404 | `not_found` | The job id is unknown |
| 409 | `external_binary` | The binary is `flag` or `path` |
| 409 | `already_installed` | Install, and a managed binary is already current |
| 409 | `not_installed` | Upgrade, and nothing is managed |
| 409 | `up_to_date` | Upgrade, and nothing newer is inside the range |
| 409 | `unsupported_platform` | This process is not one of the six targets |
| 409 | `kubo_busy` | Start or Restart while a job holds the node |
| 409 | `busy` | A second install or upgrade |
| 415 | `unsupported_media_type` | POST is not JSON |
| 503 | `offline` | The origins could not be reached. Returned on the job, not on GET. |

A job that fails after **202** stays **200** on the following GET. Its `state` is `failed` or `rolled_back`, and `error` is `verify_failed`, `offline`, `stop_failed`, `migrate_failed`, `start_failed`, or `rollback_failed`.

`POST /api/node/start` and `POST /api/node/restart` return **409** `kubo_busy` while the hold is set. Stop is still allowed.

## UI

The Node page. No new navigation item.

When `source` is `missing`, Start is not shown. The primary button is Install Kubo. The sentence stays the lifecycle sentence for a stopped or failed node, and a second line says the panel can download the checked release. A quiet link, "Install Kubo yourself", still opens `https://docs.ipfs.tech/install/command-line/` in a new tab. That link is how an expert leaves the managed path.

While a job is `running`, the button is disabled and a line under the status says the phase. A download with a known size shows a progress bar. The bar uses the primary colour, not the accent green.

When `source` is `managed`, the facts list has a Kubo row with the installed version, including while the node is stopped. When `upgrade` is true the row says a newer version is available and the button is Upgrade. When `upstream` is newer than the supported range, the row names that version and does not offer a button. When `check` is `offline`, the row says the check did not reach the download site and the installed copy is unchanged.

When `source` is `path` or `flag`, the same row names the version and says the panel will not replace that file. There is no Install and no Upgrade.

A finished job with `error` `verify_failed` says the download did not match the published checksum and nothing was installed. `rolled_back` says the new Kubo did not start and the previous one is in place. `migrate_failed` or `start_failed` after the repo version moved says the node data was already updated, so the previous Kubo was not started. `offline` says the download site could not be reached and the node on this machine is unchanged.

The running node's agent version stays on the connection card while the node is `up`. That card is unchanged apart from sharing the page with this row.

## Audit

`kubo_install` and `kubo_upgrade` are written to the manager log when the request is rejected and again when the job finishes. The fields are `event`, `result`, `username`, `ip`, `version`, and `previous` when there is one. `result` is `success`, `rolled_back`, `verify_failed`, `offline`, `stop_failed`, `migrate_failed`, `start_failed`, `rollback_failed`, `external`, `busy`, `up_to_date`, or `invalid`. The line has no URL, no file path, no checksum, and no private key.

## Tests

Unit tests serve the archives from a local HTTP server. They cover a matching checksum, a sidecar that disagrees with the pin, two origins that disagree on an unpinned version, a path that escapes the archive, an unsupported platform, offline origins, a `flag` binary that is not replaced, rollback when the repo version is unchanged, and no rollback when the repo version changed. The unpacked file in those tests is a script with a shebang. It is not `fakeipfs`, and it is not a stand-in for Kubo RPC.

`TestRealKuboInstall` runs in the integration harness when `IPFS_PANEL_INTEGRATION=1`. It downloads the pinned linux-amd64 archive through this service, checks the compiled hash, runs the extracted `ipfs version`, and runs `ipfs init` in a temporary repo. It does not start a second daemon. The other `TestRealKubo*` tests already do that with the same pin.

## Not in this design

- A Kubo plugin, or Kubo linked as a Go library.
- Editing `Addresses.API`. Init still pins loopback.
- Sending `Identity.PrivKey`, a dist URL, or a local path to the browser.
- Replacing a binary the panel does not own.
- Downgrading the repo.
- Installing a version outside the supported range, including a release candidate.
