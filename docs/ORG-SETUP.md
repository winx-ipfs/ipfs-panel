# Org setup

**English** · [中文](ORG-SETUP.zh-CN.md)

This is the settings checklist for `winx-ipfs/ipfs-panel`. Apply it in the GitHub web UI after the governance pull request is on `main`. A CI token cannot change these settings (`permissions.admin` is false). Do not also add a classic branch protection rule. Rulesets and classic protection both apply, and the two together are easy to get wrong.

Product rules live in [Contributing](../CONTRIBUTING.md). Who may bypass `main` lives in [Governance](../GOVERNANCE.md).

The old required check `web typecheck and build` is retired. Do not require it.

## Order

Do these in order. Dependabot and the nightly workflow assume `develop` is the default branch.

1. Merge the governance pull request into `main`. It is allowed to target `main` only because `develop` does not exist yet.
2. Create `develop` from that `main`.
3. Set the default branch to `develop`.
4. Set the pull request merge buttons.
5. Create the three rulesets below (`main`, `develop`, `v-tags`).
6. Set Actions permissions.
7. Turn on the security switches.
8. Confirm the only required status check is `ci-ok`.

## 1. Create `develop` and make it the default

From a clone that has the merged `main`:

```bash
git fetch origin main
git push origin origin/main:refs/heads/develop
```

Or in the UI: open the branch dropdown on `main`, type `develop`, and create it from `main`.

Then:

1. Open <https://github.com/winx-ipfs/ipfs-panel/settings>.
2. Under **Default branch**, click the swap button and choose `develop`.
3. Confirm.

`main` stays. It is no longer the default. New pull requests and Dependabot target `develop`.

## 2. Pull request settings

On <https://github.com/winx-ipfs/ipfs-panel/settings>, under **Pull Requests**:

| Setting | Value |
| --- | --- |
| Allow merge commits | On |
| Allow squash merging | On |
| Allow rebase merging | Off |
| Default squash commit message | **Pull request title** |
| Default merge commit message | **Default message** |
| Automatically delete head branches | On |

Under **Features**, leave **Discussions** off.

You need both squash and merge at the repository level. The rulesets below pick which one a base branch accepts. Rebase stays off so nobody lands a rebase merge by habit.

The Release workflow does not parse the merge commit message. It reads the merged pull request from the API. Leave the default merge message anyway, so the history still says which pull request landed.

## 3. Ruleset `main`

1. Open <https://github.com/winx-ipfs/ipfs-panel/settings/rules>.
2. **New ruleset** → **New branch ruleset**.
3. **Ruleset name:** `main`.
4. **Enforcement status:** **Active**.
5. **Bypass list:** **Add bypass**. Add the repository role **Admin** (the picker may say **Repository admin**). Set the mode to **Always**.
   - Do not choose **For pull requests only**. That mode cannot push a commit without a pull request, and the emergency hotfix needs a direct push.
   - Do not add Organization admin, Maintain, Write, or a GitHub App.
6. **Target branches:** **Add target** → **Include by pattern**. Pattern: `main`.
   - Do not use **Include default branch**. After step 1 the default branch is `develop`.
7. Rules, check these and leave the others off:

**Restrict deletions.** On.

**Block force pushes.** On.

**Require linear history.** Off. A release or hotfix lands as a merge commit. This rule would reject that commit.

**Require a pull request before merging.** On.

- Required approvals: **0**
- **Dismiss stale pull request approvals when new commits are pushed:** On
- **Require review from Code Owners:** Off. [CODEOWNERS](../.github/CODEOWNERS) still requests a review. Requiring it blocks the only maintainer's own pull requests. Turn it on when [Governance](../GOVERNANCE.md) adds a second maintainer, and raise approvals to 1 at the same time.
- **Require approval of the most recent reviewable push:** Off. With one maintainer this deadlocks.
- **Require conversation resolution before merging:** On
- **Allowed merge methods:** **Merge** only. Uncheck Squash and Rebase.
  - If this control is not on the page, the repository settings in step 2 still allow both buttons. Maintainers must use **Create a merge commit** for `main`. Do not turn linear history on to compensate.
- If you see **Require an additional approval for unattributed Copilot pull requests**, leave the default. It does nothing while approvals are 0.

**Require status checks to pass.** On.

- **Require branches to be up to date before merging:** On
- Add exactly one check: `ci-ok`
  - The picker may show `CI / ci-ok`. That is the same job. Add that one if `ci-ok` alone is not listed.
  - Do not add `web`, `manager`, `changes`, `docs`, `pr-body`, `pr-title`, `branch-policy`, `dependency-review`, or `integration`. Those skip on some events. `integration` runs only on a pull request into `develop`. A skipped required check blocks the merge.
  - Do not add `codeql-go`, `codeql-javascript`, `release`, `nightly-artifacts`, or `sync-labels`.
  - Do not add `web typecheck and build`. That job no longer exists.
  - Source of the check: any source. Do not pin an app until you mean to.

Leave off: Restrict creations, Restrict updates, Require signed commits, Require merge queue, Require deployments, Lock branch.

8. **Create**.

## 4. Ruleset `develop`

1. **New ruleset** → **New branch ruleset**.
2. **Ruleset name:** `develop`.
3. **Enforcement status:** **Active**.
4. **Bypass list:** empty. Do not add repository admins. An empty list means admins also need a pull request. An incident is not a reason to push straight to `develop`.
5. **Target branches:** **Include by pattern**. Pattern: `develop`. Not "include default branch", so a later default-branch change cannot move this ruleset onto `main`.
6. Rules:

**Restrict deletions.** On.

**Block force pushes.** On.

**Require linear history.** On. Together with squash-only, `develop` stays a straight line. Back-merges from `main` are squash commits (`chore(release): back-merge vX.Y.Z`), so they fit this rule.

**Require a pull request before merging.** On.

- Required approvals: **0**
- **Dismiss stale pull request approvals when new commits are pushed:** On
- **Require review from Code Owners:** Off, until a second maintainer exists
- **Require approval of the most recent reviewable push:** Off
- **Require conversation resolution before merging:** On
- **Allowed merge methods:** **Squash** only. Uncheck Merge and Rebase.
  - Squash-only is why the back-merge cannot be a merge commit. Git will not record a merge parent. The next release is cut from `develop` before it merges to `main`, and the back-merge only needs to carry hotfix and changelog commits that landed on `main`. If that starts to conflict, add **Merge** to this list and keep **Squash** checked. Do not remove linear history unless you also allow merge commits on purpose. Linear history rejects merge commits even if the button is enabled.
  - If the allowed-merge-methods control is missing, linear history still forces a squash or a rebase, and rebase is off at the repository level, so only squash works.

**Require status checks to pass.** On.

- **Require branches to be up to date before merging:** On
- The same single check: `ci-ok` (or `CI / ci-ok` if that is what the picker shows). Nothing else.
- **Consequence, and it is not obvious:** with this rule on, a pull request must contain every commit already on its base branch. `develop` normally has commits `main` does not, so a pull request with head `main` and base `develop` can never become up to date and can never merge. That is why [Contributing](../CONTRIBUTING.md#releases) step 6 carries the back-merge across on a `chore/back-merge-X.Y.Z` branch instead. Turning this rule off for `develop` would make the `main`-headed route work; it is on because the alternative silently allows merging a branch that is missing commits from its base.

Leave off the same extra rules as `main` (no signed commits, no merge queue, no lock).

7. **Create**.

## 5. Ruleset `v-tags`

1. **New ruleset** → **New tag ruleset**.
2. **Ruleset name:** `v-tags`.
3. **Enforcement status:** **Active**.
4. **Bypass list:** repository role **Admin**, mode **Always**. This is only so a tag that was created by mistake can be deleted before it is announced. Do not move a tag that people have already installed. Ship a new patch instead.
5. **Target tags:** **Add target** → **Include by pattern**. Pattern: `v*`.
6. Rules:
   - **Restrict deletions:** On
   - **Block force pushes:** On
   - **Restrict creations:** Off. The Release workflow creates tags with `GITHUB_TOKEN`. Restricting creation would block that token unless the Actions app were on the bypass list. Leave creation open. Write access is the maintainer, and the workflow.
   - **Require signed commits:** Off
7. **Create**.

A new `vX.Y.Z` tag can be created. It cannot be moved or deleted unless an admin bypasses this ruleset.

## 6. Actions permissions

Open <https://github.com/winx-ipfs/ipfs-panel/settings/actions>.

**General:**

| Setting | Value |
| --- | --- |
| Actions permissions | **Allow all actions and reusable workflows** |
| Fork pull request workflows from outside collaborators | **Require approval for first-time contributors** |
| Workflow permissions | **Read repository contents and packages permissions** |
| Allow GitHub Actions to create and approve pull requests | Off |

Workflows that publish a release set `permissions: contents: write` on themselves. That overrides the read default for that workflow only. The checkbox about creating pull requests stays off. Nothing in this repo opens a pull request from Actions.

An allow-list of individual actions is tighter, and it is not worth the outage the first week. Revisit it after Dependabot has bumped the pinned majors once.

**Artifact and log retention:** leave the default. Nightly artifacts set their own 7-day retention.

## 7. Code security

Open <https://github.com/winx-ipfs/ipfs-panel/settings/security_analysis> (the sidebar may say **Code security** or **Code security and analysis**).

| Setting | Value |
| --- | --- |
| Dependency graph | On |
| Dependabot alerts | On |
| Dependabot security updates | On |
| Code scanning / CodeQL default setup | Off. `.github/workflows/codeql.yml` is the setup. Do not add a second default setup on top of it. |
| Secret scanning | On |
| Push protection | On |
| Private vulnerability reporting | On |

Dependabot version updates are configured in [dependabot.yml](../.github/dependabot.yml): npm in `apps/web`, Go modules in `services/manager`, and GitHub Actions at the repo root. Each ecosystem opens one grouped pull request a week, against the default branch, with a Conventional Commit title (`chore(deps): ...`). Turn the default branch into `develop` before the Monday run, or those pull requests target `main` and `branch-policy` rejects them once `develop` exists.

The `dependency-review` job calls the dependency graph API first. While that API returns 403 or 404, the job logs that the graph is off and stays green, so `ci-ok` is not blocked before this checklist is applied. After the graph is on, the same job fails the pull request when a new dependency is high or critical.

Secret scanning and push protection are available on public repositories. If a toggle is missing, the plan does not offer it. Do not block the rest of this checklist on it. Write down which toggle was missing.

Private vulnerability reporting is what [Security](../SECURITY.md) links to. Turn it on before telling people to use the advisory form.

## 8. Confirm

- <https://github.com/winx-ipfs/ipfs-panel/rules> lists `main`, `develop`, and `v-tags`, all **Active**.
- The default branch is `develop`.
- A direct `git push origin develop` is rejected.
- A direct `git push origin main` is rejected for a non-admin. An admin can push, and must then open a post-incident issue.
- On a pull request, the `dependency-review` log runs the review action. It should not say the dependency graph is off.
- A pull request into `develop` can be squash-merged by the maintainer with 0 approvals when `ci-ok` is green and the branch is up to date.
- A pull request into `main` from `feature/*` fails `branch-policy`. One from `release/X.Y.Z` or `hotfix/X.Y.Z` can use **Create a merge commit** when `ci-ok` is green and `CHANGELOG.md` contains `## [X.Y.Z]`.
- Deleting `main` or `develop`, or force-pushing either, is rejected.
- Moving or deleting a `v*` tag is rejected. Creating a new `vX.Y.Z` tag works.

## Status checks this repository produces

Require `ci-ok` only.

| Workflow | Job name | Required |
| --- | --- | --- |
| CI | `ci-ok` | Yes |
| CI | `changes`, `docs`, `pr-body`, `web`, `manager`, `pr-title`, `branch-policy`, `dependency-review`, `integration` | No. They feed `ci-ok`. |
| CodeQL | `codeql-go`, `codeql-javascript` | No |
| Release | `release` | No. It runs after the merge to `main`, or on a tag push. |
| Nightly | `nightly-artifacts` | No |
| Labels | `sync-labels` | No |

## Later, not now

- A second required reviewer, or code owner reviews, before a second maintainer is in [Governance](../GOVERNANCE.md).
- Signed commits.
- A merge queue.
- Organization-wide rulesets. One public repository does not need them, and org rulesets are not the free-plan path.
- An "assign new issues to" bot. The issue forms have no assignees.
- Discussions.
- Treating Vercel, Cloudflare, or Cursor checks as required. They are not this repository's test.
