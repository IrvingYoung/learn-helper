## Context

LLM Wiki 是个本地单用户工具,无 auth。当前用户使用 cron(如 GitHub 热门项目日报)生成 wiki_pages 后,想分享到微信/小红书等 IM 平台,但:

- 直接发 markdown 文本,mermaid 图丢失
- IM 客户端 markdown 渲染不一致
- 复制粘贴内容太繁琐

需求是给每个 wiki 页面加一个"分享"按钮(下拉菜单),目前菜单只含"用图片分享"一项,生成整页 PNG 长截图,可下载或复制到剪贴板。菜单结构为未来的"复制链接""导出 Markdown"等预留扩展位。

约束:
- 无后端改动(项目无 auth,服务不一定公网暴露,长截图纯前端最简单)
- 必须在用户当前 dark/light 主题下都能用,但**导出图必须是浅色**(分享出去的图必须自洽,不能假设接收方是深色背景)
- mermaid 块异步渲染,必须等所有图都画好再截图
- 按钮放 title 块右上角,但要紧凑(无文字、32x32 图标),不能和 title 视觉竞争

## Goals / Non-Goals

**Goals:**
- 在 PageViewer 标题块旁加一个紧凑的"分享"图标按钮(无文字)
- 点击展开下拉菜单,当前包含"用图片分享"一项
- 选中"用图片分享"后:异步生成 PNG → 弹出预览 → 提供"下载"和"复制到剪贴板"两个动作
- 截图内容与用户在 PageViewer 中看到的内容**完全一致**(包括 mermaid)
- 始终浅色背景 + 深色文字,无视用户当前主题
- 在 user gesture 内完成截图(避免被浏览器拦截)

**Non-Goals:**
- 不做 OG 卡片 / 分享卡(短图)、PDF
- 不做整棵子树截图
- 不做服务端渲染 / 后端 API
- 不做图片编辑(裁剪、滤镜、标注)
- 不支持选区截图(始终整页)
- 本 change 不实现"复制链接""导出 Markdown"等菜单项(预留位,但未实现)

## Decisions

### D1. 截图库: `modern-screenshot`

| 选项 | 优势 | 劣势 |
|---|---|---|
| `html2canvas` | 知名度高 | ~50KB,SVG/foreignObject 处理有已知 bug,基本不维护 |
| `html2canvas-pro` | 修了部分 bug | 仍非主流,偶尔维护 |
| **`modern-screenshot`** | **~10KB,Promise API,SVG/foreignObject 支持好,mermaid 输出 SVG 友好,活跃维护** | 较新,生态略小 |

`modern-screenshot` 对内嵌 `<style>` 块的 SVG(正是 mermaid 的产物)处理最稳,且 API 简单:`domToPng(node, options)`。

### D2. 不截可见 DOM,改为"离屏克隆"

直接在 PageViewer 现有 DOM 上截图有这些问题:
- 用户的 dark mode 会渗透到导出图(违反"必须浅色")
- 截图区域受 viewport 限制,长内容被切
- 截图包含 app chrome(左侧知识树、顶部导航等)——用户要分享的只是文章本体
- 截图时若用户滚动,内容会变

**方案**:点击按钮后,程序化构造一个**离屏容器**:
- `position: fixed; left: -99999px; top: 0`
- 外宽 **800px**(`box-sizing: border-box`),`background: #fff`,`padding: 48px`(内部内容区 ≈ 704px,article 仍受 `max-w-2xl=672px` 限制,左右居中)
- 通过 `cloneNode(true)` 把 PageViewer 中 markdown 渲染后的 DOM 拷过去
- 给克隆体加 CSS 覆盖,强制浅色变量
- 在 host 顶部加 **4px 橙色品牌条**(`#c45c26`,`position: absolute; top: 0; left: 0; right: 0`)
- 在 clone 后追加 **footer**:`learn-helper · YYYY-MM-DD · N 字`,12px 灰字,顶部分隔线
- 截图后立即 `remove()` 节点

**为什么用 cloneNode 而不是重新渲染 React**:
- 重新渲染会触发 mermaid 的 useEffect,**重新请求 mermaid.render()**,耗时且可能不一致
- cloneNode 是"已渲染状态的字节级复制",**不会**触发 React 生命周期,也不会重画 mermaid
- 离屏容器中 mermaid SVG 已经是最终产物

**为什么外宽 800 而内容仍是 672**:
- 720-800px 是 IM 分享图的舒适宽度(1080p 屏刚好充满)
- 内容区 672 是 max-w-2xl,正文可读性已经验证过
- 增大的 64px(原 32 → 48 padding × 2)留给品牌条 / footer / 呼吸感,不被内容区吞掉

### D3. 等待 mermaid 渲染完成

mermaid 是异步的(用 `useEffect` + `mermaid.render()`),cloneNode 不会重画,但**首次进入页面时,可能用户点"分享"那一刻仍有 mermaid 在画**。

**等待策略**:
- 离屏容器插入后,轮询 `querySelectorAll('.mermaid-loading')`
- 全部为 0 时,再加 200ms 让浏览器完成最后的 paint/layout
- 最多等 5s,超时则降级(可能截到 loading 占位符,但不阻塞)

### D4. 强制浅色主题

深色主题下,Tailwind 的 `prose-custom` 等类会输出深色样式。克隆体需要**显式覆盖**:
- `background: #fff !important; color: #1a1a1a !important;`
- 代码块 `background: #f5f5f5 !important; color: #1a1a1a !important;`
- mermaid: 调 `mermaid.initialize({ theme: 'default' })`(虽然克隆的是已渲染 SVG,但保险起见在容器上写死颜色)

实际上,`modern-screenshot` 在 `backgroundColor` 选项上支持传入固定色,且我们直接对克隆体用行内 style 覆盖 `color-scheme: light`,即可避免深色变量渗透。

### D5. 分享按钮 = 下拉菜单(预留扩展)

不是直接按钮,而是一个**紧凑的图标按钮** + 下拉菜单:

```
按钮触发器(在 title 块右上,32x32 图标按钮,无文字):
  ┌──────┐
  │  ⤴  │   ← 三个点 share-with-nodes 图标
  └──────┘
       │
       ▼  点击展开
  ┌─────────────────┐
  │ 🖼 用图片分享    │   ← 当前唯一项
  │ ─────────────── │
  │   (未来扩展位)   │
  └─────────────────┘
```

- 触发器:32x32,无文字,无边框,仅图标 + hover bg
- 菜单:`absolute right-0 top-full mt-1`,白底 + 阴影 + 圆角,最小宽度 180px
- 菜单项是 `<button role="menuitem">`,每项 = 图标 + 文字
- 点击外部 / Esc 关闭菜单
- 当前只 1 项"用图片分享";结构已为未来的"复制链接""导出 Markdown"等预留

### D6. 预览弹窗结构

```
┌──────────────────────────────────────────┐
│  分享为图片                       [×]    │
├──────────────────────────────────────────┤
│  ┌──────────────────────────────────┐    │
│  │                                  │    │
│  │  [生成的 PNG 缩略图,             │    │
│  │   max-height: 70vh, 可滚动]      │    │
│  │                                  │    │
│  └──────────────────────────────────┘    │
│  文件大小: 1.2 MB · 1240 × 3840          │
├──────────────────────────────────────────┤
│  [复制到剪贴板]      [下载 PNG]   [关闭] │
└──────────────────────────────────────────┘
```

- "复制"成功时按钮显示 ✓ 反馈
- 不支持 clipboard API 时,"复制"按钮 disabled 并加 tooltip
- 加载中显示 spinner,期间按钮全 disabled

### D7. 复制到剪贴板

`navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])`:
- **必须在 user gesture 内**(点击事件回调同步路径)
- HTTPS 或 localhost 才能用(本地 dev 没问题)
- 主流浏览器支持(Chrome 76+, Edge 79+, Safari 13.4+)
- 不支持时降级:按钮 disabled,加 title 提示

### D8. 文件名

`wiki-${slug}-${YYYY-MM-DD}.png`
- slug 已有(可能含中文),保留中文(主流 OS 文件系统都支持)
- 日期用本地时区

### D9. 模块拆分

| 文件 | 职责 |
|---|---|
| `lib/share-as-image.ts` | 纯函数:`exportPageAsPng(sourceEl, options) → Promise<Blob>` / `copyPngToClipboard(blob) → Promise<boolean>` / `downloadBlob(blob, filename)` |
| `components/ShareAsImageModal.tsx` | UI 弹窗,接收 `blob | null`,渲染预览和按钮 |
| `components/PageViewer.tsx` | 加分享图标按钮(下拉)+ 菜单 + 调 lib + 显示 modal |

`lib` 抽出来便于单测和未来复用(比如后续要做"批量导出子树"也能用)。

## Risks / Trade-offs

**[Risk] 极长页面(mermaid 多 + 文字巨多)截图可能 OOM 或慢** → Mitigation:在 PageViewer 入口处加 content length 软上限警告(>50k 字符时按钮加 ⚠️),实际生成时用 `requestIdleCallback` 包装,允许 5s 超时取消

**[Risk] `modern-screenshot` 在 Firefox 上对内嵌 SVG 的处理有边界 bug** → Mitigation:先做"裸 markdown 不含 mermaid"路径作为基础,逐步加 mermaid;若 Firefox 实际出问题,降级到"先 svg-to-png 把 mermaid 块单独 rasterize,再合到主图"(更复杂,留作 fallback)

**[Risk] clipboard API 在非 HTTPS 下不可用(部署到公网时)** → Mitigation:按钮 disabled + 显式提示"当前环境不支持复制,请下载后手动粘贴"

**[Risk] 离屏容器被 React 状态变更影响(比如 useEffect 触发重渲染时改 DOM)** → Mitigation:cloneNode 是深拷贝,React 不知道克隆体的存在,不会触发它的重渲染

**[Risk] 截图瞬间页面被 hot-reload(Vite)清掉** → Mitigation:Dev 环境下偶发,用户重试即可;生产构建无此问题

**[Risk] 用户频繁点击"分享"导致重复生成** → Mitigation:生成中按钮 disabled、modal 上 spinner、escape 键不响应直到完成
