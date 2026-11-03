# Governance

**English** · [中文](GOVERNANCE.zh-CN.md)

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
