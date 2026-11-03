# 参与方式

[English](CONTRIBUTING.md) · **中文**

## 分支模型

长期分支只有 `main` 和 `develop`。

- `main` 是生产。上面的每次提交都应该可以发行，并打上 `vX.Y.Z`。只有 `release/X.Y.Z` 和 `hotfix/X.Y.Z` 的 pull request 可以指向它。
- `develop` 是集成分支，也是仓库的默认分支。功能从这里拉出，再合回这里。

没有第三条长期分支。开一个短期分支，提 pull request，合并，删掉分支。[组织设置](docs/ORG-SETUP.zh-CN.md) 做完之后，GitHub 会在合并后自动删掉头分支。

```text
feature/*, fix/*, chore/*, docs/*, refactor/*, test/*, ci/*
        \                         /
         ---> develop ------------
                 |            ^
                 v            |
            release/X.Y.Z     |  回并（squash）
                 |
                 v
               main  ---- 标签 vX.Y.Z
                 ^
                 |
            hotfix/X.Y.Z
```

把这套政策加进来的那一次引导 pull request 指向 `main`，因为当时还没有 `develop`。合并之后，从 `main` 建出 `develop`，再把默认分支改成 `develop`。步骤在 [组织设置](docs/ORG-SETUP.zh-CN.md)。`origin/develop` 一旦存在，CI 会拒绝头分支不是 `release/X.Y.Z` 或 `hotfix/X.Y.Z` 的、指向 `main` 的 pull request。

## 分支名

前缀用小写。`feature/login`，不要写成 `Feature/login` 或 `feat/login`。

| 前缀 | 从哪拉 | 合到哪 | 用来做什么 |
| --- | --- | --- | --- |
| `feature/` | `develop` | `develop` | 一片产品功能 |
| `fix/` | `develop` | `develop` | 不紧急的修复 |
| `chore/` | `develop` | `develop` | 工具或仓库杂务 |
| `docs/` | `develop` | `develop` | 只改文档 |
| `refactor/` | `develop` | `develop` | 不改行为的整理 |
| `test/` | `develop` | `develop` | 只改测试 |
| `ci/` | `develop` | `develop` | 只改 GitHub Actions |
| `dependabot/` | `develop` | `develop` | Dependabot 开的 |
| `release/X.Y.Z` | `develop` | `main` | 切一个版本。`X.Y.Z` 就是版本号，不要加 `v` |
| `hotfix/X.Y.Z` | `main` | `main` | 紧急的生产修复。`X.Y.Z` 是新的补丁号 |
| `main` | — | `develop` | 发行或热修之后的回并 |

稳定化期间，`release/*` 和 `hotfix/*` 也可以向 `develop` 提 pull request。把版本放上 `main` 的那一次必须用 merge commit。

## 提交和 pull request 标题

标题用 [Conventional Commits](https://www.conventionalcommits.org/)：

```text
<type>(<可选 scope>): <subject>
```

类型：`feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`revert`。

合进 `develop` 的 squash 用 pull request 标题当作提交标题。CI 会检查标题（`pr-title`）。只有一个提交的 pull request 也会查，因为 GitHub 在 squash 时会建议用那条提交说明。标题写成你希望它在 `develop` 上呈现的样子。

例子：

```text
feat(web): add the language switch
fix(manager): reject POST /health
ci: run go test with the race detector
chore(release): back-merge v1.2.3
```

分支前缀和提交类型不是同一个词。分支叫 `feature/login`。squash 标题是 `feat(web): ...`。

## 合并按钮

GitHub 不能按头分支藏起某个合并按钮。ruleset 是按目标分支限制的。请用下面这个按钮：

| 头分支 | 目标 | 按钮 |
| --- | --- | --- |
| `feature/` `fix/` `chore/` `docs/` `refactor/` `test/` `ci/` `dependabot/` | `develop` | **Squash and merge** |
| `main`（回并） | `develop` | **Squash and merge** |
| `release/X.Y.Z` `hotfix/X.Y.Z` | `main` | **Create a merge commit** |

不要把 release 或 hotfix squash 进 `main`。squash 会丢掉这段本该作为从 `develop` 合并而来的历史。不要用 **Rebase and merge**。仓库设置里关掉了它。

必须的批准人数是 0。`ci-ok` 仍然要绿，分支也要和目标分支对齐。

## 发行

`release-please` 会开自己的分支，这套政策会在 `main` 上拒绝它。版本由人来切，GitHub Actions 负责打标签和上传附件。

1. 从 `develop` 拉出 `release/X.Y.Z`。版本号是下一个 SemVer。分支名里不要加 `v`。
2. 在这条分支上，把 `CHANGELOG.md` 里 `## [Unreleased]` 的说明挪到 `## [X.Y.Z] - YYYY-MM-DD` 下面。装了 `git-cliff` 时可以用仓库根目录的 `cliff.toml` 起草：

   ```bash
   git cliff --tag vX.Y.Z -o CHANGELOG.md
   ```

   手写这个标题也够。CI 只检查标题在不在。
3. 在 release 分支上稳定。修复提交在 `release/X.Y.Z` 上。如果 `develop` 又往前走了，可以先合回 `develop`。
4. 向 `main` 开 pull request。标题例子：`chore(release): 1.2.3`。用 **Create a merge commit** 合并。
5. Release 工作流看到这次合并，在合并提交上打 `vX.Y.Z`，并附上：
   - linux、windows、darwin 的 amd64 和 arm64 管理服务二进制
   - 这些二进制的 `checksums.txt`
   - `ipfs-panel-web_X.Y.Z_dist.zip` 和 `ipfs-panel-web_X.Y.Z_dist.zip.sha256`
6. 如果从 `main` 比到 `develop` 不是空的，开一个回并 pull request：目标 `develop`，头是 `main`。用 **Squash and merge**。标题：`chore(release): back-merge vX.Y.Z`。`develop` 只允许 squash，所以这里不能是 merge commit。比较结果是空的，就不用回并。

标签由 `GITHUB_TOKEN` 创建。这次推送不会再启动第二个工作流。同一个任务里构建附件。你自己推标签也可以：Release 工作流会在 `vX.Y.Z` 标签上跑。标签 ruleset 禁止移动或删除 `v*`。不禁止新建。

`develop` 上的夜间构建只上传工作流附件。它们不是发行，也不是 `v*` 标签。定时工作流跑在默认分支上，所以要等 `develop` 成为默认分支之后才会开始。

## 热修

### 快，但仍然是 pull request

生产等得了 CI 时，用这条。

1. 从 `main` 拉出 `hotfix/X.Y.Z`。`X.Y.Z` 是新的补丁号，不是那个坏掉的版本。
2. 修好，在 `CHANGELOG.md` 里加上 `## [X.Y.Z]`，向 `main` 开 pull request。
3. `ci-ok` 必须是绿的。批准人数是 0，所以 CI 一绿维护者就可以合并。用 **Create a merge commit**。不要跳过 CI。
4. Release 工作流打上 `vX.Y.Z`，附件和正式发行一样。
5. 按发行同样的方式回并到 `develop`：squash，标题 `chore(release): back-merge vX.Y.Z`。

### 紧急情况下直接推

如果 `hotfix/*` 的 pull request 实在等不了（ruleset 或 CI 自己就是故障，或者修复必须在几分钟内落上去），仓库管理员可以直接把一个提交推到 `main`。这次绕过的身份是仓库管理员，模式是 **Always**。这是审计日志里的一次事件，不是习惯。

- 推一个新提交。不要强推，不要删 `main`。
- Release 工作流不会给这次提交打标签，因为没有 `release/*` 或 `hotfix/*` 的 pull request。如果这次提交应该发出去，管理员自己建一个附注标签并推上去：`git tag -a vX.Y.Z -m vX.Y.Z && git push origin vX.Y.Z`。标签推送会构建附件。那个提交上的 `CHANGELOG.md` 必须已经有 `## [X.Y.Z]`，否则发行任务会失败。
- 同一天开一份 [事后说明](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=post_incident.yml)。写上 SHA、为什么不能走 hotfix pull request、谁受了影响，以及后续（回并、缺了标签就补、修补用的 pull request）。

`develop` 没有绕过。后续要进 `develop`，走 pull request。

谁可以这么做，以及批准人数什么时候离开 0，写在 [治理](GOVERNANCE.zh-CN.md)。

## 本地开发

界面在 `apps/web`。Node 22.18 或更新，或者 Node 24.12 或更新。

```bash
npm ci
npm run dev
npm run lint
npm run typecheck
npm test
npm run build
```

管理服务在 `services/manager`。Go 1.26，和 `services/manager/go.mod` 一致。

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build -o /dev/null ./cmd/manager
go run ./cmd/manager
```

`golangci-lint` v2.15.0 和 CI 一致（`services/manager/.golangci.yml`）。从它的发行页安装 v2.15.0 的二进制。`gofmt` 和 `go vet` 仍然单独跑。

编辑器可以是 VS Code，加上官方 Vue 扩展（Volar）和 Go 扩展。推荐列表在 `.vscode/extensions.json`。如果还装着 Vetur，关掉它。不要提交 `.idea/`，也不要提交 `.vscode/` 里的其他文件。

## CI 怎么跑

工作流是 `.github/workflows/ci.yml`。它在指向 `main` 和 `develop` 的 pull request 上跑，也在推到这两个分支时跑。

| 任务 | 什么时候跑 |
| --- | --- |
| `changes` | 总是跑。判断 web 或 manager 的文件有没有变。 |
| `web` | `apps/web/**` 或 CI 工作流变了。`npm ci`、lint、类型检查、vitest、生产构建。 |
| `manager` | `services/manager/**` 或 CI 工作流变了。`gofmt`、`go vet`、`golangci-lint`、`go test -race`、`go build`。 |
| `pr-title` | pull request。检查 Conventional Commits 标题。 |
| `branch-policy` | pull request。上面的 Git Flow 规则，外加脚本自己的测试。 |
| `dependency-review` | pull request。依赖图打开之后，新引入高危或严重漏洞时失败。图还没开时，它记下这一点并保持绿色。 |
| `ci-ok` | 除非这次运行被取消，否则总是跑。上面每个任务成功或被跳过时，它才通过。 |

分支保护只要求 **`ci-ok`**。其他名字在有些事件上会被跳过。把一个会被跳过的检查设为必须，合并会被卡住。不要把 `web` 或 `manager` 单独设为必须。

其他工作流，不作为必须检查：

| 工作流 | 做什么 |
| --- | --- |
| CodeQL | 在 pull request、推送，以及每周定时跑 `codeql-go` 和 `codeql-javascript`。 |
| Release | 从 `main` 打标签并上传 GitHub Release 附件。 |
| Nightly | 从 `develop` 出快照二进制和网页构建，保留 7 天。 |
| Labels | `.github/labels.json` 在 `main` 或 `develop` 上变化时同步标签。 |

Dependabot 按生态系统（npm、Go 模块、GitHub Actions）每周向默认分支开一个分组的 pull request。这些分支用 `dependabot/` 前缀，squash 进 `develop`。

## 产品上的规矩

写清楚这是路线图里的哪一片。还不算产品切片的仓库卫生，就写 foundation。

改界面文案时，同一个 pull request 里同时改 `apps/web/src/locales/en.ts` 和 `apps/web/src/locales/zh.ts`。键要一致。`zh` 是简体中文。

每篇文档都是两个文件：`NAME.md` 是英文，`NAME.zh-CN.md` 是中文。改动时在同一个 pull request 里两边一起改。`CODE_OF_CONDUCT.md` 是唯一的例外：行为准则以英文原文为准，开头加一段中文说明。

和 Kubo 说话的代码放在 `services/manager`。不要加页面、示例或说明，去教别人把 Kubo RPC（5001 端口）开到公网地址上。

- 界面：Vue 3、Vite、TypeScript、Vue Router、Pinia、vue-i18n。
- 管理服务：Go。先用标准库。`ServeMux` 实在难读时可以用 [chi](https://github.com/go-chi/chi)。这个项目不用 GoFrame。
- 想再加一个框架，先在 pull request 里写理由，等同意了再把代码放进来。

## 行为和安全

[行为准则](CODE_OF_CONDUCT.md)。[安全](SECURITY.zh-CN.md)。[支持](SUPPORT.zh-CN.md)。[治理](GOVERNANCE.zh-CN.md)。
