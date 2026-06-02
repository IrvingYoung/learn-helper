## Why

Wiki 是个人知识库,公网分享的主路径应该是"能直接点开的 URL"。当前外发只有手动复制 markdown(对方收到无格式纯文本)和分享为图片(走 `share-page-as-image`),缺一条**公网 permalink**:任何拿到链接+token 的人点开就是该页可读副本,且在 IM 里粘贴时有完整的 og: 预览卡(标题/描述/图)。

## What Changes

- **新增 `share-permalink` 能力**:每个 wiki 页创建时自动生成不可枚举的 `share_token`,公开 URL 形如 `https://{host}/share/{slug}?t={token}`,Go 端 SSR 注入 og: meta,IM 抓链接可见预览
- **前端**:PageViewer 分享菜单新增 "复制链接" 项(`navigator.clipboard.writeText` + ✓ 反馈);新增 `/wiki/:slug` 路由支持深链;应用内点页时 `useNavigate` 同步 URL,浏览器后退/前进/地址栏全部正确
- **后端**:
  - DB 加 `share_token` 列 + 索引,1 个 migration
  - `CreateWikiPage` 自动生成 32 位 share_token(去歧义字符集)
  - `UpdateWikiPage` 删除 slug 重算逻辑,slug **创建时一次定终身**
  - 新增 `GET /api/share/{slug}?t={token}`(公开内容 API,token 校验后返回 content)
  - 新增 `GET /share/{slug}?t={token}`(SSR 入口,Go 读 SPA index.html 注入 og:title/description/url/image 后返回)
- **部署**:README 补充反代/Vite dev proxy 需多挂 `/share/*` → :8080

## Capabilities

### New Capabilities
- `share-permalink`: 公网可点的页面 URL,带不可枚举 token 鉴权,Go 端注入 og: meta 使 IM 链接预览完整

### Modified Capabilities
- 无。`page-image-export` 的 spec 行为不变(图片分享路径独立);`wiki-patch-edit`、`topic-content` 等的 REQUIREMENT 都不动 —— 这次只新增,不修改既有契约

## Impact

- **DB**:1 个 migration(`ALTER TABLE wiki_pages ADD COLUMN share_token` + 索引)
- **后端**:`backend/internal/handler/wiki.go` 加 2 个 handler(SSR + 公开 API),改 2 个 handler(`CreateWikiPage` 加 token 生成,`UpdateWikiPage` 删 slug 重算)
- **前端**:
  - `frontend/src/App.tsx` 加 `/wiki/:slug` 路由
  - `frontend/src/components/PageViewer.tsx` 分享菜单加新项,加 `useEffect` 同步 URL
  - `frontend/src/app/wiki/page.tsx`(`WikiPage`)读 `useParams().slug` 作初始页
  - `frontend/src/types/index.ts`(`WikiPage` 类型)加 `share_token?: string`
- **部署**:
  - 生产反代增加一条规则:`/share/*` → :8080
  - `frontend/vite.config.ts` dev proxy 同步加 `/share` 规则
  - `README.md` 加部署拓扑说明
- **风险**:
  - 服务公网暴露时,`/api/wiki/{slug}` 也可直连(泄露 share_token + 写权限) → 文档明确"必须经反代,8080 不可公网直连"
  - 公开访客的浏览器跑 SPA,会调 `/api/share/{slug}?t={token}` → 公开 API 是该方案刻意暴露的最小面
