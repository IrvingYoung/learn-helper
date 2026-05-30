# 知识点内容生成 Prompt

## Topic Content Prompt

请为知识点「{topic_name}」生成完整的学习资料。

### Topic Information

- **名称**: {name}
- **Slug**: {slug}
- **描述**: {description}
- **难度**: {difficulty}
- **关键要点**: {key_points}

### Output Format

请按照以下格式生成内容，并返回 JSON 格式：

```json
{
  "content": "## 概念定义\n\n（2-3 段落解释核心概念）\n\n## 核心操作\n\n（代码 + 说明）\n\n## 遍历方式\n（如二叉树类包含前/中/后序）\n\n## 复杂度分析\n\n（表格形式，时间/空间复杂度）\n\n## 常见变形\n（如 BST/平衡树等）\n\n## 实战应用\n\n（2-3 个实际使用场景）\n\n## 常见错误\n\n（3-5 条，与 common_mistakes 对应）",
  "code_examples": [
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
  ],
  "common_mistakes": ["错误描述1", "错误描述2", "错误描述3"]
}
```

### Requirements

1. **Content**: 使用 Markdown 格式，包含所有章节（如适用）
2. **Code Examples**: 以 Python 为主，可选 Go 示例
3. **Common Mistakes**: 3-5 条，每条 20-50 字
4. **语言**: 中文
5. **代码**: 确保语法正确、可运行
6. **概念**: 确保准确，无 factual errors

---

## Exercise Solution Prompt

请为练习题「{exercise_title}」生成详细的解法内容。

### Exercise Information

- **题目**: {title}
- **描述**: {description}
- **难度**: {difficulty}
- **标签**: {tags}

### Output Format

```json
{
  "solution_detail": "## 解法一：暴力枚举\n\n### 思路\n\n### 代码实现\n\n### 复杂度分析\n\n## 解法二：哈希优化\n\n### 思路\n\n### 代码实现\n\n### 复杂度分析\n\n## 最优解法\n\n### 思路\n\n### 代码实现\n\n### 复杂度分析\n\n## 关键点总结\n\n（3-5 条核心要点）",
  "common_errors": [
    "边界条件处理错误：数组越界",
    "循环终止条件遗漏",
    "类型转换错误"
  ]
}
```

### Requirements

1. **Solution Detail**: Markdown 格式，包含 2-3 种解法（从暴力到最优）
2. **Common Errors**: 3-5 条常见的错误
3. **代码**: 确保正确、可运行
4. **复杂度分析**: 包含时间复杂度和空间复杂度
