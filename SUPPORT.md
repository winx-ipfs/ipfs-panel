# Support

**English** · [中文](SUPPORT.zh-CN.md)

This is one maintainer and an early panel. There is no uptime promise and no support contract.

## Where to write

| You want | Where |
| --- | --- |
| The panel did something wrong | [Bug](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=bug.yml) |
| Kubo should be operable in a way the panel does not do yet | [Feature](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=feature.yml) |
| You are stuck and it is not a bug | [Question](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=question.yml) |
| A vulnerability | [Security](SECURITY.md). Not an issue. |
| You pushed to `main` with the admin bypass | [Post-incident note](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=post_incident.yml), the same day |

GitHub Discussions stay off. A question that is really a bug belongs in a bug issue.

## What we will not debug

- Websites, databases, containers, or the rest of the machine. This is not a host panel.
- How to publish Kubo RPC (usually port 5001) on a public address. Do not do that.
- Kubo bugs that the panel does not add. Those go to Kubo.

Read [Vision](docs/VISION.md) before asking for a feature that manages more than Kubo.

## Versions

Say which git SHA or `vX.Y.Z` tag you are running. `develop` moves. A release tag is the line that is meant to be installed.
