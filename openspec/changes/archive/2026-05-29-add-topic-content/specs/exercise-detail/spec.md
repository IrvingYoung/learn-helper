# Spec: exercise-detail

练习题详细解法内容管理

## Overview

管理练习题的完整解法内容，包括多种解法、完整代码、常见错误等。

## Data Model

### Exercises 表扩展字段

| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `solution_detail` | TEXT | Markdown | 详细解法（含多种解法） |
| `common_errors` | TEXT | JSON | 常见错误列表 |

### Solution Detail 内容结构

```markdown
## 解法一：暴力枚举
### 思路
### 代码实现
### 复杂度分析

## 解法二：哈希优化
### 思路
### 代码实现
### 复杂度分析

## 最优解法
### 思路
### 代码实现
### 复杂度分析

## 关键点总结
（3-5 条核心要点）
```

### Common Errors 格式

```json
[
  "边界条件处理错误：数组越界",
  "循环终止条件遗漏",
  "类型转换错误"
]
```

## Behavior

### 解法查询

- GET `/api/exercises/:id` 返回 solution_detail、common_errors
- 无 solution_detail 时返回 null，显示"暂无详细解法"

### 解法更新

- PUT `/api/exercises/:id/solution` 更新 solution_detail
- PUT `/api/exercises/:id/errors` 更新 common_errors

**更新接口规格**：

```
PUT /api/exercises/:id/solution
Content-Type: application/json

{
  "solution_detail": "..."
}

PUT /api/exercises/:id/errors
Content-Type: application/json

{
  "common_errors": ["错误1", "错误2"]
}
```

### 数据验证

- `common_errors` 需为有效 JSON 数组，最多 10 条
- `solution_detail` 需为有效 Markdown，不做格式校验
- 单条错误描述最多 200 字符

### 分层提示

- hints 字段已存在（3 层提示）
- 与 solution_detail 配合使用：提示 → 解法

## Acceptance Criteria

- [ ] 数据库新增 2 个字段
- [ ] API 返回完整 exercise 内容
- [ ] 前端支持 Markdown 渲染（解法）
- [ ] 前端支持 JSON 渲染（常见错误列表）
- [ ] AI 可生成符合格式的 solution_detail