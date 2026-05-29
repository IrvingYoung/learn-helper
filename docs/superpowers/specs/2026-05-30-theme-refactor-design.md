# LLM Wiki 主题重构设计

| 项目 | 内容 |
|------|------|
| 日期 | 2026-05-30 |
| 作者 | Claude |
| 状态 | 待实现 |

## 1. 设计目标

将现有 LLM Wiki 前端重构为支持**双主题切换**：

- **Warm 主题**（默认）：纸质书卷感，Playfair Display + Source Sans 3，暖棕色调
- **Dark Tech 主题**：深色科技风，Inter + JetBrains Mono，紫/青霓虹色调

用户可在运行时一键切换主题，所有组件自动响应。

## 2. 主题实现方案

### 2.1 方案选择：CSS 自定义属性 + data-theme

使用 CSS 自定义属性（CSS Variables）定义颜色，通过 `data-theme` 属性切换。

**原因：**
- 运行时切换无需重新构建
- Tailwind v3 可通过 `bg-[var(--xxx)]` 语法使用
- 支持任意数量主题扩展
- 与现有组件架构兼容性好

### 2.2 颜色变量映射

#### Warm 主题（默认）

```css
--bg-primary: #faf9f7
--bg-secondary: #ffffff
--bg-tertiary: #f5f3ef
--border: #eae7e0
--border-hover: #d9d5cc
--text-primary: #1a1815
--text-secondary: #4a4640
--text-muted: #8a857a
--accent: #c45c26
--accent-light: #e87d3a
--accent-bg: #fdf6f0
--success: #2d7a3e
--warning: #c78a2a
--error: #dc2626
```

#### Dark Tech 主题

```css
--bg-primary: #0a0a0f
--bg-secondary: #111118
--bg-tertiary: #16161e
--border: #2a2a35
--border-hover: #3a3a4a
--text-primary: #e2e2e8
--text-secondary: #a0a0b0
--text-muted: #6a6a7a
--accent: #7c3aed
--accent-light: #a78bfa
--accent-bg: rgba(124, 58, 237, 0.1)
--success: #22c55e
--warning: #f59e0b
--error: #ef4444
```

### 2.3 字体配置

| 主题 | Display 字体 | Body 字体 | Mono 字体 |
------|-------------|----------|----------|
| Warm | Playfair Display | Source Sans 3 | Source Sans 3 |
| Dark | Inter | Inter | JetBrains Mono |

通过 CSS 变量 `--font-display` 和 `--font-body` 切换。

## 3. 组件改造计划

### 3.1 新增/改造文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/contexts/ThemeContext.tsx` | 新增 | ThemeProvider + useTheme hook |
| `src/styles/themes.css` | 新增 | CSS 变量定义（两套主题） |
| `src/styles/fonts.css` | 新增 | Google Fonts 加载 |
| `tailwind.config.js` | 修改 | 添加 fontFamily 扩展 |
| `src/index.css` | 修改 | 导入 themes.css、fonts.css |
| `src/components/WikiPage.tsx` | 修改 | 应用主题类名 |
| `src/components/KnowledgeTree.tsx` | 修改 | 应用主题类名 |
| `src/components/ChatPanel.tsx` | 修改 | 应用主题类名 |
| `src/components/PageViewer.tsx` | 修改 | 应用主题类名 |
| `src/components/MarkdownContent.tsx` | 修改 | 暗色代码高亮 |

### 3.2 类名映射规则

| 语义 | Warm 类名 | Dark 类名 | 统一类名 |
|------|----------|----------|---------|
| 页面背景 | `bg-[#faf9f7]` | `bg-[#0a0a0f]` | `bg-bg-primary` |
| 卡片背景 | `bg-white` | `bg-[#16161e]` | `bg-bg-secondary` |
| 边框 | `border-[#eae7e0]` | `border-[#2a2a35]` | `border-border` |
| 主文字 | `text-[#1a1815]` | `text-[#e2e2e8]` | `text-text-primary` |
| 次文字 | `text-[#4a4640]` | `text-[#a0a0b0]` | `text-text-secondary` |
| 弱化文字 | `text-[#8a857a]` | `text-[#6a6a7a]` | `text-text-muted` |
| 强调色 | `text-[#c45c26]` | `text-[#7c3aed]` | `text-accent` |
| 强调背景 | `bg-[#fdf6f0]` | `bg-accent-bg` | `bg-accent-bg` |

## 4. ThemeContext API

```typescript
interface ThemeContextType {
  theme: 'warm' | 'dark';      // 当前主题
  setTheme: (theme: 'warm' | 'dark') => void;
  toggleTheme: () => void;
}

// Usage
const { theme, setTheme, toggleTheme } = useTheme();
```

- 主题状态持久化到 `localStorage`
- 初始化时读取 `localStorage`，默认 `warm`
- 切换时设置 `document.documentElement.dataset.theme`

## 5. 自定义 Tailwind 类名

在 `tailwind.config.js` 中扩展：

```js
module.exports = {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        display: ['var(--font-display)', 'serif'],
        body: ['var(--font-body)', 'sans-serif'],
        mono: ['var(--font-mono)', 'monospace'],
      },
      colors: {
        // 使用 CSS 变量，Tailwind 会在编译时处理
        'bg-primary': 'var(--bg-primary)',
        'bg-secondary': 'var(--bg-secondary)',
        'bg-tertiary': 'var(--bg-tertiary)',
        'border-custom': 'var(--border)',
        'text-primary': 'var(--text-primary)',
        'text-secondary': 'var(--text-secondary)',
        'text-muted': 'var(--text-muted)',
        'accent': 'var(--accent)',
        'accent-light': 'var(--accent-light)',
        'accent-bg': 'var(--accent-bg)',
      },
    },
  },
  plugins: [],
}
```

## 6. MarkdownContent 暗色适配

`react-syntax-highlighter` 需要支持暗色主题：

- Warm 主题：继续使用 `oneLight`
- Dark 主题：切换为 `oneDark` 或 `vscDarkPlus`

通过 ThemeContext 获取当前主题，动态选择高亮样式。

## 7. 实现顺序

1. **Phase 1: 基础设施**
   - 创建 `ThemeContext.tsx`
   - 创建 `themes.css`（两套主题变量）
   - 创建 `fonts.css`
   - 更新 `tailwind.config.js`
   - 更新 `index.css`

2. **Phase 2: 组件改造**
   - `WikiPage.tsx` — 三栏布局 + Header
   - `KnowledgeTree.tsx` — 知识树
   - `ChatPanel.tsx` — 聊天面板
   - `PageViewer.tsx` — 页面浏览器

3. **Phase 3: 细节打磨**
   - `MarkdownContent.tsx` — 暗色代码高亮
   - 自定义滚动条样式
   - 过渡动画

## 8. 验收标准

- [ ] 页面正常显示 Warm 主题（默认）
- [ ] 点击切换按钮可切换到 Dark 主题
- [ ] 切换后所有组件颜色正确变化
- [ ] 刷新页面后主题状态保持
- [ ] Markdown 代码块在 Dark 主题下使用暗色高亮
- [ ] 无视觉回归（布局、间距保持不变）
