## What

## Slice

Foundation, or roadmap slice 1–6.

## Verification

Required, and CI fails without it. AGENTS.md asks for three things in this order; put the commands and their real output here.

1. **The product runs.** Start it, exercise the changed path, paste the output. A green suite is not this step.
2. **The suite is green.** The exact commands from README.md.
3. **The claim matches.** If you say a stopped node returns `node_not_running`, run it while stopped and show the response.

```text
$ <command>
<output>
```

## Checklist

- [ ] UI copy is updated in both `en` and `zh`, or there is no UI copy change
- [ ] Kubo RPC is still not exposed
- [ ] The head branch uses a lowercase prefix from [Contributing](../CONTRIBUTING.md#branch-names)
- [ ] Into `develop` from `feature/`, `fix/`, `chore/`, `docs/`, `refactor/`, `test/`, `ci/`, or `dependabot/`: I will **Squash and merge**, and the title is a [Conventional Commit](https://www.conventionalcommits.org/)
- [ ] Into `main` from `release/X.Y.Z` or `hotfix/X.Y.Z`: I will **Create a merge commit**, and `CHANGELOG.md` has `## [X.Y.Z]`
- [ ] Back-merge from `main` into `develop`: I branched from `develop` and copied the missing commits, because a `main`-headed pull request can never satisfy the up-to-date rule. I will **Squash and merge** with title `chore(release): back-merge vX.Y.Z`

## Merge button

| Head | Base | Button |
| --- | --- | --- |
| `feature/` `fix/` `chore/` `docs/` `refactor/` `test/` `ci/` `dependabot/` | `develop` | Squash and merge |
| `chore/back-merge-X.Y.Z` (carries what `main` has) | `develop` | Squash and merge |
| `release/X.Y.Z` `hotfix/X.Y.Z` | `main` | Create a merge commit |
