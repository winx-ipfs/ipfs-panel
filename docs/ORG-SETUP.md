# Org setup

English below. 中文在后半部分。

This is the short list for `winx-ipfs` while the org is still one public repo and a small set of people. The product rules live in [Contributing](../CONTRIBUTING.md). This file is how to turn the "pull request before `main`" rule on, and what to leave off on purpose.

## English

### What a small org needs right now

- **A pull request before anything reaches `main`.** That includes docs. There is no `develop` branch. [Contributing](../CONTRIBUTING.md) already says this. GitHub does not enforce it yet.
- **The check that already exists.** [.github/workflows/ci.yml](../.github/workflows/ci.yml) runs on every pull request. The job name is `web typecheck and build`. It has already passed on `main`. Require that job. Do not require the Cloudflare, Vercel, or Cursor checks that may appear on a commit. They are not this repo's test. Do not require a check named `go test` until a workflow job actually reports that name. `go test ./...` is still a local step.
- **Required approvals stay at 0.** One maintainer cannot wait for a second person who is not in the org. The rule is "open a pull request", not "find a reviewer".
- **No auto-assign on issues.** With one maintainer, assigning every new issue to that person is noise. The issue templates have no assignees. Leave them that way. Do not add an Action that assigns issues.
- **Discussions stay off.** Issues are enough while the people here are building the panel. A question that is really a bug belongs in an issue. Turn Discussions on later if users need a place that is not a bug report.

Optional, and it matches Contributing: automatically delete head branches after a merge, so feature branches do not pile up.

Organization-wide rulesets can wait. This public repo can carry its own ruleset. An org ruleset is a paid-plan feature and is the wrong place to click for a single repo.

### Branch protection was not applied

Tried on 2026-11-01 with the GitHub App token this environment uses. The token can push and open pull requests. It cannot administer the repo (`permissions.admin` is false). No ruleset is active that this token can see: `GET /repos/winx-ipfs/ipfs-panel/rulesets` returned `200` and `[]`.

| Call | Result |
| --- | --- |
| `POST /repos/winx-ipfs/ipfs-panel/rulesets` | `403` `Resource not accessible by integration` |
| `PUT /repos/winx-ipfs/ipfs-panel/branches/main/protection` | `403` `Resource not accessible by integration` |
| `POST /orgs/winx-ipfs/rulesets` | `403` `Resource not accessible by integration` |
| `GET /orgs/winx-ipfs/rulesets` | `403`, accepted permission `organization_administration=write` |

An org owner has to click the rest. Use a browser session that can see **Settings** on the repo. If Settings is missing, this is the wrong account.

### Clicks: ruleset on `main`

Do this on the repository, not under the organization's settings.

1. Open <https://github.com/winx-ipfs/ipfs-panel/settings/rules>.
2. If the page is missing, start at <https://github.com/winx-ipfs/ipfs-panel>, click **Settings**, then in the left sidebar under **Code, planning, and automation** click **Rulesets**.
3. Click **New ruleset**, then **New branch ruleset**.
4. **Ruleset name:** `main`.
5. **Enforcement status:** **Active**. If the control currently says **Disabled**, open it and choose **Active**. A disabled ruleset does not block pushes.
6. **Bypass list:** leave it empty. Do not click **Add bypass**. An empty list means org owners also need a pull request, which is what Contributing says.
7. **Target branches:** click **Add target**, then **Include default branch**. The default branch is `main`.
8. In the branch rules, check these and leave the other boxes alone:
   - **Restrict deletions**
   - **Block force pushes**
   - **Require a pull request before merging**
     - Required approvals: **0**
     - Leave unchecked: dismiss stale approvals, require code owners, require approval of the most recent push, require conversation resolution.
   - **Require status checks to pass** (the page may say **Require status checks before merging**)
     - Add a check named `web typecheck and build`. If the box shows `CI / web typecheck and build`, add that one. It is the same job.
     - Leave **Require branches to be up to date before merging** unchecked.
9. Click **Create**.

Check the work: open <https://github.com/winx-ipfs/ipfs-panel/rules>. The ruleset is listed and **Active**. A direct `git push origin main` is rejected. A pull request can still be merged by the maintainer without a second approval, and the web job must be green first.

### If the ruleset button is not there

Classic branch protection, same policy:

1. Open <https://github.com/winx-ipfs/ipfs-panel/settings/branches>.
2. Click **Add branch protection rule**, or **Add classic branch protection rule** if both exist. Use the classic one only when a ruleset is unavailable.
3. **Branch name pattern:** `main`.
4. Check **Require a pull request before merging**. Set required approvals to **0**. If the field will not go below 1, stop and use the ruleset steps above. A solo maintainer must not need a second account to merge.
5. Check **Require status checks to pass before merging**. Add `web typecheck and build` (or `CI / web typecheck and build` if that is the name the search box offers). Do not check "require branches to be up to date".
6. Check **Do not allow bypassing the above settings**. If the label is still **Include administrators**, check that. Same meaning: a direct push to `main` is rejected for admins too.
7. Do not check **Lock branch**. A lock blocks merges as well.
8. Save.

### The other two clicks, and the ones to skip

Delete merged branches:

1. Open <https://github.com/winx-ipfs/ipfs-panel/settings>.
2. Under **Pull Requests**, check **Automatically delete head branches**.

Discussions, leave off:

1. On that same General page, under **Features**, leave **Discussions** unchecked.

Skip for now:

- A second required reviewer, code owners, or an auto-requested review team.
- Any "assign new issues to" bot or workflow.
- A long-lived branch besides `main`.
- Marking deploy previews (Vercel, Cloudflare) or Cursor checks as required.

---

## 中文

这是 `winx-ipfs` 还只有一个公开仓库、人还很少时的短清单。产品上的规矩在 [参与方式](../CONTRIBUTING.md)。这份文档说明怎么把「进 `main` 之前必须走 pull request」真正打开，以及哪些开关是故意先不动的。

GitHub 网页上的按钮名是英文。下面中文说明旁边保留英文标签，按英文去点。

### 现在这个小组织需要什么

- **任何东西进 `main` 之前都要有 pull request。** 文档也一样。没有 `develop`。[参与方式](../CONTRIBUTING.md) 里已经这么写了。GitHub 还没有强制执行。
- **用已经存在的那一项检查。** [.github/workflows/ci.yml](../.github/workflows/ci.yml) 会在每个 pull request 上跑。任务名叫 `web typecheck and build`。它已经在 `main` 上成功过。把这一项设为必须通过。不要把一次提交上可能出现的 Cloudflare、Vercel 或 Cursor 检查设为必须。那不是这个仓库的测试。在真的有一个工作流任务上报 `go test` 这个名字之前，不要要求名为 `go test` 的检查。`go test ./...` 目前仍是本机要跑的一步。
- **必须的批准人数保持 0。** 只有一个维护者时，不能干等一个组织里还不存在的第二个人。规矩是「先开 pull request」，不是「先找一个审阅的人」。
- **issue 不要自动指派。** 只有一个维护者时，把每个新 issue 都派给这个人只是噪音。issue 模板里没有 assignee。保持这样就好。不要加一个自动指派 issue 的 Action。
- **Discussions 先关着。** 现在这里的人是在做面板，用 issue 就够。真是缺陷的问题放进 issue。等使用者需要一个不属于缺陷报告的地方时，再打开 Discussions。

可选，并且和参与方式一致：合并后自动删除头分支，避免功能分支越堆越多。

组织级的 ruleset 可以以后再说。这个公开仓库用自己的 ruleset 即可。组织级 ruleset 属于付费计划里的功能，一个仓库不该去组织设置里点。

### 分支保护没有通过 API 打开

2026-11-01 用本环境里的 GitHub App 令牌试过。这个令牌可以推送、可以开 pull request，不能管理仓库（`permissions.admin` 为 false）。它能看见的 ruleset 是空的：`GET /repos/winx-ipfs/ipfs-panel/rulesets` 返回 `200` 和 `[]`。

| 调用 | 结果 |
| --- | --- |
| `POST /repos/winx-ipfs/ipfs-panel/rulesets` | `403` `Resource not accessible by integration` |
| `PUT /repos/winx-ipfs/ipfs-panel/branches/main/protection` | `403` `Resource not accessible by integration` |
| `POST /orgs/winx-ipfs/rulesets` | `403` `Resource not accessible by integration` |
| `GET /orgs/winx-ipfs/rulesets` | `403`，所需权限是 `organization_administration=write` |

剩下的要点由组织所有者在网页上完成。用一个看得到仓库 **Settings** 的浏览器账号。看不到 Settings，就是账号不对。

### 点击步骤：给 `main` 加 ruleset

在仓库上操作，不要进组织的设置。

1. 打开 <https://github.com/winx-ipfs/ipfs-panel/settings/rules>。
2. 如果没有这个页面：打开 <https://github.com/winx-ipfs/ipfs-panel>，点 **Settings**，然后在左侧 **Code, planning, and automation** 下面点 **Rulesets**。
3. 点 **New ruleset**，再点 **New branch ruleset**。
4. **Ruleset name（规则集名称）：** `main`。
5. **Enforcement status（执行状态）：** **Active**。如果现在写的是 **Disabled**，点开并选 **Active**。停用的规则集拦不住推送。
6. **Bypass list（绕过名单）：** 留空。不要点 **Add bypass**。名单为空时，组织所有者也要走 pull request，这和参与方式里写的一样。
7. **Target branches（目标分支）：** 点 **Add target**，再点 **Include default branch**。默认分支就是 `main`。
8. 在分支规则里勾上下面这些，其他框先别动：
   - **Restrict deletions**（限制删除）
   - **Block force pushes**（阻止强制推送）
   - **Require a pull request before merging**（合并前必须有 pull request）
     - 需要的批准人数：**0**
     - 这些不要勾：驳回过期批准、要求 code owners、要求批准最近一次推送、要求解决对话。
   - **Require status checks to pass**（页面上也可能写成 **Require status checks before merging**，合并前状态检查必须通过）
     - 添加名为 `web typecheck and build` 的检查。如果框里显示的是 `CI / web typecheck and build`，就加那一个。是同一个任务。
     - 不要勾 **Require branches to be up to date before merging**（合并前分支必须更新到最新）。
9. 点 **Create**。

核对：打开 <https://github.com/winx-ipfs/ipfs-panel/rules>。规则集在列表里，并且是 **Active**。直接 `git push origin main` 会被拒绝。维护者仍然可以在没有第二个人批准的情况下合并 pull request，但网页上的那一项检查必须先是绿的。

### 如果没有 ruleset 按钮

用经典的分支保护，政策相同：

1. 打开 <https://github.com/winx-ipfs/ipfs-panel/settings/branches>。
2. 点 **Add branch protection rule**。如果旁边还有 **Add classic branch protection rule**，只有在不能建 ruleset 时才用经典这条。
3. **Branch name pattern（分支名模式）：** `main`。
4. 勾上 **Require a pull request before merging**。需要的批准人数设为 **0**。如果这个数字不能低于 1，停下来，改走上面的 ruleset。只有一个维护者时，不该再需要第二个账号才能合并。
5. 勾上 **Require status checks to pass before merging**。添加 `web typecheck and build`（搜索框若给出 `CI / web typecheck and build`，就用那个名字）。不要勾「分支必须更新到最新」。
6. 勾上 **Do not allow bypassing the above settings**。如果标签还是 **Include administrators**，勾那个。意思相同：管理员直接推到 `main` 也会被拒绝。
7. 不要勾 **Lock branch**。锁住之后合并也会被挡住。
8. 保存。

### 另外两点，以及先不要做的

删除已合并的分支：

1. 打开 <https://github.com/winx-ipfs/ipfs-panel/settings>。
2. 在 **Pull Requests** 下面勾上 **Automatically delete head branches**。

Discussions 保持关闭：

1. 在同一页的 General 里，**Features** 下面不要勾 **Discussions**。

现在先不要做：

- 第二名必须的审阅者、code owners，或自动请求某个团队审阅。
- 任何「把新 issue 派给某人」的机器人或工作流。
- 除 `main` 以外的长期分支。
- 把部署预览（Vercel、Cloudflare）或 Cursor 的检查设为必须通过。
