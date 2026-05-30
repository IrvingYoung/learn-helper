# Design: 添加知识点详细讲解内容

## Context

**当前状态**：
- `topics` 表仅有 `name`, `description`, `key_points` 字段
- `exercises` 表仅有 `title`, `description`, `solution_outline` 字段
- 13 个知识点和 6 道练习题内容严重不足

**约束**：
- 使用 SQLite 数据库，需向后兼容
- 前端使用 React + TypeScript，需支持 Markdown 渲染
- 计划使用 AI 生成内容，需设计便于 AI 生成的格式

## Goals / Non-Goals

**Goals:**
- 扩展数据库 schema，支持完整讲解内容
- 设计便于 AI 生成和人类审核的数据格式
- 支持多语言代码示例（Python 为主）
- 前端能够渲染 Markdown 和代码高亮

**Non-Goals:**
- 不做知识点的多媒体内容（图片、视频）
- 不做知识点间的链接关系管理（已有 parent_id）
- 不做内容版本管理和历史记录

## Decisions

### 1. 数据格式：JSON 存储结构化内容

**选择**：将 `content`, `code_examples`, `common_mistakes` 等设计为 JSON 字段

**理由**：
- 结构化便于前端解析和渲染
- AI 生成时格式一致，易于校验
- 支持嵌套结构（如代码示例可包含语言、注释、复杂度）

```json
// code_examples 示例
[
  {"lang": "python", "code": "...", "explanation": "..."},
  {"lang": "go", "code": "...", "explanation": "..."}
]

// common_mistakes 示例
["递归时忘记 base case", "层序遍历队列判空遗漏"]
```

**替代方案考虑**：
- 纯文本存储 → 不便于查询和渲染
- 拆分为独立表 → 过度设计，单用户场景不需要

### 2. Content 内容结构

**设计**：Content 字段为 Markdown 格式，包含预定义的章节结构

```
## 概念定义
## 核心操作
## 遍历方式（图解）
## 复杂度分析
## 常见变形
## 实战应用
## 常见错误
```

**理由**：
- Markdown 便于 AI 生成和人类阅读
- 固定结构便于前端渲染和样式统一
- 可扩展（后续可加新章节）

### 3. AI 生成策略

**Prompt 模板设计**：
```
为知识点「{name}」生成完整学习资料，格式如下：

## 概念定义
（2-3 段落，解释核心概念）

## 核心操作
（代码 + 说明，Python 为主）

## 遍历方式
（如适用，二叉树类需包含前/中/后序）

## 复杂度分析
（表格形式，时间/空间复杂度）

## 常见变形
（如适用，BST/平衡树等）

## 实战应用
（2-3 个实际使用场景）

## 常见错误
（3-5 条，与 common_mistakes 字段对应）
```

### 4. 数据库迁移策略

**方式**：新增 SQL 迁移文件，不修改原 schema

```sql
-- 001_add_topic_content.sql
ALTER TABLE topics ADD COLUMN content TEXT;
ALTER TABLE topics ADD COLUMN code_examples TEXT;
ALTER TABLE topics ADD COLUMN common_mistakes TEXT;

ALTER TABLE exercises ADD COLUMN solution_detail TEXT;
ALTER TABLE exercises ADD COLUMN common_errors TEXT;
```

**理由**：
- SQLite 支持 ADD COLUMN
- 不影响现有数据
- 可回滚（删除列）

### 5. 前端渲染方案

**选择**：使用 `react-markdown` + `react-syntax-highlighter`

**理由**：
- 成熟稳定，社区活跃
- 支持 GFM（GitHub Flavored Markdown）
- 代码高亮支持多语言

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| AI 生成内容有 factual errors | 人工抽检 20%，重点检查概念准确性和代码可运行性 |
| Content 字段过长影响性能 | SQLite TEXT 无限制，但建议单知识点控制在 10KB 以内 |
| Markdown 格式不统一 | 提供 Prompt 模板约束格式；前端做基本解析容错 |
| 代码示例需要更新 | 初期以 Python 为主；后续按需扩展其他语言 |

## Migration Plan

1. **Phase 1: 数据库迁移**
   - 执行 001_add_topic_content.sql
   - 验证字段添加成功

2. **Phase 2: 后端适配**
   - 更新 sqlc 生成的 model
   - 修改 queries.sql 添加新字段查询
   - 验证 API 返回完整数据

3. **Phase 3: 前端适配**
   - 安装 react-markdown, react-syntax-highlighter
   - 更新知识点/练习详情页组件
   - 测试 Markdown 渲染和代码高亮

4. **Phase 4: 内容生成**
   - 使用 AI 生成现有 13 个知识点 + 6 道题的讲解
   - 人工抽检质量
   - 批量导入数据库

5. **回滚方案**：
   - 数据库：删除新增列
   - 后端：回滚 model/query 改动
   - 前端：移除渲染组件