# 支持

[English](SUPPORT.md) · **中文**

这里只有一名维护者，面板也还早。没有可用性承诺，也没有支持合同。

## 写到哪里

| 你想做的事 | 去哪里 |
| --- | --- |
| 面板做错了事 | [缺陷](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=bug.yml) |
| Kubo 有一件事面板还做不了 | [功能](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=feature.yml) |
| 卡住了，但不是缺陷 | [问题](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=question.yml) |
| 漏洞 | [安全](SECURITY.zh-CN.md)。不要开 issue。 |
| 你用管理员绕过直接推了 `main` | 当天开一份 [事后说明](https://github.com/winx-ipfs/ipfs-panel/issues/new?template=post_incident.yml) |

GitHub Discussions 保持关闭。真是缺陷的问题，放进缺陷 issue。

## 我们不会帮着查的

- 网站、数据库、容器，或者机器上的其他东西。这不是主机面板。
- 怎么把 Kubo RPC（一般是 5001 端口）开到公网。不要这么做。
- Kubo 自己的缺陷，面板没有再加一层。那些去找 Kubo。

想要一个管的不只是 Kubo 的功能之前，先读 [愿景](docs/VISION.zh-CN.md)。

## 版本

写上你跑的 git SHA 或 `vX.Y.Z` 标签。`develop` 会往前走。应该安装的是发行标签。
