# Vision

English below. 中文在后半部分。Same decisions, two languages.

## English

### One sentence

A Kubo panel a beginner can actually use, where an expert is not locked out of the real settings.

### Usable means a page, not a scavenger hunt

Kubo runs. The happy path is still a pile of flags and one big JSON file. People get stuck on "is the node even up?" and leave.

The panel puts the common jobs on one screen: is the node up, where is the file, what is pinned, which way is this node set up. The words should make sense. Both languages ship together. The home page does not open with every Kubo field on it.

### Like Baota or 1Panel, and only for Kubo

Baota (宝塔) and 1Panel taught a lot of people a simple habit: log into one browser page and operate the service, without first memorizing where the config file lives.

We want that habit for Kubo. We do not want their whole job list.

This repo does not install websites, create databases, manage Docker, or edit the operating system. If a request is "also handle Nginx", the answer is no. The panel is for the IPFS node.

### On this machine, or in a browser somewhere else

One UI, two seats:

- Local: you are at the machine that runs Kubo. The panel is a page on that machine.
- Remote: you open the panel in a browser on another device. You get in through our API, and only after auth.

Remote does not mean "forward Kubo's port". We are not picking a desktop shell (Electron or anything like it) in this foundation. The UI is a web app so both seats can share it later.

### The browser talks to our API

Kubo's own HTTP API, usually `127.0.0.1:5001`, stays on the loopback interface. We do not bind it to a public address, we do not document a port forward for it, and we do not put it behind a reverse proxy "just for convenience".

The browser talks to `services/manager`. That process is our API gateway. It checks the session, then calls only the Kubo operations the panel knows how to explain: start and stop, and the file actions. It is not a pass-through of the raw RPC.

Right now the manager listens on `127.0.0.1:8080` unless `MANAGER_ADDR` is set. It answers `GET /health` for itself. `GET /api/node` plus start, stop, and restart, and the files routes, need the session from [Auth](design/auth.md). Do not publish the manager until you have read that. The files routes never hand the browser Kubo's RPC address.

### Scenes first, raw config later

Most people need a setup they can say out loud:

- a quiet node on this machine
- share a folder with someone
- pin a set of content
- a read-only public gateway

Those are scenes. A scene is a named setup the panel can describe in one sentence, then write into Kubo's config. Guessing at raw keys is not the normal path.

Experts still get the raw config, logs, and peering. That is expert mode, and it comes after the common path works. It is not the home page.

### Languages from the first screen

English and 中文 ship together. `zh` means Simplified Chinese. A new string lands in both locale files in the same change. Translation is not a cleanup task we do later.

A Traditional Chinese locale can show up when someone needs it. It is another locale file, not a second product.

### One node, then more

More than one Kubo node is a real goal. It is not an early one. Get start, stop, files, pins, and scenes solid for a single node first.

Think of the manager as the thing that knows about "the node". A second node should be an addition on that idea, not a reason to throw the API away. Do not build a cluster console now.

### Where the product lives

winx.run is the front door. The code, the issues, and this roadmap live in `ipfs-panel`. There is no other "real" tree hiding somewhere else.

---

## 中文

### 一句话

做一个新手真的能用的 Kubo 面板，熟手也不会被挡在真正的设置外面。

### 好用，是一个页面，不是一场找文档

Kubo 能跑。顺的那条路仍然是一堆参数加一份大 JSON。很多人卡在「节点到底起来没有」，然后就走了。

面板把常见的事放在一个画面里：节点在不在、文件在哪、pin 了什么、这台节点现在是哪种用法。文案要看得懂。两种语言一起给。首页不要一上来铺满 Kubo 的每个字段。

### 要宝塔、1Panel 那种用法，范围只有 Kubo

用过宝塔或 1Panel 的人有个习惯：在浏览器里登录，在一个地方把服务管起来，不用先记住配置文件在哪。

我们要的是这个习惯，对象只有 Kubo。

他们那份工作清单我们不要。这个仓库不装网站、不建数据库、不管 Docker、不改操作系统。有人说「顺便管一下 Nginx」，回答是不做。面板只对 IPFS 节点负责。

### 在这台机器上，或者在别处的浏览器里

同一套界面，两种坐的地方：

- 本机：人就在跑 Kubo 的那台机器旁边。面板是这台机器上的一个页面。
- 远程：在另一台设备的浏览器里打开。要走我们自己的 API，而且要先登录。

远程不是「把 Kubo 的端口转出去」。这个地基里先不选桌面壳（Electron 之类）。界面是网页，这样两种用法以后能共用。

### 浏览器只跟我们的 API 说话

Kubo 自己的 HTTP API 一般在 `127.0.0.1:5001`，就留在回环地址上。不把它绑到公网地址，不在文档里教人做端口转发，也不为了省事把它反代出去。

浏览器访问的是 `services/manager`。这个进程是我们的 API 网关。它先检查会话，然后只去叫面板讲得清的那些 Kubo 操作：启动和停止，以及文件相关的动作。它不是原始 RPC 的透传。

现在 manager 默认听 `127.0.0.1:8080`，除非设置了 `MANAGER_ADDR`。它用 `GET /health` 说明自己还在。`GET /api/node`、启动、停止、重启，以及文件路由，都需要[认证](design/auth.md)里的会话。在读过那一篇之前，别把 manager 公布出去。文件路由不会把 Kubo 的 RPC 地址交给浏览器。

### 先场景，原始配置以后再说

大多数人要的是能用一句话说出来的用法：

- 在这台机器上安静地跑
- 把一个目录分享给别人
- 帮一组内容做 pin
- 一个只读的公共网关

这些叫场景。场景是一个有名字的配置。面板用一句话讲清楚它，再写进 Kubo 的配置。对着原始字段猜，不是正常路径。

熟手仍然可以看原始配置、日志和对等连接。那是专家模式，放在常用路径做完之后。它不是首页。

### 第一屏就有两种语言

英文和中文一起发。`zh` 是简体中文。新的一句文案，同一次修改里两个语言文件都要有。翻译不是以后再打扫的卫生。

有人真的需要繁体时，可以再加一个语言文件。那是多一个 locale，不是另一个产品。

### 先一个节点，再加更多

多个 Kubo 节点是真目标，不是早期目标。先把单个节点的启停、文件、pin、场景做扎实。

把 manager 想成「知道这个节点的那个进程」。第二个节点应该是在这个想法上往上加，而不是推倒 API 的理由。现在不要做集群控制台。

### 产品在哪

winx.run 是门口。代码、issue、这份路线图都在 `ipfs-panel`。没有另一份藏在别处的「真正的」代码。
