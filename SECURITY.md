# Security

**English** · [中文](SECURITY.zh-CN.md)

## Reporting

Report a vulnerability in private. Open a draft advisory:

<https://github.com/winx-ipfs/ipfs-panel/security/advisories/new>

Do not open a public issue, and do not put exploit detail in a pull request or a discussion.

The maintainer is [@devxiaopo](https://github.com/devxiaopo). One person reads these. Expect an acknowledgement within 7 days, and a fix or a written mitigation plan within 90 days when the report is valid. There is no bug bounty.

Private vulnerability reporting has to be enabled on the repository. The switch is in [Org setup](docs/ORG-SETUP.md). Until that switch is on, the advisory link returns an error. In that window, contact @devxiaopo on GitHub and say you have a vulnerability. Do not include the detail in the first public message.

## What to include

- What is affected (`apps/web`, `services/manager`, or a release tag)
- What an attacker can do
- How to reproduce it, if you can
- Whether Kubo itself is the bug, or this panel

## Supported versions

| Version | Security fixes |
| --- | --- |
| Latest `vX.Y.Z` tag on `main` | Yes |
| `develop`, and any older tag | No, unless a severity note says otherwise |
| Before the first tag | `main` is the line that gets the fix |

## Out of scope

- A Kubo bug that this repo does not add. Report that to Kubo.
- Putting Kubo RPC (usually port 5001) on a public address. The panel must not do that, and we will not help someone do it.
- Missing auth on the manager. Current `develop` requires a session for `/api` routes other than setup-status, setup, and login. `GET /health` stays open. A report that only restates the old "no auth yet" README is not a vulnerability. A report that shows how to skip the session, the setup token, or the loopback check is.

## Disclosure

We prefer a coordinated disclosure. We will credit you in the advisory unless you ask us not to. Please give us the 90 days above before you publish, unless we agree a date with you sooner.
