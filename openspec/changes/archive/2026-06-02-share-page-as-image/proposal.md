## Why

Wiki 页面(尤其是 cron 任务自动生成的报告,如 GitHub 热门项目日报)是用户在 IM 中乐于分享的内容。当前唯一的外向出口是手动复制 markdown 文本——但 markdown 渲染在 IM 客户端中不一致,mermaid 图直接丢失。用户需要一个"所见即所得、含图、可直接发到 IM"的导出方式。

## What Changes

- 在 PageViewer 标题块新增一个紧凑的"分享"图标按钮(三个点的 share-with-nodes 图标),点击展开下拉菜单
- 菜单第一项为"用图片分享",触发预览弹窗
- 预览弹窗展示:页面内容长截图(PNG,白底深字)、下载按钮、复制到剪贴板按钮
- 客户端实现截图,无后端改动,无新 API
- 新增前端依赖 `modern-screenshot`(轻量 DOM-to-PNG 库)
- 截图时强制浅色主题,忽略用户当前 dark mode(分享出去的图必须自洽)
- 菜单设计预留扩展位(未来可加"复制链接""导出 Markdown"等),但本 change 只实现图片分享一项

## Capabilities

### New Capabilities
- `page-image-export`: 将任意 wiki 页面导出为 PNG 图片的能力(预览、下载、复制到剪贴板)

### Modified Capabilities
- 无

## Impact

- 涉及代码:`frontend/src/components/PageViewer.tsx` (新增按钮、loading 态、预览弹窗入口)
- 新增组件:`frontend/src/components/ShareAsImageModal.tsx`
- 新增工具:`frontend/src/lib/share-as-image.ts` (封装截图与剪贴板逻辑,便于复用与测试)
- 新增依赖:`modern-screenshot` (~10KB)
- 后端:无改动
- DB:无改动、无新 migration
- 性能影响:截图操作按需触发,无持续开销;mermaid 异步渲染完成后立即可截,典型页面 <2s
- 浏览器兼容:`navigator.clipboard.write` 需 HTTPS 或 localhost;复制按钮在不支持时降级为仅下载
