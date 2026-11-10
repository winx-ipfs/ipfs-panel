# 探索

**中文** · [English](explore.md)

**状态：已实现。** 本文是「探索」页的规格。代码和本文不一致时，在修改本文的 pull request 合进来之前，以本文为准。它是场景之后下一块 webui 差距，不是路线图里新的一片，也不是专家模式。

## 这片做什么

新手可以粘贴一个内容标识，或以 `/ipfs/` 开头的路径，看见它点名的那一块：这是什么，以及它指向什么。打开一行，就看下一块。页面说「探索」。它不说「默克尔森林」，也不画一张图。

浏览器仍然不和 Kubo 说话。它只和 manager 说话。manager 在回环上调用 Kubo，而且只调用这一页需要的那些。页面拿不到 Kubo RPC 地址，自己也不会去请求网关。

## ipfs-webui 实际有什么

对照的是 ipfs-webui `c51d980920d60e9d41f53f461886d6e1ea046298`，以及 `ipld-explorer-components` 8.1.3，版本记在 `ipfs-webui/package-lock.json:38474`。

路由是 `/explore` 和 `/explore*`（`ipfs-webui/src/bundles/routes.js:14-15`）。`/explore` 是开始页。更长的路径才是浏览器（`ipfs-webui/src/explore/explore-page-renderer.jsx:25-29`）。

开始页可以粘贴内容标识，或选一个公开的示例数据集。浏览器接着显示四样东西：路径面包屑、节点（种类、内容标识、大小、数据，以及可以点开的链接表）、拆开的内容标识，以及节点和链接的图。节点上可以有「在本地网关查看」和「在公共网关查看」（`ipfs-webui/src/explore/ExploreContainer.js:31-36`）。

块是浏览器里的 Helia 从本地网关取来的，取的是原始块，不是 dag-json（`ipfs-webui/src/bundles/gateway.js:77-86`）。除非人自己打开，远程网关是关的（`ipfs-webui/src/bundles/ipfs-provider.js:328-332`）。顶栏的查看框会用 Kubo 的 `resolve` 解析 `/ipfs/` 或 `/ipns/` 路径，再按得到的内容标识打开浏览器（`ipfs-webui/src/files/explore-form/files-explore-form.tsx:63-82`）。

这一页留下输入框、面包屑、块的种类和链接。不留下图、彩色的哈希课、公开示例数据集、CAR 导入、IPNS，以及公共网关链接。

## 决定

1. **一次一块，走 `dag/get`。** 目标带路径时，manager 先调用 `dag/resolve`，再调用 `dag/get`，`output-codec=dag-json`。这个编码是命令的默认值（`core/commands/dag/dag.go:125`）。`dag/get` 解析路径再编码节点（`core/commands/dag/get.go:39-66`）。`dag/resolve` 返回路径落到的那一块的内容标识，以及块内部还没走完的余下路径（`core/commands/dag/resolve.go:23-30`，文本形式在 `test/sharness/t0053-dag.sh:378-391`）。

   webui 不调用 `dag/get`。它从网关取原始块，因为解码发生在浏览器里。这个面板不把那套解码器放进浏览器，也不打开 `Gateway.AllowCodecConversion`。这个旗标默认是 false，编码对不上就是 `406`（`docs/config.md:1326-1347`）。向网关要 `?format=dag-json`，在默认配置的节点上会失败。`dag/get` 给出的是同一块解码后的 dag-json。更新日志里的目录例子就是这个形状（`docs/changelogs/v0.18.md:221-229`）。

2. **目标是内容标识，或一条 `/ipfs/` 路径。** 规则和固定的目标一样。两端空白忽略。`/ipns/`、`ipfs://`、网址、`..`、空段都会被拒绝。单独一个内容标识就取那一块。带路径的先解析。响应里的 `target` 是规范化之后的输入。`cid` 是路径落到的那一块。`focus` 是块内部的余下路径，没有就省略。UnixFS 的名字通常没有余下路径，因为名字是指向另一块的链接。

3. **节点停着是 503。** 缓存的节点状态不是 `up` 时，路由返回 **503** `{"error":"node_not_running","state":"<state>"}`。缓存还说 `up` 但连接失败，同样是 503，`state` 为 `down`。期限是 60 秒，不是一个旗标。到点是 **504** `timeout`。处理函数使用请求的 context，浏览器断开就会取消对 Kubo 的调用。

4. **RPC 只拨回环。** 调用读取 `<repo>/config`。只有 API 地址是带端口的 `http` 回环来源时才使用。客户端拒绝其他拨号，忽略 `HTTP_PROXY`，也不跟随重定向。响应里没有 API 地址，没有 `5001` 端口，也没有多地址。

5. **本机链接只用来复制。** 网关地址是回环上的 HTTP 来源时，正文带 `localGateway`，和「文件」用的是同一个字段。页面可以复制 `localGateway` 加上屏幕上这一块的 `/ipfs/` 路径。提示说明这个链接只在这台电脑上有效。页面不会打开它，也不把来源地址当成一个字段显示出来。网关不是回环时，省略 `localGateway`。探索仍然可用，因为字节来自 API。

6. **种类是一个短词。** `folder` 是 UnixFS 目录或 HAMT 分片。`file` 是 UnixFS 文件。`symlink` 是 UnixFS 符号链接。`bytes` 是编码 `raw`。`data` 是 dag-cbor、dag-json、json 或 cbor。`block` 是其他情况，包括不是 UnixFS 的 dag-pb。UnixFS 的类型是 `Data` 字节里的第一个 protobuf 字段。更新日志里的目录是字节 `CAE`（`docs/changelogs/v0.18.md:221-224`），也就是字段 1 等于 1。类型 2 是文件，4 是符号链接，5 是 HAMT 分片，显示成文件夹。

7. **链接是这一块自己的链接，有上限。** dag-pb 用 `Links`：`Name`、`Hash`、`Tsize`。其他节点贡献形如 `{"/":"<cid>"}` 的内容标识链接。字节对象 `{"/":{"bytes":"..."}}` 不是链接。最多返回 200 条。还有更多时 `linksTruncated` 为 true。没有链接时 `links` 是 `[]`，不是 `null`。空名字保持 `""`。页面说「未命名的一块」。

8. **预览要么很小，要么不给。** 文件、原始字节和符号链接的字节，只有在它们是合法 UTF-8、不含 NUL、并且不超过 4096 字节时才放进 `text`。结构化数据只有在 JSON 不超过 8 KiB 时才放进来。更大的值设置 `dataOmitted` 并省略 `data`。dag-json 正文超过 4 MiB 是 **413** `too_large`。二进制不以 base64 发出去。

9. **内容标识那一行，只留新手用得上的部分。** `version` 是 0 或 1。`codec` 是 `raw`、`dag-pb`、`dag-cbor`、`dag-json`、`json`、`cbor` 或 `unknown`。多重哈希代码是 `0x12` 时 `hash` 为 `sha2-256`（版本 0 的内容标识必须用这个代码，`internal/pins/cid.go:135`）；代码是 `0x00` 时为 `identity`（webui 测试里的内嵌多重哈希，`ipfs-webui/test/e2e/explore.test.js:86-87`）。其他哈希代码省略 `hash`。页面不打印十六进制摘要。

10. **错误是短代码。** 正文是 `{"error":"<code>"}`。`node_not_running` 另有 `state`。Kubo 自己的句子留在 manager 日志里。

11. **会话和其他页一样。** `GET /api/explore` 需要会话。不需要 CSRF 头。`GET /health` 仍然开放。

## API

| 方法与路径 | 查询 | 成功 |
| --- | --- | --- |
| `GET /api/explore?target=` | 内容标识或 `/ipfs/` 路径 | **200** 一块 |

```json
{
  "target": "/ipfs/bafy.../notes",
  "cid": "bafkrei...",
  "version": 1,
  "codec": "dag-pb",
  "hash": "sha2-256",
  "kind": "file",
  "size": 5,
  "text": "hello",
  "links": [],
  "localGateway": "http://127.0.0.1:8081"
}
```

`size` 是 UnixFS 的文件大小，那个字段存在时才有。文件夹的大小是 0 时省略，节点没有大小时也省略。原始字节的 `size` 是这些字节的长度。`focus`、`text`、`data`、`hash`、`localGateway` 不适用时省略。`data` 是 JSON 值，不是字符串。Kubo 送来了 `Tsize` 时，`links[].size` 就是它，包括 0。

| 状态 | `error` | 何时 |
| --- | --- | --- |
| 400 | `invalid_target` | 空的，或不是内容标识，也不是 `/ipfs/` 路径 |
| 401 | `unauthorized` | 没有会话 |
| 404 | `not_found` | Kubo 解析不了这条路径或这一块 |
| 413 | `too_large` | dag-json 超过 4 MiB |
| 502 | `kubo_error` | Kubo 因其他原因拒绝 |
| 503 | `node_not_running` | 节点状态不是 `up`，或连接失败 |
| 503 | `unavailable` | 配置不是回环 RPC，或 manager 失败 |
| 504 | `timeout` | 60 秒期限到了 |

## 界面

探索在导航里排在「文件」后面：节点、文件、探索、对等节点、固定、场景、设置。

页面有标题、一句话，以及一处默认收起的说明：内容标识点名的是一块，打开它不会改节点。然后一张卡片：一个输入框，主按钮只有「探索」。提示写明粘贴内容标识，或以 `/ipfs/` 开头的路径。空着不会去叫 manager。页面自己就能拒绝的值也不会。

还没查找时，一句安静的话说明还没有打开任何东西。查找之后，第二张卡片给出路径面包屑、作为标题的种类、针对这个种类的一句话、编码和版本的一行次要文字、带复制的内容标识、有的话再给大小、有的话再给文本或数据，以及链接。面包屑的最后一项是当前这一块。更早的项会重新加载那个前缀。名字是单独一段、并且安全的链接，会加载当前路径加上这个名字。其他链接加载那条链接自己的内容标识，所以文件里未命名的一块也能打开。

宽布局是一张表：名字、中间缩短的内容标识、大小。窄布局在 720px 以下，每个链接是一颗按钮：名字，然后是缩短的内容标识和大小。名字换行，不用省略号截断。卡片里的内容标识是完整的，也会换行。

加载时是骨架，加上句子「正在查找这一块。」找不到、超时、Kubo 拒绝，都留在页面上、输入框下面，值还在。节点停了、正在启动、正在停止或需要处理时，卡片换成和其他页一样的空状态，以及一颗回到节点的按钮。连不上管理服务时，是那句话加上「再试一次」。

文案在 `en` 和 `zh`。令牌来自设计系统。栏宽是 `min(960px, 100%)`，和「文件」「固定」一样。

仅开发时，`?explore=start` 和 `?explore=result` 不需要 manager 就能画出两张截图。生产包里没有它们。

## 测试

Go：版本 0、版本 1，以及内嵌的 `bafkqac3imvwgy3zao5xxe3de` 的版本、编码和哈希；文件夹、文件、原始字节、符号链接和一条 dag-cbor 链接的 dag-json；解析到另一个内容标识的路径；余下路径留在 `focus`；被截断的链接表；超过 4 MiB 的正文；拒绝非回环；HTTP 的会话、503、400、404，以及正文里没有 5001 端口。

Web：路径和链接的辅助函数、英文下的页面状态，以及对齐的语言键。

## 不做

- 原始配置编辑器、诊断，以及集成测试基线。对等节点有自己的规格。
- 图、十六进制摘要、公开示例数据集、CAR 导入，以及 IPNS。
- 公共网关链接，或让浏览器去请求任何网关。
- 打开 `Gateway.AllowCodecConversion`。
- 编辑这一块，或在一次响应里列出超大目录的每一条链接。
