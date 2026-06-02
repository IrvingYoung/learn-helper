## Context

LLM Wiki 是本地单用户工具,Go + React/Chi + SQLite。运行在 `:3000`(Vite)/`:8080`(Go),无 auth。

**当前分享路径**:
1. `share-page-as-image`(in progress):PageViewer 标题块菜单项 "用图片分享",前端 `modern-screenshot` 把页面 DOM 截成 PNG,可下载或复制到剪贴板。**只解决"发图"场景**。
2. 手动复制 markdown 文本:接收方在 IM 里看到无格式纯文本,mermaid 全部丢失。

**用户场景**:Wiki 里有 cron 跑出来的"GitHub 热门项目日报"等可分享内容,用户希望**主分享路径是"复制一个链接粘到 IM"**。这要求:
- 链接可点击直达内容(对方无需安装/登录任何东西)
- IM 抓链接时显示**完整预览卡**(标题/描述/图),不是空白
- 主人的所有页面(草稿、概览、已发布)都能生成链接,但公开访客看到的副本**自洽、可读**

**为什么不是纯 SPA 方案**:SPA 的初始 HTML 永远是同一份 `index.html`,没有 `<meta property="og:*">`。IM 爬虫(微信、小红书、Twitter)不跑 JS,只看初始 HTML。**og: meta 必须出现在响应头几个 KB 里**,这是把"Go 注入 meta"和"前端直连"区分开的关键。

**为什么不用 Go 重渲染 markdown**:markdown 库(react-markdown + remark-gfm + mermaid + 代码高亮)在 Go 端没有等价物。强行重写一遍会失去与 SPA 的功能一致性(尤其是 mermaid 异步渲染)。**最优解是 Go 只塞 meta,内容仍由前端组件渲染**。

## Goals / Non-Goals

**Goals:**
- 每个 wiki 页有且仅有一个稳定的公网 URL,带不可枚举的 share_token
- IM(微信/小红书/Twitter)抓链接时显示完整预览(og:title/description/url/image)
- 公开访客点开链接看到页面正文,内容与主人在 SPA 内看到的一致(mermaid、代码块、内链全部正确)
- 主人端 SPA 地址栏永远反映当前页(浏览器后退/前进/深链全部正确)
- 菜单极简:只复制,不旋转 token
- slug 一次性:重命名不破坏已分享的链接

**Non-Goals:**
- 不做服务端 markdown 渲染(用 SPA 渲染)
- 不做 og:image 个性化(每页一张卡片图)—— v1 用静态 logo
- 不做 share_token 旋转 UI(菜单只复制,v1 不暴露重置入口)
- 不做公开访客鉴权(无登录、无 read token 之外的访问控制)—— token 即鉴权
- 不做 wiki 内链的"对方也能跳转"(`[[xxx]]` 链接到 `/wiki/...` 在公开访客那里会 404,文档提一下即可)
- 不做旧 slug 的 301 redirect(重命名后旧链接就 404,这是 slug 一次性策略的代价)
- 不做"按页控制是否公开"(所有非 system 页都生成 token,没有"私有页"开关)

## Decisions

### D1. 路径分离:`/wiki/{slug}` 主人用,`/share/{slug}?t={token}` 公开用

```
/wiki/{slug}                  → 主人 SPA(React Router 接管)
/share/{slug}?t={token}       → 公开 SSR(Go 拦截,注入 meta 后返回 SPA HTML)
/api/wiki/{slug}              → 主人 API(同源,无 token)
/api/share/{slug}?t={token}   → 公开 API(SPA 加载时拉数据用,带 token 校验)
```

| 备选 | 否决理由 |
|---|---|
| `/wiki/{slug}?t={token}` 主人和公开共用 | 主人端 URL 不带 token,Go SSR 时就要做"有 token 走注入/无 token 走 SPA",路径分发变复杂 |
| `/p/{slug}` 公开 | 跟 Notion/语雀习惯不一致,且 `/wiki/{slug}` 已存在 |

**理由**:路径前缀本身就表达了"这是给人看的快照"还是"这是主人的应用入口",部署反代规则只需一条:`/share/*` → :8080。

### D2. 鉴权:share_token(不可枚举,32 位)

```go
const shareTokenAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"  // 去歧义字符
func newShareToken() string {
    b := make([]byte, 32)
    for i := range b {
        b[i] = shareTokenAlphabet[rand.Intn(len(shareTokenAlphabet))]
    }
    return string(b)
}
```

| 备选 | 否决理由 |
|---|---|
| 全公开(无 token) | slug 是可枚举的(pinyin + 时间戳),任何人猜到 slug 就能看,违反"主路径分享"应有的可控性 |
| 沿用 `content_status` 只有 published 公开 | 给主人增加心智负担,且当前没区分"已发布"和"私密草稿",状态语义不匹配 |
| UUID v4 | 36 位含连字符,URL 不够干净;32 位随机等效 |

**理由**:`share_token` 充当"读权限",主人分享时只发"链接 + token"这个组合,token 不可枚举(32 位 31 字符集 = ~157 bits 熵,远超爆破空间)。

### D3. SSR 注入方式:Go 读 SPA index.html,string-replace 注入 `<head>`

```go
// 伪代码
indexBytes, _ := os.ReadFile(distPath + "/index.html")  // 生产:go:embed
meta := buildOgMeta(page)  // og:title/description/url/image
html := strings.Replace(string(indexBytes), "</head>", meta + "</head>", 1)
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.Write([]byte(html))
```

| 备选 | 否决理由 |
|---|---|
| 预渲染静态 HTML 写盘 | 写盘时机难定(发布时/访问时),缓存失效逻辑复杂 |
| 调 Node 进程跑 React SSR | 部署变重(Go 进程要带 Node),完全违反"用现有栈" |
| 写一个独立"share 专用"的 Go HTML 模板 | 失去与 SPA 的样式一致性(字体/颜色/mermaid 样式都要重维护) |
| Edge function(Cloudflare Worker) | 绑 CF,不通用 |

**理由**:主人在 SPA 看到的页面和公开访客看到的页面应该**像素级一致**,最稳的做法就是同一份 HTML,Go 只动 `<head>`。

### D4. og:description 来源:`summary` 字段,fallback 首段非空行

```go
func buildOgDescription(page *WikiPage) string {
    if page.Summary != "" {
        return truncate(page.Summary, 200)
    }
    // 从 content 中提取第一段非空文本(去掉 markdown 标记)
    return truncate(extractFirstParagraph(page.Content), 200)
}
```

`summary` 字段已在 DB 里(`backend/cmd/server/main.go:238` 的 migration 引入),由 AI 自动生成,优先用;没有则取首段 200 字符。

### D5. og:image:v1 静态 logo

`/static/og-default.png`,项目 logo + 品牌色(项目已用 `BRAND_TEXT = "learn-helper"` 配 `#c45c26` 强调色)。后续可加"按页生成缩略图",留作 v2。

### D6. 公开 API `/api/share/{slug}?t={token}` 与主人 API `/api/wiki/{slug}` 共存

后端 handler 拆分:
- `GetPublicSharePage`:校验 token 后返回 `{id, title, slug, content, content_status, summary, ...}`,**不返回** `share_token`(防止访客拿到 token 再传)
- `GetWikiPageBySlug`(主人):维持现状,顺便返回 `share_token` 字段供前端菜单用

主人端在 SPA 内的所有 API 调用都不带 token(SPA 同源即信);公开访客的 SPA 加载时自动从 URL 读 `?t=`,拼到 `/api/share/...` 请求里。

### D7. Slug 一次性:`UpdateWikiPage` 不再算 slug

```go
// UpdateWikiPage 旧逻辑
newSlug := slugify(req.Title)  // 删掉
h.db.Exec("UPDATE wiki_pages SET title=?, content=?, slug=?, ...", ...)  // 删 slug=?
h.db.Exec("UPDATE wiki_pages SET title=?, content=?, ...", ...)  // 改成不更新 slug
```

历史 slug 链接失效是 v1 接受的成本,文档说明。

### D8. 前端路由同步:点页 → `useNavigate` 更新 URL

`WikiPage` 组件:
- `const { slug: urlSlug } = useParams<{slug: string}>()`
- `useEffect(() => { if (urlSlug && urlSlug !== selectedSlug) setSelectedSlug(urlSlug) }, [urlSlug])`
- 树点页时:`onSelectPage={(slug) => navigate('/wiki/' + slug)}`

这样:
- 直接访问 `/wiki/foo` → 打开 foo
- 主人点树里的 bar → URL 变 `/wiki/bar`,浏览器后退回 `/wiki/foo`
- 主人复制地址栏粘到 IM(自己) → 拿到的是带 slug 的完整 permalink
- 公开访客访问 `/share/foo?t=...` → 不进这个路由(Go 拦截后直接吐 HTML,不走 SPA 的 React Router),SPA 启动后用 `useLocation` 看到 `/share/...` 路径,调 `/api/share/...` 拉数据

### D9. `MarkdownContent` 在公开路径下隐藏主人专有 UI

公开访客的 SPA 不应显示"在 AI 中打开"选择 tooltip、确认发布草稿条等。两种实现:

| 方案 | 说明 |
|---|---|
| A. 路由级判断 | `WikiPage` 读 `useLocation()`,`/share/...` 时不传 `onAskAI` 等 props |
| B. `MarkdownContent` 内置 `readOnly` prop | 更显式,组件级控 |

**采用 A**:`WikiPage` 是容器,所有"主人功能"都通过 props 注入,`/share/...` 时不注入即关闭。无需改 `MarkdownContent`。

## Risks / Trade-offs

**[Risk] Go 反代/路由配置漏改 `/share/*`,公开链接 404** → README + 部署文档明确列出反代规则表;dev 模式 Vite proxy 同步配;生产部署时 sanity check 脚本(可选,留 v2)

**[Risk] 主人端 SPA 在公开访客的浏览器跑,会调用写 API(理论上)** → 现状项目无 auth,任何公网暴露的 API 都有写权限;本次新增的 `/api/share/...` 是**只读 API**,只 GET 不 PUT/POST/DELETE;**但主人原 `/api/wiki/...` 写 API 仍可被公网访客调用**——这是项目架构层面的限制,文档明确"必须经反代,8080 不可公网直连,或加 nginx basic auth"

**[Risk] 旧 slug 失效(用户重命名后)** → v1 接受,文档说明;后续可加 `slug_history` 表 + 301 redirect

**[Risk] og:description 取首段时,首段是 mermaid 代码块,渲染成空字符串** → `extractFirstParagraph` 实现里跳过 fenced code block;fallback 仍是站点默认描述

**[Risk] `/api/share/...` 返回的 content 含有 `[[wiki 内链]]` 语法,公开访客点击会跳到 `/wiki/...` 触发 404** → `MarkdownContent` 已有 `onInternalLink` 钩子,在 `/share/...` 路径下不传 `onInternalLink`,内链降级为纯文本(`[[xxx]]` → "xxx",不渲染为 `<a>`)。v1 接受"公开访客点 wiki 内链无响应"

**[Risk] 公开访客的 SPA 加载需要执行 JS,无 JS 环境(罕见)看不到内容** → 折中:SSR HTML 的 `<body>` 里包含一个 `<noscript>该页面需要 JavaScript</noscript>` 提示,作为最低保障

**[Risk] 注入 `og:*` 时如 dist/index.html 结构变了(Vite 升级),`</head>` 替换失效** → 用 `html/template` 解析 + 注入,或保留 Vite 自定义插件在 index.html 里放一个 `<!--OG-META-->` 占位符,Go 直接 replace 占位符(更稳)。**v1 先用 `</head>` 替换**,Vite 升级时再切换到占位符方案

**[Risk] 同一页面高频被不同人点开,每次都重读 index.html + 字符串拼接** → 性能不是瓶颈(SQLite 读页面也耗时),`index.html` 内存常驻即可;不优化

## Migration Plan

无破坏性改动:
1. **DB migration**:加 `share_token` 列 + 索引,**默认值 `''`(空串)**,已有页面不会自动有 token(必须由主人**主动**触发重生成,或在主人首次进入 SPA 时**惰性回填**)
2. **后端**:`CreateWikiPage` 加 token 生成逻辑,**只影响新页**;已存在的页 token 为空,`/api/share/{slug}?t=` 永远 404(等同不可分享)
3. **前端**:新路由、新菜单项,无破坏性
4. **回填策略**(可选,留作实施期决定):SPA 启动时检查所有本地缓存的页面,token 为空时调一个 `POST /api/wiki/{id}/ensure-share-token` 后端补上

回滚:删 share_token 列 + 删新路由 + 删新菜单项。零数据丢失(share_token 是可丢弃的元数据)。

## Open Questions

- **og:description 截断长度 200 字符是否合适?** IM(尤其小红书)预览卡宽度有限,可能要试几个值,留作实施时按真机调整
- **`/api/share/...` 公开 API 是否需要 rate limit?** 32 位 token 不可枚举,理论上不需要;但万一 token 泄露(用户截图分享到公网),无防护。下个版本可加 IP-based rate limit
- **公开访客的 SPA 是否要显示站点头部("在 LLM Wiki 中打开")?** 设计上有助于主人自己点自己分享的链接,直接进入 SPA。当前决定:显示一个简单的 `<a href="/wiki/{slug}">` 顶部链接,纯样式、无 SPA 行为
