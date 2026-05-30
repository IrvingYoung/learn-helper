# Tasks: 添加知识点详细讲解内容

## 1. 数据库迁移

- [x] 1.1 创建迁移文件 `001_add_topic_content.sql`
- [x] 1.2 执行迁移，验证字段添加成功
- [ ] 1.3 更新 sqlc 配置（可选）

## 2. 后端适配

- [x] 2.1 重新生成 sqlc model（获取新字段）
- [x] 2.2 更新 queries.sql 添加新字段查询
- [x] 2.3 修改 GET `/api/topics/:slug` 返回 content/code_examples/common_mistakes
- [x] 2.4 修改 GET `/api/exercises/:id` 返回 solution_detail/common_errors
- [x] 2.5 添加 PUT `/api/exercises/:id/solution` 更新解法接口
- [x] 2.6 添加 PUT `/api/topics/:slug/content` 更新知识点内容接口
- [x] 2.7 添加 POST `/api/topics/batch-content` 批量更新知识点内容接口
- [x] 2.8 验证 API 返回完整数据

## 3. 前端依赖安装

- [x] 3.1 安装 `react-markdown`
- [x] 3.2 安装 `react-syntax-highlighter`
- [x] 3.3 安装 `@types/react-syntax-highlighter`（如需要）

## 4. 前端组件开发

- [x] 4.1 创建 `MarkdownRenderer` 组件（支持代码高亮）
- [x] 4.2 更新知识点详情页，展示 content、code_examples
- [x] 4.3 更新练习题详情页，展示 solution_detail、common_errors
- [x] 4.4 测试 Markdown 渲染效果

## 5. 内容生成（AI）

- [x] 5.1 设计 AI 生成 Prompt 模板
- [x] 5.2 为"数组"知识点生成 content/code_examples/common_mistakes
- [x] 5.3 为"链表"知识点生成 content/code_examples/common_mistakes
- [x] 5.4 人工抽检内容质量（2-3 个知识点）
- [x] 5.5 批量生成剩余知识点（11 个）
- [x] 5.6 为现有 6 道练习题生成 solution_detail
- [x] 5.7 导入数据到数据库
- [x] 5.8 内容质量抽检（抽检 20%，发现问题则回滚）

## 6. 验证与测试

- [x] 6.1 测试知识点详情页完整展示
- [x] 6.2 测试练习题详情页完整展示
- [x] 6.3 验证代码高亮效果
- [x] 6.4 验证 JSON 字段解析正确