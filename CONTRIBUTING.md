# Contributing

English below. 中文在后半部分。

## Branches

`main` is the only long-lived branch. It should build.

Every other branch is short-lived:

- `feat/<short-name>` for a feature
- `fix/<short-name>` for a fix

Open the branch, open a pull request, merge, delete the branch. There is no `develop` branch. Do not push commits straight to `main`. A pull request is required, including for a small docs change.

GitHub does not enforce that yet. Owner steps are in [Org setup](docs/ORG-SETUP.md).

## Pull requests

Say which roadmap slice the change belongs to. Use "foundation" for repo hygiene that is not a product slice yet.

If you change UI copy, change `apps/web/src/locales/en.ts` and `apps/web/src/locales/zh.ts` in the same pull request. The keys match. `zh` is Simplified Chinese.

Anything that talks to Kubo goes through `services/manager`. Do not add a screen, a sample, or a paragraph that tells people to put Kubo RPC (port 5001) on a public address.

## Stack

- UI: Vue 3, Vite, TypeScript, Vue Router, Pinia, vue-i18n.
- Manager: Go. Use the standard library. [chi](https://github.com/go-chi/chi) is fine once `ServeMux` is getting hard to read. GoFrame is not part of this project.
- A new framework needs a reason in the pull request, and a yes, before the code lands.

Editor hint: VS Code with the official Vue extension (Volar). Turn Vetur off if it is still installed.

## Checks

From `apps/web`:

```bash
npm run typecheck
npm run build
```

From `services/manager`:

```bash
go test ./...
```

---

# 参与方式

## 分支

只有 `main` 是长期分支。它应该能构建。

其他分支都是短期的：

- `feat/<短名字>` 做功能
- `fix/<短名字>` 做修复

开分支，提 pull request，合并，删掉分支。没有 `develop`。不要把提交直接推到 `main`。改几个字的文档也要走 pull request。

GitHub 还没有强制这一点。组织所有者的点击步骤在 [组织设置](docs/ORG-SETUP.md)。

## Pull request

写清楚这是路线图里的哪一片。还不算产品切片的仓库卫生，就写 foundation。

改界面文案时，同一个 pull request 里同时改 `apps/web/src/locales/en.ts` 和 `apps/web/src/locales/zh.ts`。键要一致。`zh` 是简体中文。

和 Kubo 说话的代码放在 `services/manager`。不要加页面、示例或说明，去教别人把 Kubo RPC（5001 端口）开到公网地址上。

## 技术栈

- 界面：Vue 3、Vite、TypeScript、Vue Router、Pinia、vue-i18n。
- 管理服务：Go。先用标准库。`ServeMux` 实在难读时可以用 [chi](https://github.com/go-chi/chi)。这个项目不用 GoFrame。
- 想再加一个框架，先在 pull request 里写理由，等同意了再把代码放进来。

编辑器可以是 VS Code 加上官方 Vue 扩展（Volar）。如果还装着 Vetur，关掉它。

## 检查

在 `apps/web`：

```bash
npm run typecheck
npm run build
```

在 `services/manager`：

```bash
go test ./...
```
