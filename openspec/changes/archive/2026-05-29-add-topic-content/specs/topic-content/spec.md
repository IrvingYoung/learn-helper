# Spec: topic-content

知识点详细讲解内容管理

## Overview

管理知识点的完整学习资料，包括概念定义、代码示例、常见错误等。

## Data Model

### Topics 表扩展字段

| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `content` | TEXT | Markdown | 详细讲解内容 |
| `code_examples` | TEXT | JSON | 代码示例数组 |
| `common_mistakes` | TEXT | JSON | 常见错误列表 |

### Content 内容结构

```markdown
## 概念定义
（2-3 段落解释核心概念）

## 核心操作
（代码 + 说明）

## 遍历方式（可选）
（如二叉树类包含前/中/后序）

## 复杂度分析
（表格形式）

## 常见变形（可选）
（如 BST/平衡树）

## 实战应用
（2-3 个场景）

## 常见错误
（与 common_mistakes 对应）
```

### Code Examples 格式

```json
[
  {
    "lang": "python",
    "code": "def example():\n    pass",
    "explanation": "代码说明"
  },
  {
    "lang": "go",
    "code": "func example() {}",
    "explanation": "代码说明"
  }
]
```

### Common Mistakes 格式

```json
["错误描述1", "错误描述2", "错误描述3"]
```

### key_points vs common_mistakes 区别

| 字段 | 用途 | 时机 | 内容特点 |
|------|------|------|----------|
| `key_points` | 快速回顾 | 学习时快速浏览 | 核心概念要点列表 |
| `common_mistakes` | 错误预防 | 做题前/后查看 | 容易犯的错误列表 |

两者用途不同，可并存。key_points 侧重"学什么"，common_mistakes 侧重"防什么错"。

## Behavior

### 内容查询

- GET `/api/topics/:slug` 返回完整 content、code_examples、common_mistakes
- 无 content 时返回 null，前端显示"暂无详细讲解"

### 内容更新

- PUT `/api/topics/:slug/content` 更新单个知识点 content
- POST `/api/topics/batch-content` 批量更新知识点内容

**批量更新接口规格**：

```
POST /api/topics/batch-content
Content-Type: application/json

Request Body:
{
  "items": [
    {"slug": "array", "content": "...", "code_examples": "[...]", "common_mistakes": "[...]"},
    {"slug": "linked-list", "content": "...", "code_examples": "[...]", "common_mistakes": "[...]"}
  ]
}

Response:
{
  "updated": 2,
  "failed": []
}
```

- 单次请求限制：最多 50 条
- 原子性：部分失败不影响已成功的条目
- 失败条目记录在 `failed` 数组中

### 内容验证

- `lang` 字段只允许：`python`, `go`, `javascript`, `java`, `cpp`, `c`
- `code` 字段不做语法校验，但需为非空字符串
- `common_mistakes` 需为有效 JSON 数组，最多 10 条
- `content` 需为有效 Markdown，不做格式校验

## Acceptance Criteria

- [ ] 数据库新增 3 个字段
- [ ] API 返回完整 topic 内容
- [ ] 前端支持 Markdown 渲染
- [ ] 前端支持代码高亮
- [ ] AI 可生成符合格式的 content