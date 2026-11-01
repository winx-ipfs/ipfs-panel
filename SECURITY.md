# Security

English below. 中文在后半部分。

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
- Missing auth on the manager today. That is a known gap, written in the README. A report that only restates it is not a vulnerability. A report that shows how to abuse it beyond the loopback default is.

## Disclosure

We prefer a coordinated disclosure. We will credit you in the advisory unless you ask us not to. Please give us the 90 days above before you publish, unless we agree a date with you sooner.

---

# 安全

## 报告

漏洞请私下报告。开一份草案通告：

<https://github.com/winx-ipfs/ipfs-panel/security/advisories/new>

不要开公开 issue，也不要在 pull request 或讨论里写利用细节。

维护者是 [@devxiaopo](https://github.com/devxiaopo)。目前只有一个人看。有效报告会尽量在 7 天内确认收到，并在 90 天内给出修复或书面的缓解计划。没有漏洞奖金。

仓库需要先打开「私下漏洞报告」。开关在 [组织设置](docs/ORG-SETUP.md)。那个开关没开时，上面的链接会报错。这段时间请在 GitHub 上联系 @devxiaopo，只说你有一个漏洞。第一条公开消息里不要写细节。

## 请写上

- 影响哪里（`apps/web`、`services/manager`，或某个发行标签）
- 攻击者能做什么
- 如果能复现，怎么复现
- 这是 Kubo 自己的问题，还是这个面板的问题

## 支持的版本

| 版本 | 会修安全问题 |
| --- | --- |
| `main` 上最新的 `vX.Y.Z` 标签 | 会 |
| `develop`，以及更旧的标签 | 不会，除非严重程度说明里另写 |
| 还没有第一个标签时 | 修在 `main` 上 |

## 不在范围内

- Kubo 自己的缺陷，而这个仓库没有加一层。请向 Kubo 报告。
- 把 Kubo RPC（一般是 5001 端口）开到公网。面板不应该这么做，我们也不会帮别人这么做。
- 管理服务目前没有认证。这是 README 里写明的缺口。只是复述这一点，不算漏洞。如果报告说明了怎么在默认的本机回环之外滥用它，那算。

## 公开

我们希望协调后再公开。除非你要求不写名字，通告里会注明你。请在上面说的 90 天之内不要自行公开，除非我们和你约了更早的日期。
