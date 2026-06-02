# AI ↔ 知识库交互审计与优化路线图

**日期**：2026-06-02（原版） · 2026-06-03（修订）
**范围**：审计当前 AI 如何理解、搜索、获取、更新知识库，识别缺失/优化/重构点，给出按 ROI 排序的实施路线图。
**性质**：纯研究文档。原版执行时不动代码；修订版反映 2026-06-03 P0 修复 + system prompt 重写后的现状。

> ⚠️ **重要前提变化（2026-06-03）**：原版 audit 写作时未发现一个关键 bug——两个 AI provider 静默丢弃了 `req.SystemPrompt`，整段 system prompt（包括 KnowledgeMap）从未到达模型。修复后 audit 中多项问题的前提变了，详见文末 §6「2026-06-03 修订」。**阅读时以修订部分为准**。

---

## 1. 现状摘要

### 1.1 五个交互面

| 面 | 实现 | 入口 |
|---|---|---|
| **理解**（被动注入） | 每次对话渲染 `buildKnowledgeMap` 5 段进 system prompt | `handler/ai_renderer.go` |
| **搜索**（主动调用） | `lookup_page`（标题精确等值）/ `search_pages`（SQL LIKE） | `handler/ai.go:executeLookupPage / executeSearchPages` |
| **获取** | `read_page`（整页 markdown 全文返回） | `handler/ai.go:executeReadPage` |
| **更新** | 6 个写工具 + permission gate 二次确认 | `handler/ai_react.go` + `engine/engine.go` |
| **记忆** | messages 全量存 DB；滑窗 40 条；`tool_summary` 单行回忆补偿 | `handler/ai.go:AIChat` |

### 1.2 KnowledgeMap 5 段（注入到 system prompt）

```
【知识库概览】         统计 + 最近更新
【知识地图】           完整树（每页带摘要+反链数+标签+ID+状态）
【全局标签索引】       所有标签 + 出现页数
【最近活动】           过去 7 天 wiki_log，最多 20 条
【结构健康检查】       孤儿/重名/死页
【知识缺口】           按顶层分类聚合空页
```

### 1.3 实战数据（截至 2026-06-02）

| 维度 | 数值 |
|---|---|
| 总页面 | **43** |
| 有内容 / 空 | 36 / 7 |
| 摘要 ready / pending / empty | 35 / 1 / 7 |
| 平均页面字符数 | 4,134 |
| 最大页面字符数 | **20,006** |
| 平均出链 / 反链 | 1.23 / 1.42 |
| **死页**（有内容但 0 链接 0 反链） | **13**（占有内容页 36% = 13/36） |
| **打标签的页面**（tags 字段非空数组） | **0** |
| 对话总数 / 消息总数 | 26 / 162 |
| 最长对话消息数 | 21（< 40 滑窗阈值） |
| AI 写入 vs 手工 | 61 vs 15（**AI 占 80%**） |

### 1.4 AI 实战调用模式（从 `tool_summary` 抽样）

主要模式：

```
模式 A（写入循环）：search_pages → read_page → patch_page
模式 B（信息重复获取）：read_page(ID=18) × 5 次，每次 13K-20K 字符
模式 C（lookup 失败回退）：lookup_page("X") → 未找到 → search_pages("X") → 找到
模式 D（搜索噪声）：search_pages 返回的第一条常常是"概览"页，AI 必须二次筛选
```

---

## 2. 问题清单（12 项，按 ROI 排序）

### P0 — 立刻见效，体验改善大

#### 问题 #1：`search_pages` 是 SQL LIKE，无相关性排序、无语义检索 ⭐⭐⭐⭐⭐

**现象**
- `WHERE title LIKE '%q%' OR content LIKE '%q%'` 全表扫
- 排序仅按 `sort_order, id`，最常见返回顺序是"概览页排在最前"
- 不支持词序变换（"Go GC 调优" ↔ "GC 调优原理"）
- 不支持多关键词（无 AND 语义）

**实战影响**
- 模式 D 直接观察到：`search_pages("RESTful API 设计")` 返回 4 个匹配，第 1 个是噪声"概览"页
- 模式 C：AI 用 `lookup_page` 精确等值失败后回退到 `search_pages`，浪费 1 轮 ReAct + 1 次 tool_call_id

**改进方向**
- **方案 A**（轻量）：保留 LIKE，但加排序：标题命中权重 > 内容命中权重，词频累计为相关性分；同时过滤掉 `page_type='overview'` 噪声（或降权）
- **方案 B**（中等）：引入 SQLite FTS5 虚表，BM25 排序；建索引时分词用 jieba 或简单 unicode segmentation
- **方案 C**（重）：引入向量检索（页面 embedding + cosine）；适合长期但增加部署复杂度

**成本估算**
- 方案 A：~3 小时（1 个 PR）
- 方案 B：~8 小时（schema 迁移 + 索引重建 + 工具改造）
- 方案 C：~3 天（要选 embedding model + 存储格式 + 增量索引）

**推荐**：先做方案 A 抢收益，方案 B 作为下一步规划

---

#### 问题 #2：`read_page` 无章节粒度，AI 反复读巨型页面 ⭐⭐⭐⭐⭐

**现象**
- `read_page(page_id)` 整页 markdown 返回，最大 20K 字符
- ID=18 一个 conversation 内被读取 5 次，每次完整内容

**实战影响**
- 单次 tool_result 20K 字 ≈ 7000 tokens
- 一个 ReAct 循环里 read 多次 = tokens 翻倍消耗
- 长对话时 ReAct 上下文撑爆，触发 40 条滑窗后丢失 read 历史，AI 又重新 read（恶性循环）

**改进方向**
- 新参数 `read_page(page_id, section?, depth?)`：
  - `section="## 核心概念"` → 只返回该章节及其子章节
  - `depth=1` → 只返回顶层概要 + 子标题列表
- 默认行为不变（向后兼容），AI 通过 prompt 引导优先用 section
- 工具描述里强烈鼓励："改单段用 section 参数避免整页读取"

**成本估算**：~4 小时（解析 markdown 标题树 + 工具描述更新 + 测试）

---

#### 问题 #3：缺 `list_backlinks` / `list_links` 等结构化导航工具 ⭐⭐⭐⭐

**现象**
- AI 知道某页有 N 个反链（KnowledgeMap 显示数字），但**没有工具可以列出是哪些页**
- 想做"哪些页引用了 X" → 只能 `search_pages` 全表扫文本
- 想做"X 链向哪些页" → 同样只能 search

**实战影响**
- 重构操作（如重命名、合并、拆分页面）困难，AI 不知道连带影响
- 链接维护质量低，**13 个死页**（36% 有内容页）就是这个无能力的副作用

**改进方向**
- 新工具：
  - `list_backlinks(page_id)` — 返回引用了该页的所有页 [{id, title, snippet}]
  - `list_links(page_id)` — 返回该页引用了哪些页
  - `list_children(parent_id, depth?)` — 返回子树（已有数据，只是没暴露给 AI）
  - `find_broken_links(page_id?)` — 找出指向不存在标题的 `[[X]]` 链接
- 数据已就绪（`wiki_pages.links` / `backlinks` 是 JSON 数组）

**成本估算**：~4 小时（4 个简单工具）

---

#### 问题 #4：所有工具输出是自由文本（`[系统] 工具 X 已执行...`），下游靠正则解析 ⭐⭐⭐⭐

**现象**
- 工具结果如：`[系统] 搜索「X」找到 4 个匹配页面：\n\n- [ID=50] Web (有内容)\n  preview...`
- `summarizeToolCall` 用一堆正则从这些文本里 parse 出 ID / count / title（`ai_summary.go`）
- 已有注释承认："已知脆弱点"

**实战影响**
- 工具改一行输出格式 → summarize 静默失败 → tool_summary 变成"X → 完成"通用占位
- AI 自己也要 parse 文本，浪费 token 又容易错
- 多语言切换困难（中文 "已读「X」" 写死在两处）

**改进方向**
- 工具返回 JSON 而非文本：
  ```json
  {"type":"search_pages","query":"X","results":[{"id":50,"title":"Web","preview":"..."}]}
  ```
- AI 接收 JSON tool_result 完全没问题（DeepSeek/Claude 都支持）
- `summarizeToolCall` 直接 unmarshal 结构体而非正则 parse
- Optional：保留一个"human view"字段供日志/调试

**成本估算**：~6 小时（10 个工具的输出结构定义 + summarizer 改造 + 测试 + 兼容历史 tool_result）

---

### P1 — 体验 + 长期可扩展性

#### 问题 #5：KnowledgeMap 每次全树渲染，token 开销随页面数线性增长 ⭐⭐⭐

**现象**
- 当前 43 页 → KnowledgeMap ≈ 3500 tokens/请求
- 加上 wiki_maintainer 系统提示 ≈ 6000 tokens 固定开销/请求
- 页面到 200 → 12000+ tokens 固定开销 → context window 紧张

**实战影响**
- 当前 43 页还能扛，但增长曲线明确
- 每次新对话都重付一遍（无 prompt cache 复用）

**改进方向**
- **分级渲染**：
  - 默认只渲染**有内容的 published** 页（空页可只显示 ID + 标题，不带摘要）
  - 加 `focus_page_id` 时只渲染该子树（已部分实现，但 ReAct 内不用）
  - depth 限制：超过 N 层仅显示"... (N more)"
- **token 预算**：定一个上限（如 4K），超过则截断 + 标记"完整树用 list_children 查"
- **Prompt cache**：把 KnowledgeMap 前缀化（无 dynamic 部分提到最后），让 provider 缓存生效

**成本估算**：~6 小时

---

#### 问题 #6：`lookup_page` 隐式追加 subtree KnowledgeMap ⭐⭐⭐

**现象**
- `lookup_page(title)` 返回页 ID + meta 之后，**额外渲染** 该页的 subtree KnowledgeMap（`handler/ai.go:728`）
- 调用方完全无法预知 token 消耗（可能 100 token 也可能 2000 token）

**实战影响**
- API 名实不符 — 名字暗示"查页面"，实际是"查页面 + 探索子树"
- AI 不清楚副作用，习惯性把 `lookup_page` 当便宜操作

**改进方向**
- 拆成两个工具：
  - `lookup_page(title)` — 只返 ID + 基本 meta（< 200 tokens）
  - `explore_subtree(page_id, depth?)` — 显式探索子树（已有 `list_children` 后可省略）
- 工具描述里写清成本

**成本估算**：~2 小时（如果 #3 已做了 list_children 则更少）

---

#### 问题 #7：标签系统 0 使用率，但全局标签索引段每次渲染 ⭐⭐⭐

**现象**
- 全部 43 页 → 0 页打了标签
- `renderTagIndex` 每次渲染都查、聚合、输出"（空）"或干脆不输出
- system prompt 里有"全局标签索引帮你做跨分类检索"指引，AI 没法用

**实战影响**
- KnowledgeMap 渲染的代码路径白跑（虽然开销小）
- prompt 里写"标签可用"误导 AI

**改进方向**
- **方案 A**（删除）：把标签段从 KnowledgeMap 移除，prompt 里也别提
- **方案 B**（激活）：AI 写页时主动生成 tags（在 `propose_plan` / `create_page` 工具里加 tags 字段 + system prompt 要求每页给 1-3 个标签）
- **方案 C**（半激活）：保留索引但只在有标签时渲染；先观察自然使用率

**推荐**：方案 C（懒激活）+ 在写入工具描述里加一句"建议给页面打 1-3 个相关标签"

**成本估算**：~2 小时

---

#### 问题 #8：摘要降级用页面前 80 字，80 字常是 H1 + 空行 ⭐⭐⭐

**现象**
- 摘要 `failed/pending/empty` 时，`renderSummaryLine` 截前 80 字符当摘要
- 80 字符往往是 `# 标题\n\n> 引用块\n---` 这种 markdown 结构，无信息量
- 当前 1 页 pending，未来重启或新页时会触发

**实战影响**
- AI 看到的"摘要"全是噪声 → 没法用知识地图判断该不该读全文
- 加大问题 #2 的恶化（AI 索性 read_page 全文）

**改进方向**
- 截取策略：
  - 跳过开头的 H1 + 引用块 + 横线
  - 取首段正文，限制 80 字
- 或者：对 empty 状态直接显示"(空内容)"不要截前 80 字假装有摘要

**成本估算**：~1 小时

---

### P2 — 长期价值，但优先级低

#### 问题 #9：滑窗 40 条粗暴截断，可能丢用户长期目标 ⭐⭐

**现象**
- 超过 40 条 → 仅保留最近 40 + 一行"早期 X 条已压缩"提示
- 当前最长对话 21 条，**滑窗尚未真生效**

**改进方向**
- 短期不动（无实战影响）
- 长期考虑：分级 summarization（早期消息生成 1 段会话摘要保留）

**推荐**：先放着，等真有 60+ 消息对话再处理

---

#### 问题 #10：无跨会话长期记忆 ⭐⭐

**现象**
- 每个新会话 system prompt 从零渲染
- 用户偏好（"喜欢通俗讲解" / "更想看原理"）每次重学

**改进方向**
- 新表 `user_preferences`（key-value）
- AI 在对话末尾用工具 `remember(key, value)` 写
- 渲染时把 preferences 段加入 system prompt
- 类似 Claude Code memory 的思路，但范围更窄

**成本估算**：~6 小时

**推荐**：单用户本地工具，价值在但不紧急

---

#### 问题 #11：无内容质量评分 / 推荐补充工具 ⭐⭐

**现象**
- KnowledgeMap 显示"覆盖率"百分比，但 AI 没法主动找到"该补充哪页"
- 死页 13 个，AI 没工具知道

**改进方向**
- 派生工具 `list_pages_needing_attention()` — 返回死页/超短页/超长页/未链接到根
- 主要让 AI 在对话中主动建议

**成本估算**：~3 小时

---

#### 问题 #12：updateOverviewPage 异步重写，时机随便 ⭐

**现象**
- 每次 AIChat 完成都 `go h.updateOverviewPage()` fire-and-forget
- 没去重 / 没限流 / 静默 fail
- 26 次会话 = 26 次重算 overview

**改进方向**
- debounce 或 mark-dirty + 定期 flush
- 但当前数据集小，没痛点

**推荐**：先放着

---

## 3. 推荐实施路线图

### Phase 1 — 立即 (1-2 周, 总计约 17 小时)

**目标**：让 AI 的搜索/获取能力补齐基本盘，token 利用率显著优化

| 顺序 | 任务 | 问题编号 | 估时 | 价值 |
|---|---|---|---|---|
| 1 | `read_page` 加 section 参数 | #2 | 4h | 巨型页面阅读成本立刻砍半 |
| 2 | 新增 `list_backlinks` / `list_links` / `list_children` / `find_broken_links` | #3 | 4h | 解锁结构化导航 + 链接维护能力 |
| 3 | `search_pages` 加排序 + 过滤 overview 噪声（方案 A） | #1 | 3h | 搜索质量明显改善 |
| 4 | 工具返回 JSON 替代自由文本 | #4 | 6h | 让 summarize 不再脆弱 + AI 解析更准 |

Phase 1 结束验收：随便挑一个长对话（如 conversation_id=8 的 21 条消息样本），手工跟踪一遍 AI 的 tool_summary，检查：
- `read_page` 调用是否多数携带 `section` 参数
- 重构/合并/重命名类操作是否用了新的 `list_backlinks`
- `search_pages` 返回的第一条不再是"概览"页
- `tool_summary` 中没有"X → 完成"这种 fallback 占位（说明 JSON 解析全部成功）

### Phase 2 — 中期 (3-4 周, 总计约 11 小时，不含可选的 FTS5)

**目标**：让上下文工程可扩展到 100-200 页规模

| 顺序 | 任务 | 问题编号 | 估时 |
|---|---|---|---|
| 5 | KnowledgeMap 分级渲染 + token 预算 + prompt cache 友好 | #5 | 6h |
| 6 | `lookup_page` 去掉隐式 subtree | #6 | 2h |
| 7 | 摘要降级跳过标题/引用 | #8 | 1h |
| 8 | 写入工具加 tags 提示 + 索引段懒渲染 | #7 | 2h |
| 9 | `search_pages` 升级到 FTS5（方案 B） | #1 续 | 8h（可选；可滑到 Phase 3） |

### Phase 3 — 长期 (按需)

| 顺序 | 任务 | 问题编号 | 估时 |
|---|---|---|---|
| 10 | 跨会话长期记忆 (`user_preferences`) | #10 | 6h |
| 11 | 内容质量推荐工具 | #11 | 3h |
| 12 | 滑窗分级 summarization | #9 | 6h |
| 13 | updateOverviewPage debounce | #12 | 1h |
| 14 | embedding 向量检索（方案 C） | #1 终极 | 3d |

---

## 4. 跨问题的设计原则

整理 12 个问题时浮现的几个共通方向，做后续 spec 时按这些原则展开：

1. **工具输出结构化优先于自由文本** — JSON 是 AI 和后续代码共同的语言
2. **预算化 / 分级渲染** — context 不是免费的，给每段定 token 上限
3. **能力对称** — 给 AI 看的统计（如反链数）必须配套能"展开看"的工具，否则只是装饰
4. **降级要有信息量** — 占位符不要假装是数据，宁可显式说"空"
5. **隐式副作用是 bug** — 工具应该名实相符，副作用必须显式（subtree、tag、文件 IO）

---

## 5. 本文档之后

按用户的选择，本次**不动代码**。后续若同意 Phase 1，挑其中任一项单独立 spec → 走 brainstorming/writing-plans 流程。建议从 **#2 (`read_page` section)** 开始，因为：
- 价值立刻可见（巨型页面阅读成本砍半）
- 不依赖其他改动
- 实现明确，2-4 小时一个 PR

如果想验证 audit 中某个判断的真实性（比如 13 死页是否真的影响 AI 行为），可以单独做一次"AI 重构 X 页"的实验对话来观察。

---

## 6. 2026-06-03 修订

### 6.1 触发本次修订的两件事

1. **发现并修复 P0 bug**（commit `74321bc`）
   `OpenCodeProvider` / `DeepSeekProvider` 的 `Chat` / `StreamChat` 实现都没有把 `req.SystemPrompt` 加到 OpenAI-compatible 请求里——`deepseekRequest` 结构也没有 `system` 字段。结果**整段 system prompt**（wiki_maintainer 角色规范、KnowledgeMap 知识地图、Skill catalog）**从未到达模型**。AI 之前"能用"是因为：
   - `tools` 字段是真传了
   - user message 里夹带的 currentSlug
   - 模型自身训练里有 ReAct 模式
   修复方式：新增 `buildDeepSeekMessages` helper，把 SystemPrompt 当作 `messages[0]={role:"system",content:...}` 发送。

2. **重写 wiki_maintainer system prompt**（commit `e530b5a`）
   按原 audit + system prompt 自身审视的 20 项中，应用了 18 项（2 项 #14 wiki_purpose / #17 user_preferences 需 schema 改保留为待办）。变化要点：
   - 删 `没有 propose_plan,不要试图调用` 反向声明 + `SystemPromptTemplates` 死代码 + 重复 3 次的权限闸门段
   - 知识地图使用指南**前置**到 wikiContext 之前（先讲怎么读，再给数据）
   - 加链接 lookup 验证 / 避免重复 read / 失败降级 / 任务完成回顾 / batch 场景 / 自查规则 / 写作规范 / page_type · content_status 语义
   - 总长度从 ~3.6K 字降到 ~2.8K 字（含 KnowledgeMap 渲染时约 ~6K tokens）

### 6.2 原 audit 12 项的优先级重评

| # | 原判断 | 现状变化 | 新优先级 |
|---|---|---|---|
| 1 | search_pages 无相关性排序 | KnowledgeMap 现在真到 AI 手里，多数查询能在地图里定位再 read_page，搜索压力降低 | P0 → **P1** |
| 2 | read_page 无章节粒度 | prompt 新加"避免重复 read + 看 tool_summary 回忆"，需观察 1-2 个真实长对话再定 | **观察后定** |
| 3 | 缺 list_backlinks / list_links / list_children / find_broken_links | 没变；prompt 新加自查规则但没工具支持，规则与能力错配 | **P0**（与自查规则配套） |
| 4 | 工具输出自由文本 | 没变；脆弱性还在但优先级下降 | P0 → **P1** |
| 5 | KnowledgeMap 全树渲染 token 开销 | **从 0 变 ~6000 tokens/请求**——之前被 P0 屏蔽看不到，现在真消耗 | 见 §6.3 新 **#13** |
| 6 | lookup_page 隐式追加 subtree | prompt 新规则"链接前 lookup_page 验证" → 调用频次会显著上升 → 隐式副作用更碍事 | P1 → **P0** |
| 7 | 标签段死代码 | prompt 不再提标签，索引段 0 标签时不渲染。短期无影响 | P1 → **P2**（独立 spec） |
| 8 | 摘要前 80 字降级 | 没变 | P1 |
| 9 | 滑窗 40 条粗暴 | 没变，最长对话仍 21 条未触发 | P2 |
| 10 | 无跨会话长期记忆 | 没变。audit 已建议 user_preferences | P2（与原 #17 合并独立 spec） |
| 11 | 无内容质量推荐工具 | prompt 新加"自查/主动建议"，部分缓解；但 AI 没工具看到"哪些页该被引用" | **与 #3 合并** |
| 12 | updateOverviewPage 异步重写 | 没变；当前数据集小无痛点 | P2 |

### 6.3 P0 修复 + prompt 重写后**新增**的 audit 项

#### #13 ⭐⭐⭐⭐⭐ system prompt cache 不友好

**现象**
- 每次请求 system prompt ~6000 tokens 完整发送
- 当前 prompt 顺序：静态角色规范 → 当前日期（动态）→ KnowledgeMap 使用指南（静态）→ `[wikiContext]`（动态）
- OpenAI / DeepSeek 的 prompt cache 按**前缀**缓存命中，中间嵌动态字段会破坏后面所有静态部分的缓存命中
- 当前形态：前缀的"静态角色规范"段确实能缓存，但其后任何静态内容（指南、wikiContext 末尾的固定提示）都缓存不到

**影响**
- 每次新对话 / 每轮 ReAct 都全额支付 ~6000 tokens 输入
- DeepSeek 标价：cache hit $0.07/M vs cache miss $0.27/M，~4x 差距
- 当前 26 对话 + 162 消息估算月输入成本不大（页面 43），但**增长曲线明确**：页面到 200 → KnowledgeMap 撑到 12K+ tokens

**改进方向**
- **方案 A**（轻量，0 schema 改）：调整 system prompt 顺序——所有静态内容放最前面（角色 + 工具集 + 工作流 + 规范 + 知识地图使用指南 + page_type 语义 + 写作规范），所有动态内容（当前日期 + wikiContext）拉到末尾
- **方案 B**（中等）：把 wikiContext 拆出来不进 system prompt，作为一条 `role: user` 的固定上下文消息发送（OpenAI cache 也能缓存历史 user 消息前缀），但破坏现有 BuildSystemPrompt API
- **方案 C**（深度）：增量更新 KnowledgeMap——只有结构变化时重渲染并写入一个"会话级 cache key"，否则复用上次

**推荐**：方案 A，~3h 实现 + 测试，立即生效

#### #14 ⭐⭐⭐ update_page 不支持 tags 字段

**现象**
- 重写 prompt 时本想加 tags 提示鼓励 AI 写页时打标签（呼应原 #7）
- 检查 `engine/engine.go:execUpdatePage` line 287 发现：**update 时 tags 字段被强制保留旧值**，传入新 tags 会被静默忽略
- create_page 倒是支持（line 161 `tags := strVal(params, "tags", "[]")`），但格式约定不清（JSON 数组 vs 逗号分隔）

**影响**
- 如果未来激活标签（原 #7 / #4-方案-B），必须先修这个，否则 AI 信 prompt 调用后被吞
- 当前规避：prompt 暂不提标签写入

**改进方向**
与原 #7 一起处理：定义 tags 字段格式（建议 JSON 数组）+ update_page 支持 tags 参数 + `tags_normalized` 同步逻辑 + AI 工具 schema 加 tags + prompt 鼓励打标签

**推荐**：与原 #7 合并到一个"标签激活" spec

### 6.4 修订后路线图

#### Phase 1 — 立即 (~14h)

| 顺序 | 任务 | 项 | 估时 | 价值 |
|---|---|---|---|---|
| 1 | system prompt 顺序调整使 cache 命中（方案 A） | **#13** | 3h | 立省 ~75% 输入 token 成本 |
| 2 | `list_backlinks` / `list_links` / `list_children` / `find_broken_links` 四个工具 | #3 + #11 | 4h | 解锁自查 + 结构化导航能力 |
| 3 | `lookup_page` 去掉隐式 subtree 副作用（拆 explore_subtree，或直接砍） | #6 | 2h | 避免链接验证规则引发的 token 爆炸 |
| 4 | `read_page` 加 section 参数 | #2 | 4h | **先观察 2 天再做**——看 prompt 新规则是否已经缓解了重复 read |
| 5 | 摘要降级用首段而非前 80 字 | #8 | 1h | 知识地图可读性提升 |

#### Phase 2 — 中期 (~10h)

| 顺序 | 任务 | 项 | 估时 |
|---|---|---|---|
| 6 | search_pages 排序优化（标题权重 + 过滤 overview 噪声）方案 A | #1 | 3h |
| 7 | 工具输出 JSON 化 | #4 | 6h |
| 8 | search_pages FTS5（方案 B）— 可选 | #1 续 | 8h（滑到 Phase 3） |

#### Phase 3 — 按需

| 顺序 | 任务 | 项 | 估时 |
|---|---|---|---|
| 9 | 标签激活（含 update_page tags 支持） | #7 + **#14** | 6h |
| 10 | 跨会话长期记忆（user_preferences / wiki_purpose） | #10 + 原 spec #14/#17 | 8h |
| 11 | 滑窗分级 summarization | #9 | 6h |
| 12 | updateOverviewPage debounce | #12 | 1h |
| 13 | embedding 向量检索（方案 C） | #1 终极 | 3d |

### 6.5 修订摘要

| 维度 | 原版（2026-06-02） | 修订（2026-06-03） |
|---|---|---|
| Phase 1 任务数 | 4 | 5 |
| Phase 1 估时 | 17h | 14h |
| 新 P0 项 | — | **#13 prompt cache** |
| 升级 P0 | — | #3（与 #11 合并）、#6 |
| 降级 | — | #1（P0→P1）、#4（P0→P1）、#7（P1→P2） |
| 已修复 | — | **P0 bug** (commit `74321bc`，provider 丢 SystemPrompt) + **system prompt 重写** (commit `e530b5a`，独立审视清单 20 项中应用 18 项) |
| 待办 audit 项 | 12 | 12 + 2 新 = 14（其中 5 项 P0、3 项 P1、6 项 P2/独立 spec） |

### 6.6 下一步建议

按 §6.4 路线图，建议立即推进 **#13 (prompt cache 顺序调整)**——3h 一个 PR，回报立即可量化（看 DeepSeek dashboard 的 cache hit rate）。然后 **#3+#11 (导航工具)** 让自查规则真正可执行。`#2 read_page section` 故意放后面，等收集 2-3 天 prompt 重写后的 tool_summary 看 AI 是否还重复 read 同一页。

