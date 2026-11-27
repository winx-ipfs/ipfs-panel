# 功能清单与开发计划

[English](PLAN.md) · **中文**

这是已经商量好的清单：面板已经有什么、接下来做什么、什么不做。后面接着开发的人，开工前先读这份。盘点日期：2026-11-25（北京时间）。

`docs/HANDOFF.zh-CN.md` 仍然回答「现在什么是真的」。这份文件是计划。如果顺序或范围改了，要在同一个 pull request 里改这份文件，并改交接文档里的「下一步」。

## 这是什么

ipfs-panel 是星萤无垠的本机开源运维软件，用来管一台真实的 Kubo。它不是 SaaS，也不是官方 IPFS 的营销后台。目标是让普通人不用命令行、也不用改原始 JSON，就能启动和停止节点、传文件、固定内容、看连着谁、看日志、改常用设置。专家有入口，但不挡在新手前面。同一套 Web 界面以后会给桌面端和远控管理器共用；现在先把本机这一台做扎实。

- 仓库：https://github.com/winx-ipfs/ipfs-panel
- 品牌：星萤无垠（视觉语言跟 winx.run）
- 默认分支：`main`（发运）。集成分支：`develop`
- 已发行版：v0.1.1。`main` 仍停在这里。`develop` 已经先于这个发行版。

## 后续开发怎么用这份

开工前按这个顺序读。不要凭记忆再开一轮视觉改版。一个 pull request 做一块产品功能。这一刀需要的小重构可以放在旁边。

1. **先读仓库文档。** `AGENTS.zh-CN.md`、`docs/HANDOFF.zh-CN.md`（及英文配对）、本文件、`docs/DESIGN-SYSTEM.zh-CN.md`，以及这一刀对应的 `docs/design/*`。设计文档高于代码。
2. **做当前这一刀。** 就是 [接下来要做的功能](#接下来要做的功能) 里第一项还没做完的，发版那一行除非用户明确要求否则跳过。先写设计再写代码。这一刀需要的重构，或能拿掉后面某一刀明显障碍的重构，可以进同一个 pull request，但要写明原因。不要开工下一刀功能。没有理由就不要重开已决定的事项。不要做 Kubo 插件。不要改设计系统的皮。
3. **竖切一条，pull request 只进 `develop`。** 界面、管理器路由、Kubo 调用、中英文案、测试和截图一起落地。squash，标题用 Conventional Commit。`ci-ok` 必须绿。除非用户明确要求，不要合进 `main`、不要打 tag、不要发版。
4. **做完再交。** 在桌面宽度下审你改过的每一种状态，修好再说完成。布局或文案可能在另一种主题、另一种语言或窄屏上坏掉时，再检查那些。跟这次改动无关的整张矩阵不是交工门槛。不要每修一个小痕迹就停下来等确认。

## 已经落地的功能

状态截至 2026-11-25。「已发运」表示在 `main` / v0.1.1。「在 `develop`」表示代码已合入，但还没有打进发行版。

| 功能 | 状态 | 说明 |
| --- | --- | --- |
| 面板外壳与中英文切换 | 已发运 | 语言切换保存在浏览器里。新字符串必须中英同一 pull request 落地。 |
| 首次运行建管理员与会话登录 | 已发运 | argon2id、会话、CSRF。没有会话不能调 `/api`。 |
| 启动、停止、重启本机一台 Kubo | 已发运 | 一句话状态。API 与网关默认只听 loopback。 |
| 文件（可变文件系统） | 已发运 | 添加、下载、重命名、移动、复制、删除。 |
| 固定与取消固定 | 已发运 | 独立页面，不藏在文件浏览器里。 |
| 四种场景设置 | 在 `develop` | 安静、分享、固定集合、只读网关；可预览、应用、撤销。 |
| 探索一块内容 | 在 `develop` | 按内容标识或 `/ipfs/` 路径看类型与链接。 |
| 对等节点列表与一次性连接 | 在 `develop` | 没有地图。一次性拨号不写 `Peering.Peers`。 |
| 永久对等 | 在 `develop` | 对等节点页上保持连接。实时 `swarm peering` 加上 `Peering.Peers`，并留备份。 |
| 安装与升级 Kubo | 在 `develop` | 下载并校验钉住的发行版。在 `>=0.43.1 <0.45.0` 内升级。操作者自己提供的二进制不会被替换。 |
| 真实 Kubo 集成测试 | 在 `develop` | CI 钉在 Kubo v0.43.1。进 `develop` 的 pull request 会跑。 |
| winx 视觉语言 | 在 `develop` | shadcn-vue + Tailwind + Motion。不再改皮，只修真正的缺陷。 |
| 专家原始配置编辑器 | 在 `develop` | 设置 → 高级。去掉 `PrivKey`。锁定 `Addresses.API`。先看差异再保存，备份可还原。 |
| 日志页 | 在 `develop` | 管理器日志与节点标准输出 / 标准错误。敏感信息已脱敏。 |

## 接下来要做的功能

表里的顺序就是推进顺序。前一刀还没合入 `develop` 之前，不要开下一刀。发版必须用户明确说发才做。

| 顺序 | 功能 | 为什么现在做 | 完成标准 |
| --- | --- | --- | --- |
| 1 | 发布 v0.2.0 | `develop` 上已经有一大批能力，包括安装 Kubo 和永久对等，`main` 还是 v0.1.1，外面下不到。 | 从 `develop` 开 pull request 进 `main`，打 tag，出 linux / macOS / Windows 二进制和校验和。 |
| 2 | 控制面远控 | 管理器默认只听本机，装到服务器后无法用浏览器远管。 | 可以绑定到局域网或公网，走现有登录与 CSRF；Kubo RPC 仍不对外。 |
| 3 | 桌面端 | 同一套界面，包成可安装的本机软件。到这一刀再选壳，不要提前决定。 | 可以像普通软件一样打开本机面板，连的仍是本机管理器。 |
| 4 | IPNS 密钥 | 和 webui 对齐里还缺的专家能力，放在安装和远控之后。 | 能生成、列出、发布与解析名称，私钥不出浏览器。 |
| 5 | 远端固定 | 把内容交给远端 pinning 服务。不是新手主路径。 | 能配置服务与密钥、提交并查看远端固定状态，密钥不进前端代码。 |
| 6 | 诊断 | 检索检查与 DHT provide，让运维能判断内容能不能被找到。 | 新手能看懂一次检查的结果，专家能看到足够细节。 |
| 7 | 第二台节点 | 单机可安装、可远控之后再扩。不要做集群控制台。 | 同一个面板能管两台 Kubo，现有接口不被推倒重来。 |

永久对等已经做完。规格是[永久对等](design/peering.zh-CN.md)。安装与升级 Kubo 已经做完。规格是[安装与升级 Kubo](design/kubo-install.zh-CN.md)。

### 1. 发布 v0.2.0

这是发版，不是功能切片。必须用户明确说发才动手。包进已经在 `develop` 的场景、探索、对等节点、永久对等、安装与升级 Kubo、新界面、配置编辑器和日志。按仓库现有的发版步骤走：`develop` → `main` 的 pull request，打 tag，上传二进制和校验和。发版的 pull request 不夹带新功能。

### 2. 控制面远控

远控的是管理器，不是 Kubo 的 5001 端口。现有登录体系要足够硬，才能从只听本机改成可听局域网或公网。

- 先写设计：绑定地址、TLS、反向代理、防火墙，以及第一次绑到非 loopback 时的警告。
- Kubo RPC 仍只允许 loopback；程序里要再次把关，不只写在文档里。
- 同一套 Web UI，不另起一套远程皮。

### 3 到 7. 桌面端、IPNS、远端固定、诊断、第二台节点

这五刀都要单独设计、单独 pull request。桌面端开始时再比较壳（例如 Tauri 与 Electron），不要提前选。IPNS 与远端固定的密钥不出浏览器。诊断是给运维看的检查，不是分析产品。第二台节点是在「这台节点」上扩，不要推倒现有 API 去做集群控制台。

## 明确不做或暂缓

| 事项 | 怎么对待 | 原因 |
| --- | --- | --- |
| 再开一轮视觉改版 | 停 | 语言已锁定；只修使用中碰到的真正缺陷。 |
| Kubo 插件 | 不做 | 维持成本高，且与用户自行安装 Kubo 冲突。 |
| 分析统计仪表盘 | 暂缓 | 不是运维面板的刚需。 |
| 重启 winx.run | 后话 | 不进本仓库的功能切片。 |
| 站点、数据库、Docker、系统管理 | 不做 | 面板只管 IPFS 节点。 |
| 对外开放 Kubo RPC | 不做 | 浏览器永远不直接调 Kubo。 |
| 把面板做成托管网关产品 | 不做 | 不是本产品。 |
| 换 React / HeroUI / DaisyUI | 不做 | 栈已定：shadcn-vue + Tailwind + Motion。 |
| 多站点 | 后话 | 单机做扎实之后再说。 |

## 一刀怎么做

1. 由本计划或用户指定一刀之后才开工，从最新的 `develop` 拉分支。
2. 先在 `docs/design/` 写清：用户能做什么、管理器路由、Kubo 调用、怎么验证、错误怎么告诉人、与已有切片如何共存、中英文案。
3. 再写管理器与测试，并在集成测试里对真实的 Kubo v0.43.1 补覆盖。
4. 再写界面：工作面铺满、工具条贴列表、行高 44、内边距与其他页一致；只用真实数据。
5. lint、类型检查、Go 与 Web 测试（机器能跑 `-race` 时含它）、文档检查，以及你改过的状态的截图自审。更新交接文档、路线图，以及这一刀碰到的对齐文档。
6. 打开进 `develop` 的 pull request，等 `ci-ok` 全绿，不自动合并。

可以把下面这段贴进云端开发的任务说明里，再补上当前这一刀的范围。

```
You are continuing winx-ipfs/ipfs-panel.
Read AGENTS.md, docs/HANDOFF.md (+ zh-CN), docs/PLAN.md (+ zh-CN), docs/DESIGN-SYSTEM.md, and the design doc for this slice first. Design docs outrank code.

Do the named slice. Write the design in docs/design/ before code. One vertical slice: UI + manager + Kubo call + en/zh + tests + screenshots. A small refactor that the slice needs, or that makes the next named slice easier to extend, can land in the same pull request when the pull request says why. Do not start the next feature.
PR into develop only (squash, Conventional Commit). Do not merge to main, do not tag, do not release unless the user explicitly asked.
Do not reopen a settled decision without a reason in the pull request. No Kubo plugin. No new UI libraries. Do not start a visual restyle. A new dependency needs a reason; do not add one by habit.
Browser never talks to Kubo RPC. Never send Identity.PrivKey to the browser. Addresses.API stays locked.
Visual tokens stay locked: #F9FAFB background, white cards, #E5E5E5 lines, #0A0A0A text, #171717 primary (near-white in dark), #20FFA9 only for logo / selection bar / small tags, radii 6/8/10/12/16, Inter + system CJK, 44px rows, motion 150-250ms ease-out. shadcn-vue + Tailwind + Motion only.
Review the states you changed at desktop width. Check the other theme, language, and a narrow width when the layout or the copy can break there. Wait for ci-ok. Do not auto-merge.
```

## 已锁定的规则

没有理由、没有说明原因的 pull request，不要重开这些决定。

- 技术栈：Vue 3 + Vite + TypeScript。管理器是 Go 标准库（`net/http`），不是 GoFrame，也不是 chi。
- 架构：浏览器 → 管理器 → Kubo；浏览器永远见不到 Kubo RPC 地址。
- Kubo 走进程边界，不做插件；测试基线是已发布的 Kubo 二进制（目前 v0.43.1）。
- 组件库：shadcn-vue + Tailwind + Motion，不换栈、不混用 DaisyUI / HeroUI。
- 配色跟 winx.run：浅灰底、白卡片、黑主按钮；萤光绿 `#20FFA9` 只用于标识、选中条和小标签。
- 圆角同心：标签 6、按钮和导航 8、输入 10、卡片 12、对话框 16。
- 功能合入 `develop`，再从 `develop` 开 pull request 进 `main` 发版；squash；`ci-ok` 必须绿。不要自己合并那个 pull request。
- 不要在用户说发之前合 `main`、打发行版。模型选择和费用不是产品规则。
