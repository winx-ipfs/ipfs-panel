# 治理

[English](GOVERNANCE.md) · **中文**

## 现在的状态

[@devxiaopo](https://github.com/devxiaopo) 是唯一的维护者，也是唯一的仓库管理员。在真正有第二名维护者之前，必须的批准人数保持 **0**。pull request 仍然要开。规矩是「先开 pull request」，不是「先找第二个人」。

这份文件是政策。真正挡住推送的点击在 [组织设置](docs/ORG-SETUP.zh-CN.md)。

## 角色

**维护者。** 可以合并进 `develop` 和 `main`。更新 [CODEOWNERS](.github/CODEOWNERS)、这份文件和 ruleset。以 [愿景](docs/VISION.zh-CN.md) 和 [路线图](docs/ROADMAP.zh-CN.md) 为产品边界，决定什么可以进来。

**仓库管理员。** 维护者里还可以改设置的人，并且只在紧急情况下可以不经 pull request 推到 `main`。现在是 @devxiaopo。绕过不是日常的第二条路。

**贡献者。** 提 pull request 或 issue 的人。贡献者不需要写权限。

没有委员会，也不投票。一名维护者做决定。不同意就开 issue 或 pull request，维护者在线程里说可以或不可以。

## 改动怎么进来

日常工作的目标分支是 `develop`。发行和紧急的生产修复目标是 `main`。分支名、提交标题、该点哪个合并按钮，写在 [参与方式](CONTRIBUTING.zh-CN.md)。

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
