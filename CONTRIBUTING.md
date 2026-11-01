# Contributing

English below. 中文在后半部分。

## Branch model

`main` and `develop` are the only long-lived branches.

- `main` is production. Every commit on it is releasable and tagged `vX.Y.Z`. Only `release/X.Y.Z` and `hotfix/X.Y.Z` pull requests may target it.
- `develop` is the integration branch and the repository default. Feature work starts here and comes back here.

There is no third long-lived branch. Open a short-lived branch, open a pull request, merge, and delete the branch. GitHub is set to delete the head branch after a merge once [Org setup](docs/ORG-SETUP.md) is applied.

```text
feature/*, fix/*, chore/*, docs/*, refactor/*, test/*, ci/*
        \                         /
         ---> develop ------------
                 |            ^
                 v            |
            release/X.Y.Z     |  back-merge (squash)
                 |
                 v
               main  ---- tag vX.Y.Z
                 ^
                 |
            hotfix/X.Y.Z
```

The bootstrap pull request that added this policy targets `main`, because `develop` does not exist yet. After it merges, create `develop` from `main` and make `develop` the default branch. The steps are in [Org setup](docs/ORG-SETUP.md). Once `origin/develop` exists, CI rejects any pull request into `main` whose head is not `release/X.Y.Z` or `hotfix/X.Y.Z`.

## Branch names

Lowercase prefixes. `feature/login`, not `Feature/login` or `feat/login`.

| Prefix | From | Into | Use |
| --- | --- | --- | --- |
| `feature/` | `develop` | `develop` | A product slice |
| `fix/` | `develop` | `develop` | A non-urgent fix |
| `chore/` | `develop` | `develop` | Tooling or repo chores |
| `docs/` | `develop` | `develop` | Docs only |
| `refactor/` | `develop` | `develop` | Behavior-preserving change |
| `test/` | `develop` | `develop` | Tests only |
| `ci/` | `develop` | `develop` | GitHub Actions only |
| `dependabot/` | `develop` | `develop` | Opened by Dependabot |
| `release/X.Y.Z` | `develop` | `main` | A version cut. `X.Y.Z` is the version, with no `v` |
| `hotfix/X.Y.Z` | `main` | `main` | An urgent production fix. `X.Y.Z` is the new patch |
| `main` | — | `develop` | Back-merge after a release or hotfix |

`release/*` and `hotfix/*` may also open a pull request into `develop` while they stabilize. The pull request that lands the version on `main` is the one that must use a merge commit.

## Commits and pull request titles

Titles follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>(<optional scope>): <subject>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.

Squash merges into `develop` use the pull request title as the commit subject. CI checks the title (`pr-title`). A one-commit pull request is also checked, because GitHub will offer that commit message at squash time. Write the title the way you want it to read on `develop`.

Examples:

```text
feat(web): add the language switch
fix(manager): reject POST /health
ci: run go test with the race detector
chore(release): back-merge v1.2.3
```

The branch prefix and the commit type are not the same word. A branch is `feature/login`. The squash title is `feat(web): ...`.

## Merge button

GitHub cannot hide a merge button per head branch. The rulesets do it per base branch. Use this button:

| Head | Base | Button |
| --- | --- | --- |
| `feature/` `fix/` `chore/` `docs/` `refactor/` `test/` `ci/` `dependabot/` | `develop` | **Squash and merge** |
| `main` (back-merge) | `develop` | **Squash and merge** |
| `release/X.Y.Z` `hotfix/X.Y.Z` | `main` | **Create a merge commit** |

Do not squash a release or hotfix into `main`. A squash drops the history the tag is supposed to point at as a merge from `develop`. Do not use **Rebase and merge**. It is disabled in the repository settings.

Required approvals are 0. You still need a green `ci-ok` check, and the branch has to be up to date with the base.

## Releases

`release-please` opens its own branch, which this policy would reject on `main`. Releases are cut by hand, and GitHub Actions publishes the tag.

1. Branch `release/X.Y.Z` from `develop`. The version is the next SemVer. No `v` in the branch name.
2. On that branch, move `## [Unreleased]` notes in `CHANGELOG.md` under `## [X.Y.Z] - YYYY-MM-DD`. `git-cliff` can draft this when it is installed (`cliff.toml` at the repo root):

   ```bash
   git cliff --tag vX.Y.Z -o CHANGELOG.md
   ```

   Writing the heading by hand is enough. CI only checks that the heading is there.
3. Stabilize on the release branch. Fixes commit on `release/X.Y.Z` and can merge back to `develop` if `develop` has moved.
4. Open a pull request into `main`. Title example: `chore(release): 1.2.3`. Merge with **Create a merge commit**.
5. The Release workflow sees that merge, tags `vX.Y.Z` on the merge commit, and attaches:
   - manager binaries for linux, windows, and darwin, amd64 and arm64
   - `checksums.txt` for those binaries
   - `ipfs-panel-web_X.Y.Z_dist.zip` and `ipfs-panel-web_X.Y.Z_dist.zip.sha256`
6. If the compare from `main` to `develop` is not empty, open a back-merge pull request: base `develop`, head `main`. **Squash and merge** it. Title: `chore(release): back-merge vX.Y.Z`. `develop` allows squash only, so this cannot be a merge commit. If the compare is empty, skip the back-merge.

The tag is created by `GITHUB_TOKEN`. That push does not start a second workflow. The same job builds the artifacts. Pushing a tag yourself also works: the Release workflow runs on `vX.Y.Z` tags. The tag ruleset blocks moving or deleting `v*` tags. It does not block creating one.

Nightly builds from `develop` upload workflow artifacts only. They are not releases and they are not `v*` tags. They start after `develop` is the default branch, because scheduled workflows run from the default branch.

## Hotfixes

### Fast path, still a pull request

Use this unless production cannot wait for CI.

1. Branch `hotfix/X.Y.Z` from `main`. `X.Y.Z` is the new patch, not the broken version.
2. Fix, add `## [X.Y.Z]` to `CHANGELOG.md`, and open a pull request into `main`.
3. `ci-ok` has to be green. Approvals stay at 0, so a maintainer can merge as soon as CI is green. Merge with **Create a merge commit**. Do not skip CI.
4. The Release workflow tags `vX.Y.Z` and uploads the same assets as a release.
5. Back-merge into `develop` the same way as a release: squash, title `chore(release): back-merge vX.Y.Z`.

### Emergency direct push

If a `hotfix/*` pull request cannot be waited for (the ruleset or CI is itself the outage, or the fix has to land in minutes), a repository admin may push a commit straight to `main`. The ruleset bypass for that is repository admin, mode **Always**. It is an audit-log event, not a habit.

- Push a new commit. Do not force-push, and do not delete `main`.
- The Release workflow will not tag this commit, because there is no `release/*` or `hotfix/*` pull request. If the commit should ship, the admin creates an annotated tag and pushes it: `git tag -a vX.Y.Z -m vX.Y.Z && git push origin vX.Y.Z`. The tag push builds the assets. `CHANGELOG.md` on that commit must already contain `## [X.Y.Z]`, or the release job fails.
- The same day, open a [post-incident note](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=post_incident.yml). Name the SHA, why the hotfix pull request could not be used, who was affected, and the follow-up (back-merge, tag if missing, repair pull request).

`develop` has no bypass. Land the follow-up there with a pull request.

Who may do this, and when the approval count moves off 0, is in [Governance](GOVERNANCE.md).

## Local development

UI, from `apps/web`. Node 22.18 or newer, or Node 24.12 or newer.

```bash
npm ci
npm run dev
npm run lint
npm run typecheck
npm test
npm run build
```

Manager, from `services/manager`. Go 1.22.

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build -o /dev/null ./cmd/manager
go run ./cmd/manager
```

`golangci-lint` v2.15.0 matches CI (`services/manager/.golangci.yml`). Install the v2.15.0 binary from its releases. `gofmt` and `go vet` still run on their own.

Editor hint: VS Code with the official Vue extension (Volar) and the Go extension. Recommendations are in `.vscode/extensions.json`. Turn Vetur off if it is still installed. Do not commit `.idea/` or the rest of `.vscode/`.

## How CI works

The workflow is `.github/workflows/ci.yml`. It runs on pull requests and pushes to `main` and `develop`.

| Job | When it runs |
| --- | --- |
| `changes` | Always. Decides whether web or manager files changed. |
| `web` | `apps/web/**` or the CI workflow changed. `npm ci`, lint, typecheck, vitest, production build. |
| `manager` | `services/manager/**` or the CI workflow changed. `gofmt`, `go vet`, `golangci-lint`, `go test -race`, `go build`. |
| `pr-title` | Pull requests. Conventional Commits title. |
| `branch-policy` | Pull requests. The Git Flow rules above, plus the script's own tests. |
| `dependency-review` | Pull requests. Fails on high or critical new vulnerabilities. |
| `ci-ok` | Always, unless the run was cancelled. Passes when every job above succeeded or skipped. |

Branch protection requires **`ci-ok` only**. The other names are skipped on some events. A skipped required check blocks the merge. Do not add `web` or `manager` as their own required checks.

Other workflows, not required:

| Workflow | What it does |
| --- | --- |
| CodeQL | `codeql-go`, `codeql-javascript` on pull requests, pushes, and a weekly schedule. |
| Release | Tags and GitHub Release assets from `main`. |
| Nightly | Snapshot binaries and a web build from `develop`, kept 7 days. |
| Labels | Syncs `.github/labels.json` when that file changes on `main` or `develop`. |

Dependabot opens one grouped weekly pull request per ecosystem (npm, Go modules, GitHub Actions) against the default branch. Those branches use the `dependabot/` prefix and squash into `develop`.

## Product rules

Say which roadmap slice the change belongs to. Use "foundation" for repo hygiene that is not a product slice yet.

If you change UI copy, change `apps/web/src/locales/en.ts` and `apps/web/src/locales/zh.ts` in the same pull request. The keys match. `zh` is Simplified Chinese.

Anything that talks to Kubo goes through `services/manager`. Do not add a screen, a sample, or a paragraph that tells people to put Kubo RPC (port 5001) on a public address.

- UI: Vue 3, Vite, TypeScript, Vue Router, Pinia, vue-i18n.
- Manager: Go. Use the standard library. [chi](https://github.com/go-chi/chi) is fine once `ServeMux` is getting hard to read. GoFrame is not part of this project.
- A new framework needs a reason in the pull request, and a yes, before the code lands.

## Conduct and security

[Code of Conduct](CODE_OF_CONDUCT.md). [Security](SECURITY.md). [Support](SUPPORT.md). [Governance](GOVERNANCE.md).

---

# 参与方式

## 分支模型

长期分支只有 `main` 和 `develop`。

- `main` 是生产。上面的每次提交都应该可以发行，并打上 `vX.Y.Z`。只有 `release/X.Y.Z` 和 `hotfix/X.Y.Z` 的 pull request 可以指向它。
- `develop` 是集成分支，也是仓库的默认分支。功能从这里拉出，再合回这里。

没有第三条长期分支。开一个短期分支，提 pull request，合并，删掉分支。[组织设置](docs/ORG-SETUP.md) 做完之后，GitHub 会在合并后自动删掉头分支。

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

把这套政策加进来的那一次引导 pull request 指向 `main`，因为当时还没有 `develop`。合并之后，从 `main` 建出 `develop`，再把默认分支改成 `develop`。步骤在 [组织设置](docs/ORG-SETUP.md)。`origin/develop` 一旦存在，CI 会拒绝头分支不是 `release/X.Y.Z` 或 `hotfix/X.Y.Z` 的、指向 `main` 的 pull request。

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

谁可以这么做，以及批准人数什么时候离开 0，写在 [治理](GOVERNANCE.md)。

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

管理服务在 `services/manager`。Go 1.22。

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
| `dependency-review` | pull request。新引入高危或严重漏洞时失败。 |
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

和 Kubo 说话的代码放在 `services/manager`。不要加页面、示例或说明，去教别人把 Kubo RPC（5001 端口）开到公网地址上。

- 界面：Vue 3、Vite、TypeScript、Vue Router、Pinia、vue-i18n。
- 管理服务：Go。先用标准库。`ServeMux` 实在难读时可以用 [chi](https://github.com/go-chi/chi)。这个项目不用 GoFrame。
- 想再加一个框架，先在 pull request 里写理由，等同意了再把代码放进来。

## 行为和安全

[行为准则](CODE_OF_CONDUCT.md)。[安全](SECURITY.md)。[支持](SUPPORT.md)。[治理](GOVERNANCE.md)。
