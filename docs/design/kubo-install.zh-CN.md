# 安装与升级 Kubo

**中文** · [English](kubo-install.md)

**Status: implemented.** 这是把 Kubo 二进制装到本机、以及以后换掉它的规格。如果代码和这份文件不一致，以这份文件为准，直到某个 pull request 改掉它。

manager 仍然把 Kubo 当作子进程拉起。不把 Kubo 当 Go 库导入，也不加载插件。浏览器仍然拿不到 Kubo RPC 地址。`Identity.PrivKey` 仍然不会离开 manager。

## 一个人能做什么

机器上没有 `ipfs` 命令的新手，可以在节点页安装钉住的发行版，装完节点是运行着的。之后同一页会说明支持范围内有没有更新的 Kubo。升级会下载、校验、停掉节点、换上新二进制、用 Kubo 自己的仓库迁移，然后再启动。如果失败时仓库版本号还没变，就把上一个二进制放回去并启动。

专家如果设置了 `-kubo-bin` 或 `IPFS_PANEL_KUBO_BIN`，或者 `PATH` 上已经有 `ipfs`，就继续用那个二进制。面板说明自己在用哪一个，不会下载并覆盖它。

## 二进制从哪来

两个来源，按这个顺序。浏览器不能选。运维可以用 `IPFS_PANEL_KUBO_DIST` 和 `IPFS_PANEL_KUBO_GITHUB` 把进程指到镜像。不设置时就是下面这两个源。请求体里带 URL 是 `bad_request`。

| 顺序 | 来源 | 压缩包 | 校验和 |
| --- | --- | --- | --- |
| 1 | `https://dist.ipfs.tech/kubo/v<version>/<asset>` | 发行版压缩包 | 同一路径加上 `.sha512` |
| 2 | `https://github.com/ipfs/kubo/releases/download/v<version>/<asset>` | 同名文件 | 同一路径加上 `.sha512` |

linux 和 darwin 的文件名是 `kubo_v<version>_<os>-<arch>.tar.gz`，windows 是 `kubo_v<version>_<os>-<arch>.zip`。`<os>` 是 `linux`、`darwin` 或 `windows`。`<arch>` 是 `amd64` 或 `arm64`。其他平台是 `unsupported_platform`，什么都不下载。

压缩包里有 `kubo/ipfs`，Windows 上是 `kubo/ipfs.exe`，另外还有许可证和说明。manager 只解出这一个二进制。不运行 `install.sh`。路径逃出目标目录、符号链接，或多出来的二进制，都是 `verify_failed`，目标目录删掉。

2026-11-27 对照 GitHub 发行版 `v0.43.1`（发布于 2026-09-15）核对过。那个发行版有三十个附件：本面板会安装的六个压缩包，freebsd、openbsd、linux-riscv64 的同类文件，源码包，以及每个旁边的 `.sha512` 和 `.cid`。没有 `.asc`，没有 minisign 文件，也没有别的分离签名。`.cid` 是内容标识，不是签名，本面板不把它当成签名。写这份文件时，`https://dist.ipfs.tech/kubo/versions` 在当时的网络上没有应答（连接超时）。安装器仍然先试这个源，不应答时再用 GitHub 发行版。

## 完整性

压缩包的 SHA-512 对上之前，什么都不执行。

钉住的版本是 `0.43.1`，和[测试基线](harness.zh-CN.md)是同一个钉。它的 SHA-512 编译进 manager，每个平台一个，2026-11-27 从 GitHub 发行版的旁路文件抄来。linux-amd64 的哈希就是 `.github/scripts/fetch-kubo.sh` 里的那个。

| 平台 | 压缩包的 SHA-512 |
| --- | --- |
| linux-amd64 | `ff53b2428794fc8cca39505d28c15b4cceaef4b90a09284f291f611fcdc8c06399690575fc89cbd4d85ef71f3652581296c6f2e710386f887c8edf36b6e89d71` |
| linux-arm64 | `70f082584651ef78fb5b07448be53bb0adf141aadcb7eea15ffa59bd9ca78fed46781c2cf5915b3528c591052f97486067fedb0a7a1415abd5bfd44942921501` |
| darwin-amd64 | `efe0c1561595fbf7b5bdc3fc651863697c2bf388eed5f8c94c163f60d9575201808538abba84f652718f015ad1af6e3c01ced1321ddb2082f934ba8952e4a4ab` |
| darwin-arm64 | `bc282d781d856736051ce6d216ba47d52bec703f1a54ab5fa20fba26a92e4af3e8fb131e220cdf01f20205f1effb1a5b7bedbdeff15e8edbbe8e5b01923a7cbb` |
| windows-amd64 | `e611cecef25cddb10b7f907145383608d9b31219ad7120041aeacfe2313882da1944f2897fda29bc2d959dd357e4950089ec8f2cbd52b29f2fc472bcae4356d0` |
| windows-arm64 | `a6f2578ad3386a567c437eeebac1646293276b5fb44a26a33195f4310ed7b06cab9dfe0e02e17f33ce8ba1beb4f1ccef45375a861c8cc60192f46da7e0c02166` |

对于 `0.43.1`：

1. 从每个有应答的源取 `.sha512` 旁路文件。旁路文件的第一个字段是哈希，后面是文件名，也就是 `sha512sum` 写出的样子。
2. 旁路文件和编译进去的哈希不一致，就是 `verify_failed`，哪怕另一个源是对的。
3. 至少要有一个旁路文件，并且等于编译进去的哈希。每个源都连不上，结果是 `offline`。
4. 下载压缩包，先试 dist.ipfs.tech，再试 GitHub。每次尝试限 45 秒。一个源没有在时限内完成，就当作它没有应答，再试另一个源。文件的 SHA-512 必须等于编译进去的哈希。对不上就删掉半截文件，结果是 `verify_failed`。
5. 这之后才解压，并运行 `ipfs version --number`（这个旗标被拒绝时改跑 `ipfs version`）。打出来的版本必须是 `0.43.1`。别的输出会删掉解出的二进制，结果是 `verify_failed`。

支持范围内更后面的版本没有编译进去的哈希，也没有签名，所以一个来源不够：

1. 两份旁路文件都必须取到，哈希必须相同。缺一份、对不上，或者哈希不是 128 位十六进制，都是 `verify_failed`。两个连接都失败，结果是 `offline`。
2. 压缩包按同样的方式下载，一次一个源，各限 45 秒。它的哈希必须等于这个共同的值。
3. 解出的二进制的 `version --number` 必须等于请求的版本。

下载超过 256 MiB 就拒绝，不再往下读。解出的二进制用同一个上限。

## 存在哪里

在 manager 的数据目录下，不在 Kubo 仓库里，也不在这个 git 仓库里：

```text
<data>/kubo-bin/state.json
<data>/kubo-bin/versions/<version>/ipfs
<data>/kubo-bin/staging/          下载中的半截文件，任务结束后删掉
```

Windows 上文件名是 `ipfs.exe`。`state.json` 记下当前版本和上一个版本。上一个版本的目录留着，回滚只是改指针，不用重新下载。`state.json` 用写完的临时文件改名替换。`.gitignore` 忽略 `kubo-bin/`，这样有人把数据目录放进工作副本时也不会被提交。

平台有这个权限位时，二进制是 `0755`。数据目录仍然是 `0700`。

## supervisor 跑哪一个二进制

每次启动、停止和收养，按这个顺序：

1. 设置了 `-kubo-bin` / `IPFS_PANEL_KUBO_BIN` 时用它。文件必须存在。这是 `flag`。
2. 否则，如果 `state.json` 指向一个存在的文件，用当前由面板管理的二进制。这是 `managed`。
3. 否则用 `PATH` 上的 `ipfs`。这是 `path`。
4. 否则是 `missing`。

`flag` 和 `path` 是专家自己的二进制。安装和升级用 `external_binary` 拒绝，不往 `kubo-bin/` 里写。显式路径不存在时，仍然是[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)里的 `binary_not_found`。页面给的是重试，不是安装。

## 版本钉和支持范围

| | |
| --- | --- |
| 钉，也就是安装会下载的版本 | `0.43.1` |
| 支持范围 | `>=0.43.1` 且 `<0.45.0` |

这个范围是面板的限制，不是 Kubo 的。RPC 面在 `0.43.1` 上测过。Kubo 自己的规则是发行版之间不破坏 `/api/v0`，所以下一个次版本 `0.44` 可以提供升级。`0.45.0` 及以上不下载，直到某个 pull request 提高上限并写明原因。比上限新的稳定版仍然会报告，页面可以说它存在，但不给升级按钮。

目录是 `https://dist.ipfs.tech/kubo/versions`，一行一个版本。这个源不应答时，manager 读 GitHub 的发行版列表，只保留稳定版本（`v0.43.1` 这种，不是候选版，不是草稿）。范围内最新的稳定版是 `latest`。不设上限时最新的稳定版是 `upstream`。这次检查记住 15 分钟。检查失败不改二进制，也不停掉已经在跑的节点。

只有来源是 `managed`、`latest` 能解析、并且 `latest` 比已安装的版本新时，`upgrade` 才是 true。

## 安装

`POST /api/kubo/install`，正文是 `{}` 或空。只在来源是 `missing` 时允许。已经是 `managed` 则返回 `already_installed`。`flag` 和 `path` 返回 `external_binary`。已经有任务在跑则返回 `busy`。

响应返回之后任务才跑。阶段是 `downloading`、`verifying`、`stopping`、`swapping`、`migrating`、`starting`。

1. 按上面的规则下载并校验钉住的版本。节点此时还不动。这一步失败不会留下二进制。
2. 按住普通的启动和重启。安装任务自己的启动除外。
3. 节点是 `up` 或正在走向 `up` 时，停掉它，等到 `down` 或 `failed`。停不完是 `stop_failed`，放开按住，不替换。
4. 让 `state.json` 指向新版本。
5. 仓库没有 `version` 文件就没有可迁移的东西。supervisor 的第一次启动仍然跑 `ipfs init`，仍然把 API 和网关钉在 loopback，和[节点生命周期](../DESIGN-node-lifecycle.zh-CN.md)已经要求的一样。
6. 仓库有 `version` 文件时，按下面的方式迁移。
7. 启动节点，等到 `up` 或 `failed`。
8. 放开按住。

第一次安装没有上一个由面板管理的二进制。启动失败时，校验过的二进制留着，任务是 `failed`，错误是 `start_failed`，节点页可以再启动它。这不是回滚。

## 升级

`POST /api/kubo/upgrade`，正文是 `{}` 或空。只在来源是 `managed` 且 `upgrade` 为 true 时允许。否则是 `external_binary`、`not_installed` 或 `up_to_date`。

新压缩包在旧二进制还在跑的时候下载并校验。哈希对上之后才停节点。后面和安装一样：按住、停止、让 `state.json` 指向新版本并把 `previous` 设成旧版本、迁移、启动。

## 迁移和回滚

迁移是 Kubo 的命令，不是这个仓库里的代码。`IPFS_PATH` 指向面板仓库，manager 运行：

```text
ipfs repo migrate
```

不加 `--allow-downgrade`。在 Kubo v0.43.1 上，这条命令把仓库迁到该二进制支持的版本。仓库已经是当前版本时，命令成功退出并说明这一点。manager 在命令前和命令后各读一次 `<repo>/version`。

同时满足这几条才回滚：存在上一个版本，新二进制没有变成健康的 `up`，并且 `<repo>/version` 没变（仍然没有，或者还是同一个整数）。任务状态是 `rolled_back`。新目录留在磁盘上，但 `state.json` 不再指向它，所以下次启动不会跑它。然后把上一个二进制放回去并启动。

如果 `<repo>/version` 变了，就不能用旧二进制去启动这个仓库。Kubo v0.43.1 说明降级需要 `--allow-downgrade` 和更旧的二进制，而且可能丢数据。本面板不做这件事。新二进制保持选中，任务是 `failed`，错误是 `migrate_failed` 或 `start_failed`，页面说明节点数据已经更新，所以上一个 Kubo 没有被启动。

回滚后上一个二进制也启动不了，是 `rollback_failed`。每条路径都会放开按住。

## 进度

一次只跑一个任务。形状和固定任务一样：请求马上返回，页面轮询。`GET /api/kubo` 带上当前任务。`GET /api/kubo/jobs/<id>` 返回那个任务。

`phase` 是 `downloading` 时，`received` 和 `total` 是字节数。源没有给长度时 `total` 是 `0`。其他阶段把计数留在下载结束时的大小。`state` 是 `running`、`succeeded`、`failed` 或 `rolled_back`。`error` 为空，或是下面的某个代码。Kubo 的标准输出和标准错误留在 manager 日志里，不抄进 JSON。

已结束的任务留在内存里，新的在前，最多八个。不写磁盘。manager 重启就忘了。二进制和 `state.json` 还在。

## 离线

`GET /api/kubo` 不需要网络就能报告 `source` 和已安装版本。它们来自磁盘、来自旗标，或来自 `PATH`。

目录取不到时，`check` 是 `offline`，`latest` 和 `upstream` 为空，`upgrade` 为 false。节点不会因此被停掉。

安装或升级取不到旁路文件和压缩包时，结果是 `offline`。半截文件删掉。已经是当前的受管理二进制不会被换掉。字节到了但对不上，叫 `verify_failed`，不叫 `offline`。

## 用户自己提供的二进制

| 来源 | 页面怎么说 | 安装 | 升级 |
| --- | --- | --- | --- |
| `missing` | 还没安装 Kubo。按钮是安装。 | 下载钉住的版本 | `not_installed` |
| `managed` | 已安装的版本。`upgrade` 为 true 时可以升级。 | `already_installed` | 上面的流程 |
| `path` | 正在用 `PATH` 上的 Kubo。面板不会替换它。 | `external_binary` | `external_binary` |
| `flag` | 正在用你指定的 Kubo。面板不会替换它。 | `external_binary` | `external_binary` |

`path` 和 `flag` 的版本字符串来自那个二进制的 `ipfs version --number`，缓存一分钟；节点是 `up` 时也可以用正在运行的节点的 agent 版本。读失败就让版本为空，仍然不下载。

## API

会话和 CSRF 与[认证](auth.zh-CN.md)一致。GET 需要会话。POST 需要 `Content-Type: application/json` 和 `X-CSRF-Token`。未知的 JSON 字段是 **400** `bad_request`。第二个 JSON 值是 **400** `bad_request`。

| 方法与路径 | 正文 | 成功 |
| --- | --- | --- |
| `GET /api/kubo` | 无 | **200** 状态 |
| `POST /api/kubo/install` | `{}` 或空正文 | **202** 任务 |
| `POST /api/kubo/upgrade` | `{}` 或空正文 | **202** 任务 |
| `GET /api/kubo/jobs/{id}` | 无 | **200** 任务 |

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

`source` 是 `missing`、`managed`、`path` 或 `flag`。`check` 是 `ok` 或 `offline`。还没有任务时 `job` 是 `null`。读不到二进制时 `version` 是 `""`。正文里没有分发 URL、没有文件路径、没有校验和、没有 Kubo RPC 地址、没有私钥。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `bad_request` | JSON 形状不对 |
| 401 | `unauthorized` | 没有会话 |
| 403 | `csrf` | POST 没有 CSRF 头 |
| 404 | `not_found` | 任务 id 不存在 |
| 409 | `external_binary` | 二进制是 `flag` 或 `path` |
| 409 | `already_installed` | 安装，但已经有受管理的二进制 |
| 409 | `not_installed` | 升级，但没有受管理的二进制 |
| 409 | `up_to_date` | 升级，但范围内没有更新的版本 |
| 409 | `unsupported_platform` | 这个进程不是六个目标之一 |
| 409 | `kubo_busy` | 任务按住节点时的启动或重启 |
| 409 | `busy` | 第二个安装或升级 |
| 415 | `unsupported_media_type` | POST 不是 JSON |
| 503 | `offline` | 源连不上。写在任务上，不写在 GET 上。 |

**202** 之后失败的任务，接下来的 GET 仍是 **200**。`state` 是 `failed` 或 `rolled_back`，`error` 是 `verify_failed`、`offline`、`stop_failed`、`migrate_failed`、`start_failed` 或 `rollback_failed`。

按住期间，`POST /api/node/start` 和 `POST /api/node/restart` 返回 **409** `kubo_busy`。停止仍然可以。

## 界面

节点页。不加新的导航项。

`source` 是 `missing` 时不显示启动。主按钮是安装 Kubo。句子仍是生命周期里停止或失败的那句，第二行说明面板可以下载校验过的发行版。一个安静的链接「自己安装 Kubo」仍然在新标签页打开 `https://docs.ipfs.tech/install/command-line/`。专家靠这个链接离开受管理的路径。

任务是 `running` 时按钮禁用，状态下面一行写当前阶段。下载知道总大小时显示进度条。进度条用主色，不用强调绿。

`source` 是 `managed` 时，属性列表有一行 Kubo，写已安装的版本，节点停着时也在。`upgrade` 为 true 时这一行说明有更新的版本，按钮是升级。`upstream` 比支持范围新时，这一行写出那个版本，不给按钮。`check` 是 `offline` 时，这一行说明没有连上下载站，已安装的那份没变。

`source` 是 `path` 或 `flag` 时，同一行写出版本，并说明面板不会替换那个文件。没有安装，也没有升级。

结束的任务如果 `error` 是 `verify_failed`，说明下载和公布的校验和不一致，什么都没装。`rolled_back` 说明新的 Kubo 没有启动，上一个还在。仓库版本已经变了之后的 `migrate_failed` 或 `start_failed`，说明节点数据已经更新，所以上一个 Kubo 没有被启动。`offline` 说明连不上下载站，这台机器上的节点没变。

节点是 `up` 时，正在运行的 agent 版本仍在连接卡片上。那张卡片除了和这一行共处一页之外不变。

## 审计

请求被拒绝时，以及任务结束时，manager 日志写 `kubo_install` 和 `kubo_upgrade`。字段是 `event`、`result`、`username`、`ip`、`version`，有上一个版本时还有 `previous`。`result` 是 `success`、`rolled_back`、`verify_failed`、`offline`、`stop_failed`、`migrate_failed`、`start_failed`、`rollback_failed`、`external`、`busy`、`up_to_date` 或 `invalid`。这一行没有 URL、没有文件路径、没有校验和、没有私钥。

## 测试

单元测试用本地 HTTP 服务提供压缩包。覆盖：校验和一致、旁路文件和钉不一致、未钉版本的两个源不一致、压缩包路径逃逸、不支持的平台、源离线、`flag` 二进制不被替换、仓库版本没变时回滚、仓库版本变了时不回滚。这些测试里解出来的文件是带 shebang 的脚本。它不是 `fakeipfs`，也不是 Kubo RPC 的替身。

`TestRealKuboInstall` 在 `IPFS_PANEL_INTEGRATION=1` 时由集成测试基线运行。它通过这个服务下载钉住的 linux-amd64 压缩包，核对编译进去的哈希，运行解出的 `ipfs version`，并在临时仓库里运行 `ipfs init`。它不另起一个守护进程。其他 `TestRealKubo*` 测试已经用同一个钉做了那件事。

## 不在这份设计里

- Kubo 插件，或把 Kubo 链成 Go 库。
- 修改 `Addresses.API`。初始化仍然钉在 loopback。
- 把 `Identity.PrivKey`、分发 URL 或本机路径发给浏览器。
- 替换面板不拥有的二进制。
- 降级仓库。
- 安装支持范围以外的版本，包括候选版。
