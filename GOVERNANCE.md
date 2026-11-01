# Governance

English below. 中文在后半部分。

## Current state

[@devxiaopo](https://github.com/devxiaopo) is the only maintainer and the only repository admin. Required approvals stay at **0** until a second maintainer is actually here. A pull request is still required. The rule is "open a pull request", not "find a second person".

This file is the policy. The clicks that enforce it are in [Org setup](docs/ORG-SETUP.md).

## Roles

**Maintainer.** Merge rights on `develop` and `main`. Updates [CODEOWNERS](.github/CODEOWNERS), this file, and the rulesets. Decides what lands, using [Vision](docs/VISION.md) and [Roadmap](docs/ROADMAP.md) as the product boundary.

**Repository admin.** A maintainer who can also change settings and, in an emergency only, push to `main` without a pull request. Today that is @devxiaopo. The bypass is not a second way to do normal work.

**Contributor.** Anyone who sends a pull request or an issue. Contributors do not need write access.

There is no steering committee and no vote. One maintainer decides. Disagreement is an issue or a pull request, and the maintainer says yes or no in the thread.

## How a change lands

Day-to-day work targets `develop`. Releases and urgent production fixes target `main`. Branch names, commit titles, and the merge button are in [Contributing](CONTRIBUTING.md).

Code owner review is requested and not required. With one maintainer, requiring it would block that maintainer's own pull requests.

Inbound contributions are accepted under the [MIT license](LICENSE). There is no separate CLA.

## Adding a maintainer

When a second person has been landing work here and is willing to share the on-call for releases:

1. Add them to [CODEOWNERS](.github/CODEOWNERS).
2. Give them write access. Give them admin only if they should be able to use the emergency `main` bypass.
3. In both the `main` and `develop` rulesets, set required approvals to **1**.
4. Turn on **Require review from Code Owners** in both rulesets.
5. Leave the `develop` bypass list empty.
6. Leave the `main` bypass as repository admin, mode **Always**, for emergencies only.
7. Note the change in this file.

Until that list is real, do not raise the approval count.

## Emergency push to main

Repository admins can bypass the `main` ruleset. That exists for a production incident when a `hotfix/*` pull request cannot wait. It is recorded in the audit log.

The same day, the admin opens a [post-incident issue](.github/ISSUE_TEMPLATE/post_incident.yml). The issue names the commit, why the hotfix pull request could not be used, and the follow-up: back-merge into `develop`, a `vX.Y.Z` tag if the release workflow did not create one, and any repair pull request.

`develop` has no bypass. An incident is not a reason to push straight to `develop`.

## Conduct

[Code of Conduct](CODE_OF_CONDUCT.md). Reports go to a private advisory, not a public issue.

---

# 治理

## 现在的状态

[@devxiaopo](https://github.com/devxiaopo) 是唯一的维护者，也是唯一的仓库管理员。在真正有第二名维护者之前，必须的批准人数保持 **0**。pull request 仍然要开。规矩是「先开 pull request」，不是「先找第二个人」。

这份文件是政策。真正挡住推送的点击在 [组织设置](docs/ORG-SETUP.md)。

## 角色

**维护者。** 可以合并进 `develop` 和 `main`。更新 [CODEOWNERS](.github/CODEOWNERS)、这份文件和 ruleset。以 [愿景](docs/VISION.md) 和 [路线图](docs/ROADMAP.md) 为产品边界，决定什么可以进来。

**仓库管理员。** 维护者里还可以改设置的人，并且只在紧急情况下可以不经 pull request 推到 `main`。现在是 @devxiaopo。绕过不是日常的第二条路。

**贡献者。** 提 pull request 或 issue 的人。贡献者不需要写权限。

没有委员会，也不投票。一名维护者做决定。不同意就开 issue 或 pull request，维护者在线程里说可以或不可以。

## 改动怎么进来

日常工作的目标分支是 `develop`。发行和紧急的生产修复目标是 `main`。分支名、提交标题、该点哪个合并按钮，写在 [参与方式](CONTRIBUTING.md)。

Code owner 会被请求审阅，但不是必须的。只有一名维护者时，如果把它设为必须，维护者自己的 pull request 就合并不了。

进来的贡献按 [MIT 许可证](LICENSE) 接受。没有另外的 CLA。

## 增加一名维护者

当第二个人已经在这里持续合并工作，并且愿意一起管发行：

1. 把这个人写进 [CODEOWNERS](.github/CODEOWNERS)。
2. 给写权限。只有这个人也应该能使用 `main` 的紧急绕过时，才给管理员。
3. 在 `main` 和 `develop` 两个 ruleset 里，把必须的批准人数改成 **1**。
4. 两个 ruleset 都打开 **Require review from Code Owners**。
5. `develop` 的绕过名单保持空白。
6. `main` 的绕过仍然只给仓库管理员，模式是 **Always**，只用于紧急情况。
7. 在这份文件里记下这次变化。

名单还不是真的之前，不要把批准人数调高。

## 紧急直接推到 main

仓库管理员可以绕过 `main` 的 ruleset。这是给生产事故用的：`hotfix/*` 的 pull request 实在等不了。审计日志会记下这次绕过。

同一天，管理员要开一份 [事后说明 issue](.github/ISSUE_TEMPLATE/post_incident.yml)。写上提交、为什么不能走 hotfix pull request，以及后续：回并到 `develop`、如果发行工作流没有打 `vX.Y.Z` 标签就补上，还有任何修补用的 pull request。

`develop` 没有绕过。出事也不是直接推 `develop` 的理由。

## 行为

[行为准则](CODE_OF_CONDUCT.md)。举报走私下通告，不走公开 issue。
