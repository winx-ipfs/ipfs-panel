# 组织设置

[English](ORG-SETUP.md) · **中文**

这是 `winx-ipfs/ipfs-panel` 的设置清单。治理 pull request 进入 `main` 之后，在 GitHub 网页上逐项打开。CI 用的令牌改不了这些设置（`permissions.admin` 为 false）。不要再加一套经典的分支保护。ruleset 和经典保护会一起生效，两套叠在一起很容易设错。

产品上的规矩在 [参与方式](../CONTRIBUTING.zh-CN.md)。谁可以绕过 `main` 写在 [治理](../GOVERNANCE.zh-CN.md)。

旧的必须检查 `web typecheck and build` 已经不用了。不要再要求它。

网页上的按钮名是英文。下面中文说明旁边保留英文标签，按英文去点。

## 顺序

按这个顺序做。Dependabot 和夜间工作流假定默认分支已经是 `develop`。

1. 把治理 pull request 合并进 `main`。它现在可以指向 `main`，只是因为 `develop` 还不存在。
2. 从那个 `main` 建出 `develop`。
3. 把默认分支改成 `develop`。
4. 设置 pull request 的合并按钮。
5. 建下面三个 ruleset（`main`、`develop`、`v-tags`）。
6. 设置 Actions 权限。
7. 打开安全开关。
8. 确认唯一的必须状态检查是 `ci-ok`。

## 1. 建出 `develop`，并把它设为默认分支

在已经包含合并后 `main` 的克隆里：

```bash
git fetch origin main
git push origin origin/main:refs/heads/develop
```

或者在网页上：在 `main` 的分支下拉里输入 `develop`，从 `main` 创建。

然后：

1. 打开 <https://github.com/winx-ipfs/ipfs-panel/settings>。
2. 在 **Default branch** 下面点交换按钮，选 `develop`。
3. 确认。

`main` 还在。它不再是默认分支。新的 pull request 和 Dependabot 会指向 `develop`。

## 2. Pull request 设置

在 <https://github.com/winx-ipfs/ipfs-panel/settings> 的 **Pull Requests** 下面：

| 设置 | 值 |
| --- | --- |
| Allow merge commits | 开 |
| Allow squash merging | 开 |
| Allow rebase merging | 关 |
| Default squash commit message | **Pull request title** |
| Default merge commit message | **Default message** |
| Automatically delete head branches | 开 |

**Features** 下面不要勾 **Discussions**。

仓库这一层需要同时允许 squash 和 merge。下面的 ruleset 决定每个目标分支接受哪一种。Rebase 保持关闭，避免有人习惯性用 rebase 合并。

Release 工作流不解析合并提交的说明。它从 API 读取已合并的 pull request。默认的合并说明还是留着，历史里才能看出是哪一个 pull request 进来的。

## 3. Ruleset `main`

1. 打开 <https://github.com/winx-ipfs/ipfs-panel/settings/rules>。
2. **New ruleset** → **New branch ruleset**。
3. **Ruleset name（规则集名称）：** `main`。
4. **Enforcement status（执行状态）：** **Active**。
5. **Bypass list（绕过名单）：** 点 **Add bypass**。加上仓库角色 **Admin**（选择器里也可能写成 **Repository admin**）。模式设为 **Always**。
   - 不要选 **For pull requests only**。那种模式不能在没有 pull request 的情况下推提交，紧急热修需要直接推。
   - 不要加 Organization admin、Maintain、Write，或某个 GitHub App。
6. **Target branches（目标分支）：** **Add target** → **Include by pattern**。模式：`main`。
   - 不要用 **Include default branch**。做完第 1 步之后，默认分支是 `develop`。
7. 规则里勾上下面这些，其他的保持关闭：

**Restrict deletions（限制删除）。** 开。

**Block force pushes（阻止强制推送）。** 开。

**Require linear history（要求线性历史）。** 关。发行和热修是以 merge commit 进来的。这条规则会拒绝那个提交。

**Require a pull request before merging（合并前必须有 pull request）。** 开。

- 需要的批准人数：**0**
- **Dismiss stale pull request approvals when new commits are pushed（有新提交时驳回过期的批准）：** 开
- **Require review from Code Owners：** 关。[CODEOWNERS](../.github/CODEOWNERS) 仍然会请求审阅。把它设为必须，会挡住唯一维护者自己的 pull request。等 [治理](../GOVERNANCE.zh-CN.md) 里有了第二名维护者再打开，同时把批准人数改成 1。
- **Require approval of the most recent reviewable push：** 关。只有一名维护者时，这条会卡死。
- **Require conversation resolution before merging（合并前必须解决对话）：** 开
- **Allowed merge methods（允许的合并方式）：** 只留 **Merge**。取消 Squash 和 Rebase。
  - 如果页面上没有这个控件，第 2 步的仓库设置仍然两个按钮都允许。维护者在 `main` 上必须点 **Create a merge commit**。不要为了补这个缺口去打开线性历史。
- 如果看到 **Require an additional approval for unattributed Copilot pull requests**，保持默认。批准人数是 0 时它不起作用。

**Require status checks to pass（状态检查必须通过）。** 开。

- **Require branches to be up to date before merging（合并前分支必须更新到最新）：** 开
  - **后果，而且不明显：** 开了这条，pull request 必须包含目标分支上已有的每一个提交。`develop` 通常有 `main` 没有的提交，所以「头是 `main`、目标是 `develop`」的 pull request 永远更新不到最新，也就永远合不了。这就是 [参与方式](../CONTRIBUTING.zh-CN.md#发行) 第 6 步改成用 `chore/back-merge-X.Y.Z` 分支搬运回并内容的原因。给 `develop` 关掉这条能让「头是 `main`」那条路走通；之所以开着，是因为关掉等于默许合入一个缺少目标分支提交的分支。
- 只加一个检查：`ci-ok`
  - 选择器里可能显示 `CI / ci-ok`。那是同一个任务。如果单独搜不到 `ci-ok`，就加那一个。
  - 不要加 `web`、`manager`、`changes`、`docs`、`pr-body`、`pr-title`、`branch-policy`、`dependency-review` 或 `integration`。它们在有些事件上会被跳过。`integration` 只在指向 `develop` 的 pull request 上跑。一个被跳过的必须检查会挡住合并。
  - 不要加 `codeql-go`、`codeql-javascript`、`release`、`nightly-artifacts` 或 `sync-labels`。
  - 不要加 `web typecheck and build`。那个任务已经不存在了。
  - 检查来源：任意来源。不要在你还没打算锁定某个应用时去钉一个 app。

保持关闭：Restrict creations、Restrict updates、Require signed commits、Require merge queue、Require deployments、Lock branch。

8. 点 **Create**。

## 4. Ruleset `develop`

1. **New ruleset** → **New branch ruleset**。
2. **Ruleset name：** `develop`。
3. **Enforcement status：** **Active**。
4. **Bypass list：** 空。不要加仓库管理员。名单为空时，管理员也要走 pull request。出事也不是直接推 `develop` 的理由。
5. **Target branches：** **Include by pattern**。模式：`develop`。不要用「包含默认分支」，免得以后默认分支一改，这条规则跑到 `main` 上去。
6. 规则：

**Restrict deletions。** 开。

**Block force pushes。** 开。

**Require linear history。** 开。加上只允许 squash，`develop` 会保持一条直线。从 `main` 回并是 squash 提交（`chore(release): back-merge vX.Y.Z`），符合这条规则。

**Require a pull request before merging。** 开。

- 需要的批准人数：**0**
- **Dismiss stale pull request approvals when new commits are pushed：** 开
- **Require review from Code Owners：** 关，直到有第二名维护者
- **Require approval of the most recent reviewable push：** 关
- **Require conversation resolution before merging：** 开
- **Allowed merge methods：** 只留 **Squash**。取消 Merge 和 Rebase。
  - 因为只允许 squash，回并不能是 merge commit。Git 不会记下合并父提交。下一次发行是先从 `develop` 切出去，再合进 `main`。回并只需要把落在 `main` 上的热修和变更日志带回来。如果这开始产生冲突，把 **Merge** 加进这个列表，并保持 **Squash** 勾着。除非你有意允许 merge commit，否则不要关掉线性历史。线性历史即使按钮开着，也会拒绝 merge commit。
  - 如果页面上没有「允许的合并方式」，线性历史仍然会强制 squash 或 rebase，而仓库层面 rebase 是关的，所以只剩 squash。

**Require status checks to pass。** 开。

- **Require branches to be up to date before merging：** 开
- 同样只加一个检查：`ci-ok`（选择器若显示 `CI / ci-ok`，就用那个）。不要加别的。

和 `main` 一样，不要打开签名提交、合并队列、锁定分支。

7. 点 **Create**。

## 5. Ruleset `v-tags`

1. **New ruleset** → **New tag ruleset**。
2. **Ruleset name：** `v-tags`。
3. **Enforcement status：** **Active**。
4. **Bypass list：** 仓库角色 **Admin**，模式 **Always**。这只为了在宣布之前删掉一个误打的标签。不要移动别人已经安装过的标签。改发一个新的补丁。
5. **Target tags：** **Add target** → **Include by pattern**。模式：`v*`。
6. 规则：
   - **Restrict deletions：** 开
   - **Block force pushes：** 开
   - **Restrict creations：** 关。Release 工作流用 `GITHUB_TOKEN` 创建标签。如果限制创建，这个令牌会被挡住，除非把 Actions 应用加进绕过名单。创建保持开放。有写权限的是维护者，以及这个工作流。
   - **Require signed commits：** 关
7. 点 **Create**。

新的 `vX.Y.Z` 标签可以创建。除非管理员绕过这条 ruleset，否则不能移动或删除。

## 6. Actions 权限

打开 <https://github.com/winx-ipfs/ipfs-panel/settings/actions>。

**General：**

| 设置 | 值 |
| --- | --- |
| Actions permissions | **Allow all actions and reusable workflows** |
| Fork pull request workflows from outside collaborators | **Require approval for first-time contributors** |
| Workflow permissions | **Read repository contents and packages permissions** |
| Allow GitHub Actions to create and approve pull requests | 关 |

真正发布的工作流自己写了 `permissions: contents: write`。那只抬高那一个工作流的权限。让 Actions 创建 pull request 的那个勾选保持关闭。这个仓库里没有工作流会自己开 pull request。

逐个 action 做允许名单更紧，但第一周不值得为此让 CI 挂掉。等 Dependabot 把钉住的主版本升过一次之后再考虑。

**Artifact and log retention：** 保持默认。夜间附件自己设了 7 天保留。

## 7. 代码安全

打开 <https://github.com/winx-ipfs/ipfs-panel/settings/security_analysis>（侧栏也可能写成 **Code security** 或 **Code security and analysis**）。

| 设置 | 值 |
| --- | --- |
| Dependency graph | 开 |
| Dependabot alerts | 开 |
| Dependabot security updates | 开 |
| Code scanning / CodeQL 的默认设置 | 关。`.github/workflows/codeql.yml` 就是设置。不要在它上面再加一套默认设置。 |
| Secret scanning | 开 |
| Push protection | 开 |
| Private vulnerability reporting | 开 |

Dependabot 的版本更新写在 [dependabot.yml](../.github/dependabot.yml)：`apps/web` 的 npm、`services/manager` 的 Go 模块，以及仓库根目录的 GitHub Actions。每个生态系统每周向默认分支开一个分组的 pull request，标题是 Conventional Commit（`chore(deps): ...`）。周一的运行之前把默认分支改成 `develop`，否则这些 pull request 会指向 `main`，而 `develop` 一旦存在，`branch-policy` 会拒绝它们。

`dependency-review` 会先调用依赖图 API。API 返回 403 或 404 时，任务只记下「图还没开」，并保持绿色，这样在这份清单做完之前不会挡住 `ci-ok`。图打开之后，同一个任务会在新依赖是高危或严重时让 pull request 失败。

公开仓库可以用 secret scanning 和 push protection。如果某个开关不存在，就是当前计划没有这项。不要为了它卡住这份清单的其余部分。记下缺了哪个开关。

私下漏洞报告就是 [安全](../SECURITY.zh-CN.md) 里的那个链接。在告诉别人使用通告表单之前，先把它打开。

## 8. 核对

- <https://github.com/winx-ipfs/ipfs-panel/rules> 上有 `main`、`develop`、`v-tags`，并且都是 **Active**。
- 默认分支是 `develop`。
- 直接 `git push origin develop` 会被拒绝。
- 非管理员直接 `git push origin main` 会被拒绝。管理员可以推，并且必须随后开一份事后说明 issue。
- 在一个 pull request 上，`dependency-review` 的日志会真正跑审阅。它不应该再说依赖图是关着的。
- 指向 `develop` 的 pull request，在 `ci-ok` 为绿且分支已更新时，维护者可以在 0 个批准的情况下 squash 合并。
- 从 `feature/*` 指向 `main` 的 pull request 会在 `branch-policy` 失败。从 `release/X.Y.Z` 或 `hotfix/X.Y.Z` 来的，在 `ci-ok` 为绿且 `CHANGELOG.md` 含有 `## [X.Y.Z]` 时，可以用 **Create a merge commit**。
- 删除 `main` 或 `develop`，或对它们强推，会被拒绝。
- 移动或删除 `v*` 标签会被拒绝。新建一个 `vX.Y.Z` 标签可以。

## 这个仓库会产生的状态检查

只把 `ci-ok` 设为必须。

| 工作流 | 任务名 | 是否必须 |
| --- | --- | --- |
| CI | `ci-ok` | 是 |
| CI | `changes`、`docs`、`pr-body`、`web`、`manager`、`pr-title`、`branch-policy`、`dependency-review`、`integration` | 否。它们汇入 `ci-ok`。 |
| CodeQL | `codeql-go`、`codeql-javascript` | 否 |
| Release | `release` | 否。它在合并进 `main` 之后跑，或在推送标签时跑。 |
| Nightly | `nightly-artifacts` | 否 |
| Labels | `sync-labels` | 否 |

## 现在先不要做

- 在 [治理](../GOVERNANCE.zh-CN.md) 有第二名维护者之前，不要要求第二名审阅者或 code owner 审阅。
- 签名提交。
- 合并队列。
- 组织级 ruleset。一个公开仓库不需要，而且组织级 ruleset 不是免费计划上该走的路。
- 「把新 issue 派给某人」的机器人。issue 表单里没有 assignee。
- Discussions。
- 把 Vercel、Cloudflare 或 Cursor 的检查设为必须。那不是这个仓库的测试。
