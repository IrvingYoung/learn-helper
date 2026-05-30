# wiki_pages 树模型重构：Materialized Path 设计

| 项目 | 内容 |
|------|------|
| 日期 | 2026-05-30 |
| 作者 | Claude |
| 状态 | 待实现 |

## 1. 设计目标

将 wiki_pages 的树结构从**邻接表（Adjacency List）** 重构为 **Materialized Path（物化路径）**，解决两个核心痛点：

- **AI 侧查子树慢**：邻接表需要全量加载或递归查询才能获取某个节点下的所有子孙
- **前端树展示需要全量数据加载**：同样的问题，即使只展示一小部分子树也需要加载整个森林

## 2. 方案选择：ID 路径

选择用节点 ID 拼接路径（如 `"1/3/7/"`），而不是 slug 路径或自定义编码。

**原因：**
- ID 不可变，不会因重命名导致路径更新
- 路径格式简明，长度可控
- SQLite 对 `LIKE 'prefix/%'` 前缀匹配可走索引
- 配合保留 `parent_id`，兼顾邻接表的直接父节点查询效率

## 3. Schema 变更

### 3.1 新增 path 列 + 索引

```sql
ALTER TABLE wiki_pages ADD COLUMN path TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);
```

- `parent_id` **保留不动**——查直接父节点时比从 path 解析快，且 INSERT 时更方便
- 根节点 path 形如 `"7/"`（自己 ID + `/`）
- 子节点 path 形如 `"1/3/7/"`（父节点 path + 自己 ID + `/`）
- 末尾带 `/` 保证 `LIKE '1/3/%'` 不会误匹配 `1/30/`

### 3.2 数据模型层

`backend/internal/model/models.go` 中 WikiPage 结构体新增 `Path string` 字段。

## 4. 核心操作逻辑

### 4.1 创建页面

```sql
INSERT INTO wiki_pages (title, slug, parent_id, ...) VALUES (?, ?, ?, ...);
-- 取得 last_insert_id

UPDATE wiki_pages SET path =
  CASE WHEN parent_id IS NULL THEN last_insert_id || '/'
       ELSE (SELECT path FROM wiki_pages WHERE id = parent_id) || last_insert_id || '/'
  END
WHERE id = last_insert_id;
```

两条 SQL 比一条复杂 INSERT 更清晰，且避免了在 Go 里预计算或查两次。

### 4.2 查子树（主要性能收益）

```sql
-- 查询 /go/basics (id=3, path="1/3/") 下的所有子节点
SELECT * FROM wiki_pages
WHERE path LIKE '1/3/%'
ORDER BY path, sort_order, id;
```

**对比之前**：全量加载所有页面 → Go 中遍历构建树 → 过滤。改为直接 SQL 过滤 + 索引扫描，从 O(N) 降到 O(M)（M 为目标子树节点数）。

### 4.3 移动页面

```go
// 1. 查源节点旧 path
oldPath := "1/3/"
// 2. 查目标父节点的新 path
newParentPath := "1/2/5/"
// 3. 新 path = newParentPath + oldPath 最后一段
newPath := newParentPath + "3/"
// 4. 批量更新
UPDATE wiki_pages SET path = REPLACE(path, '1/3/', '1/2/5/3/')
WHERE path LIKE '1/3/%' OR id = 3;
```

- 禁止将页面移到自己或自己的子孙下面——通过 path 检测更简单：`newPath LIKE oldPath || '%'` 说明目标在新路径的子树上（注意 `newPath` 和 `oldPath` 都含尾部 `/`，精确匹配不会混淆不同 ID 段）
- 批量 UPDATE 是原子操作

### 4.4 删除页面

现有 `DeleteWikiPage` 行为不变（只删自身）。子节点提升为根级节点，其 path 自动更新：

```sql
-- 如删除 id=3 (path="1/3/")
-- 子节点的 path 从 "1/3/7/" 变为 "7/"
UPDATE wiki_pages SET path = REPLACE(path, '1/3/', '')
WHERE path LIKE '1/3/%';
```

如果希望级联删除，后续可单独加选项。

### 4.5 查直接子节点（保持不变）

邻接表优势场景不变：

```sql
SELECT * FROM wiki_pages WHERE parent_id = ? ORDER BY sort_order, id;
```

## 5. 数据迁移

已有数据需要批量回填 `path`。因为当前结构是邻接表且数据量小（个人知识库），用一次 Go 脚本在内存做 BFS 即可：

```go
// 1. 加载所有页面到 map[id]page
// 2. 从根节点（parent_id IS NULL）开始 BFS
// 3. 逐层计算 path = parent.path + id + "/"
// 4. UPDATE wiki_pages SET path = ? WHERE id = ?
```

无需多次执行——数据量小，内存 BFS 一次完成。

## 6. API 变更

### 6.1 GET /api/wiki（知识树）

返回的 `WikiTreeNode` 新增 `path` 字段。

```json
{
  "tree": [
    {"id": 1, "title": "Go", "path": "1/", "parent_id": null, "children": [...]}
  ]
}
```

前端渲染逻辑不变——path 只是额外元数据，递归结构不变。

### 6.2 AI context 构建（主要收益点）

`buildWikiContext` 改为：

- **不传目标节点时**：保持现有行为（全量加载构建树，用于 AI 获取全局视野）
- **AI 指定了目标节点时**（如"总结 Go 基础语法"——AI 已定位到具体页面 ID）：用 `LIKE path || '%'` 查子树，只返回相关子结构
- 两种模式互斥，AI 通过工具调用参数控制

## 7. 涉及文件（按影响面排列）

| 文件 | 改动 |
|------|------|
| `backend/db/migrations/004_materialized_path.sql` | 新增：path 列 + 索引 + 数据迁移 |
| `backend/internal/model/models.go` | WikiPage 结构体加 Path 字段 |
| `backend/internal/model/queries.sql` | GetWikiPageTree 加 path；新增 GetSubtreePages 查询 |
| `backend/internal/handler/wiki.go` | GetWikiTree 返回 path；Create/Move/Delete 同步更新 path |
| `backend/internal/handler/ai.go` | buildWikiContext 支持按需查子树 |
| `frontend/src/types/index.ts` | WikiTreeNode 加 path 字段 |
| `frontend/src/components/KnowledgeTree.tsx` | 传递 path（渲染不变） |

## 8. 不做的事

- 不改 `sort_order` 语义（全局排序够用）
- 不删 `parent_id`（兼容性和直接父节点查询）
- 不引入闭包表等额外结构（Materialized Path 已满足需求）
- 不改前端递归渲染逻辑（path 只是传递，不改变数据结构）
