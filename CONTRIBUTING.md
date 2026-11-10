# Contributing

**English** · [中文](CONTRIBUTING.zh-CN.md)

## Branch model

`main` and `develop` are the only long-lived branches.

- `main` is production. Every commit on it is releasable and tagged `vX.Y.Z`. Only `release/X.Y.Z` and `hotfix/X.Y.Z` pull requests may target it.
- `develop` is the integration branch. Feature work starts here and comes back here.
- **The repository default branch is still `main`.** `docs/ORG-SETUP.md` step 3 asks for it to become `develop`; until that is applied, anything that reads the default branch reads `main`. Dependabot is the live example: its `.github/dependabot.yml` is read from `main`, which is why the three `package-ecosystem` entries there carry `target-branch: develop`. Removing that override would re-open the problem it fixed.

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
6. If the compare from `main` to `develop` is not empty, carry those commits back. **Do not open a pull request with head `main`.** The `develop` ruleset sets `strict_required_status_checks_policy`, so a pull request must contain every commit already on its base branch. `develop` has commits `main` does not (anything merged after the last release), so a `main`-headed pull request can never satisfy that and stays "out of date with the base branch" permanently. #25 and #26 both died there.

   Instead, branch from `develop` and bring the missing content across:

   ```bash
   git checkout -b chore/back-merge-X.Y.Z origin/develop
   git checkout origin/main -- CHANGELOG.md    # and any other file main has
   git commit -m "chore(release): back-merge vX.Y.Z"
   ```

   Then open that branch into `develop` and **squash and merge** it. `develop` allows squash only.

   Check what is actually missing before you copy anything: `git log --oneline origin/develop..origin/main`. After a clean release the answer is usually just `CHANGELOG.md`, because everything else reached `develop` first. Copying a file wholesale is safe only when the source version contains everything the destination had; diff the entry lists rather than assuming.

   If the compare is empty, skip this step.

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

Manager, from `services/manager`. Go 1.26, as in `services/manager/go.mod`.

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
| `dependency-review` | Pull requests. Fails on high or critical new vulnerabilities once the dependency graph is enabled. Until then it logs that and stays green. |
| `ci-ok` | Always, unless the run was cancelled. Passes when every job above succeeded or skipped. |

Branch protection requires **`ci-ok` only**. The other names are skipped on some events. A skipped required check blocks the merge. Do not add `web` or `manager` as their own required checks.

Other workflows, not required:

| Workflow | What it does |
| --- | --- |
| CodeQL | `codeql-go`, `codeql-javascript` on pull requests, pushes, and a weekly schedule. |
| Release | Tags and GitHub Release assets from `main`. |
| Nightly | Snapshot binaries and a web build from `develop`, kept 7 days. |
| Labels | Syncs `.github/labels.json` when that file changes on `main` or `develop`. |

Dependabot opens one grouped weekly pull request per ecosystem (npm, Go modules, GitHub Actions). It reads `.github/dependabot.yml` from the default branch but honours `target-branch` inside it, so those pull requests land on `develop` because the configuration says so, not because of the default. Their branches use the `dependabot/` prefix and squash into `develop`.

## Product rules

Say which roadmap slice the change belongs to. Use "foundation" for repo hygiene that is not a product slice yet.

If you change UI copy, change `apps/web/src/locales/en.ts` and `apps/web/src/locales/zh.ts` in the same pull request. The keys match. `zh` is Simplified Chinese.

Every doc ships as two files: `NAME.md` in English, `NAME.zh-CN.md` in 中文. Change both in the same pull request. `CODE_OF_CONDUCT.md` is the one exception: the Contributor Covenant is the English text of record, with a short Chinese note at the top.

Anything that talks to Kubo goes through `services/manager`. Do not add a screen, a sample, or a paragraph that tells people to put Kubo RPC (port 5001) on a public address.

- UI: Vue 3, Vite, TypeScript, Vue Router, Pinia, vue-i18n.
- Manager: Go. Use the standard library. [chi](https://github.com/go-chi/chi) is fine once `ServeMux` is getting hard to read. GoFrame is not part of this project.
- A new framework needs a reason in the pull request, and a yes, before the code lands.

## Conduct and security

[Code of Conduct](CODE_OF_CONDUCT.md). [Security](SECURITY.md). [Support](SUPPORT.md). [Governance](GOVERNANCE.md).
