## What

## Slice

Foundation, or roadmap slice 1–6.

## Checklist

- [ ] UI copy is updated in both `en` and `zh`, or there is no UI copy change
- [ ] Kubo RPC is still not exposed
- [ ] The head branch uses a lowercase prefix from [Contributing](../CONTRIBUTING.md#branch-names)
- [ ] Into `develop` from `feature/`, `fix/`, `chore/`, `docs/`, `refactor/`, `test/`, `ci/`, or `dependabot/`: I will **Squash and merge**, and the title is a [Conventional Commit](https://www.conventionalcommits.org/)
- [ ] Into `main` from `release/X.Y.Z` or `hotfix/X.Y.Z`: I will **Create a merge commit**, and `CHANGELOG.md` has `## [X.Y.Z]`
- [ ] Back-merge from `main` into `develop`: I will **Squash and merge** with title `chore(release): back-merge vX.Y.Z`

## Merge button

| Head | Base | Button |
| --- | --- | --- |
| `feature/` `fix/` `chore/` `docs/` `refactor/` `test/` `ci/` `dependabot/` | `develop` | Squash and merge |
| `main` | `develop` | Squash and merge |
| `release/X.Y.Z` `hotfix/X.Y.Z` | `main` | Create a merge commit |
