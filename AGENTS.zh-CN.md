# 在这个仓库上工作

**中文** · [English](AGENTS.md)

这份文件写给刚接手这个仓库的 AI 或人。动手之前先读一遍。它说明：产品是什么、哪些事已经定了、怎么验证一条结论、以及怎么把仓库留给下一个人。

这里的规则**能自动检查的都自动检查**。只写在文档里的规则会被跳过——这个仓库在一次会话里已经发生过四次。

## 这是什么

Kubo（Go 版 IPFS 节点）的面板。`apps/web` 是界面（Vue 3、Vite、TypeScript）。`services/manager` 是 API 网关（Go，标准库）。浏览器从不直接跟 Kubo RPC 说话；manager 去说，走 loopback，而且先做认证。

`docs/VISION.zh-CN.md` 说明这是什么、以及拒绝变成什么。`docs/ROADMAP.zh-CN.md` 给切片编号；切片 1–4 已作为 v0.1.0 发布，切片 5 是下一个。

## 决定争议的三个文件

来源冲突时，按这个顺序：

1. **`docs/design/*.md`** —— 它描述的东西的规范。文件开头写着：代码与该文件冲突时，以文件为准，直到某个 PR 修改它。改行为就要在同一个 PR 里改规范。
2. **`docs/REFERENCE-ipfs-stack.zh-CN.md`** —— 关于 Kubo 与 boxo 的真相在哪，附每条结论核对时用的确切上游 commit。
3. **上游克隆本身** —— `reference/`，已 gitignore，由 `.github/scripts/fetch-reference.sh` 构建。

如果你无法为一条关于 Kubo、boxo 或 IPFS 的结论指出**文件和行号**，你就是没有这条结论。写"未核实"。本仓库已经写下并推翻过三条自信而错误的说法；每一条只用一个 `grep` 就能避免。

## 循环

1. **先读再写。** 先 `docs/HANDOFF.md`，再你动的那块的设计文档，再 `docs/ROADMAP.zh-CN.md` 看它落在哪。
2. **判断这是切片还是修 bug。** 切片改变一个人能做什么；修 bug 恢复规范已经承诺过的东西。两者都需要一行规范，不只是代码。
3. **在分支上工作，前缀必须合法** —— `feature/`、`fix/`、`chore/`、`docs/`、`refactor/`、`test/`、`ci/`、`dependabot/`。绝不直接提交到 `develop` 或 `main`；两者都受保护，推送会被拒绝。
4. **靠运行来验证**，不是读 diff。细则见下。
5. **向 `develop` 开 PR**，标题用 Conventional Commits。把真正跑过的东西填进 Verification 一节——**没有它 CI 会拒**。
6. **停止前更新 `docs/HANDOFF.md`**，即使工作没做完。

## 验证，按抓问题最多的顺序

三项都必需，不是可选菜单。

**1. 产物真的跑起来。** 启动 manager，走一遍改动的路径，把输出贴回来。测试全绿不等于这一步。改界面就打开页面看；改二进制就跑那个二进制。

**2. 测试套件绿。** 确切命令在 `README.md`。注意哪些需要 C 工具链：`go test -race` 依赖 cgo，没有 gcc 的机器跑不了——见[本地环境](#本地环境)。

**3. 你说的话和观察到的行为一致。** 如果 PR 说节点停机返回 `node_not_running`，就在停机时跑一次并贴出响应。如果说校验和匹配，就把两边都算出来。

关键路径需要人来决定才能继续：任何发布到 `main`、改动认证、写数据目录之外的路径、或删除数据的操作。要问，不要从上一次任务推断出同意。

## 各类工作在哪验证

| 工作 | 命令 |
| --- | --- |
| Manager 单元测试 | `cd services/manager && go test ./...` |
| Manager 竞态（需 cgo） | `CGO_ENABLED=1 go test -race -count=1 ./...` |
| Manager 对发行二进制集成 | 见 [docs/design/harness.zh-CN.md](docs/design/harness.zh-CN.md) —— 尚未接入 CI |
| 界面 | `cd apps/web && npm run lint && npm run typecheck && npm test && npm run build-only` |
| 文档 | `.github/scripts/check-docs.sh` |
| 文档引用 | `.github/scripts/check-doc-citations.sh`（需要 `reference/`） |
| PR 正文 | `.github/scripts/check-pr-body.sh`（CI 任务 `pr-body`） |
| CI 跑的全部 | `.github/workflows/ci.yml`；只有 `ci-ok` 是必需检查 |

CI 在 `ubuntu-latest` 上跑同样的命令。本地过、那边不过，差别就在平台或工具链——**那个差别就是 bug**。

有两个 CI 任务专门用来让这一节落地：`docs` 在文档丢掉 `.zh-CN` 配对或相对链接断裂时失败；`pr-body` 在 PR 没有 Verification 证据时失败。两者都接进 `ci-ok`。

## 平台：反复咬人的地方

manager 发布 linux、macOS、Windows 三种二进制。改动涉及以下任一项，就必须在 Windows 上验一遍，因为三处都只在那边坏：

- **目录 fsync。** `os.File.Sync` 对目录在 Windows 返回 access-denied。`internal/atomicfile` 按平台分开；不要合并。
- **连接被拒。** Windows 报 `WSAECONNREFUSED`（10061），不是 POSIX 的 `ECONNREFUSED`。`internal/kuborpc.IsRefused` 两个都比。
- **权限位。** Windows 没有 `0600`/`0700`。断言 mode 的测试放在 `perm_unix_test.go` / `perm_windows_test.go` 成对文件里。

任何断言 mode、路径分隔符、信号、或错误字符串的测试，都是待爆的可移植性 bug。

## 写文档

每份文档成对：`name.md` 与 `name.zh-CN.md`，各自开头有语言切换行。CI 强制配对，并在相对链接断裂时失败。`CODE_OF_CONDUCT.md` 是唯一例外。

界面文案同理：一个字符串要在同一次改动里同时进 `apps/web/src/locales/en.ts` 与 `zh.ts`。

## 本地环境

以下是在开发这个仓库的机器上观察到的；先检查再假设，但要预期这些：

- **Windows 上没有 gcc**，所以 `-race` 在那里跑不了。WSL 装 Go 与 gcc 可以。**如果跑不了 `-race`，就在 PR 里说明，不要声称有竞态覆盖。**
- **`http_proxy` 是设着的**，会让 `curl` 访问 localhost 和 release 下载失败。访问 localhost 用 `--noproxy '*'`；release 重定向失败时改用 GitHub API 的 asset 端点。
- **`/dev/null` 在这里的 shell 里可能无法解析。** 重定向到真实文件，别丢弃输出。
- **shell 是 Git Bash**，所以 `/c/...` 与 Windows 的 `C:/...` 在不同工具里行为不同。

## 不要做

- **不要猜 Kubo 的行为。** 见 `docs/REFERENCE-ipfs-stack.zh-CN.md`。每个入口和响应形状都能在 `reference/` 里查到。
- **不要为你想测的东西手写替身。** `internal/node/fakeipfs` 是 `ipfs` CLI 的替身，只测进程生命周期；它从不实现 RPC，让它实现等于测我们自己的猜测而不是 Kubo。
- **不要给已经存在的东西加第二种做法。** 规范写了就照做，否则先改规范。
- **不要写一个不会失败的检查。** 这里加的每个脚本都用"注入它本该抓到的错误"验证过。只会打印 OK 的检查比没有更糟。
- **不要提交 `reference/`。** 那是 gitignore 的上游源码，永不属于产品。

## 离开仓库之前

停止之前——无论做完没做完：

1. `.github/scripts/check-docs.sh` 与测试套件通过，或 PR 里写明哪个没过、为什么。
2. `docs/HANDOFF.md` 说明：改了什么、定了什么、还开着什么、下一步是什么。
3. 没有临时文件、没有落在被跟踪路径里的本地克隆、没有注释掉的实验。

一个没有任何对话历史的人，必须能只看文件就回答"现在什么是真的"和"我下一步该做什么"。这就是这份文件存在的目的。