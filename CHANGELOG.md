# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Releases are tagged from `main` only. The heading `## [X.Y.Z]` has to be in this file on the `release/X.Y.Z` or `hotfix/X.Y.Z` pull request. See [Contributing](CONTRIBUTING.md#releases).

## [0.1.1] - 2026-11-09

Two fixes that only matter for how the repository is maintained, not for the panel itself.

- **Dependabot targets `develop`.** GitHub reads `.github/dependabot.yml` from the default branch, which here is `main`. The configuration on `develop` is never read, so every dependency pull request it opened targeted `main` and was rejected by `branch-policy`, which only allows `release/X.Y.Z` and `hotfix/X.Y.Z` into `main`. The three `package-ecosystem` entries now carry `target-branch: develop`.
- **`minimatch` is pinned to 11.0.1** through an `overrides` entry. The old 10.2.6 has an attacker-controlled pattern amplification in `firstPhasePreProcess` when `optimizationLevel >= 2` (affected range `>= 7.0.0, < 11.0.1`, Dependabot alert 3). Every consumer in the tree requires `^10.x`, so npm cannot resolve the patched version on its own. This is a development dependency; nothing here reaches the shipped bundle.

## [0.1.0] - 2026-11-08

First tagged release. Slices 1–4 of the roadmap are in: shell and languages, node start/stop with health, files on the mutable file system, and pins.

- Every doc ships as two files, one per language, with a switch at the top of each: the README, `docs/`, `CONTRIBUTING`, `GOVERNANCE`, `SECURITY`, and `SUPPORT`. `CODE_OF_CONDUCT.md` stays English, with a Chinese note.
- Pins page for what this node is keeping. Add a pin by content ID, upload a file and pin it, pin an item from Files, and unpin. A fetch from the network is a background job with progress and cancel. A stopped node returns `503 node_not_running`.
- Files page for the node's mutable files. Upload, download, rename, move, copy, and delete go through the manager to Kubo on loopback. A stopped node returns `503 node_not_running`. The narrow file list is a compact card, and the content-ID note uses the panel's disclosure.
- Login and setup passwords can be shown or hidden. A failed login keeps the username. Setup stays disabled until the form is valid.
- First-run admin setup and session login. The manager rejects `/api` routes without a session. Passwords are hashed with argon2id. A forgotten password is cleared with `manager reset-admin`.
- A design system and a polished panel shell, in English and 中文, with light, dark, and system themes.
- Start, stop, and restart one local Kubo from the panel, with a one-sentence status in English and 中文.
- Git Flow, CI, governance docs, and release automation.
