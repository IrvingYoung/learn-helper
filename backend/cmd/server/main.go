package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"learn-helper/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
)

// Schema SQL - inline to avoid external file dependency for initialization
const schemaSQL = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER REFERENCES topics(id),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    key_points TEXT,
    difficulty TEXT DEFAULT 'beginner' CHECK(difficulty IN ('beginner', 'intermediate', 'advanced')),
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER NOT NULL REFERENCES topics(id),
    type TEXT DEFAULT 'algorithm' CHECK(type IN ('algorithm', 'system_design', 'knowledge')),
    title TEXT NOT NULL,
    description TEXT,
    difficulty TEXT DEFAULT 'medium' CHECK(difficulty IN ('easy', 'medium', 'hard')),
    tags TEXT,
    hints TEXT,
    solution_outline TEXT,
    time_complexity_expected TEXT,
    space_complexity_expected TEXT,
    sample_code TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS learning_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    status TEXT DEFAULT 'not_started' CHECK(status IN ('not_started', 'in_progress', 'completed')),
    mastery_level INTEGER CHECK(mastery_level >= 1 AND mastery_level <= 5),
    notes TEXT,
    last_reviewed_at DATETIME,
    review_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    context_type TEXT CHECK(context_type IN ('topic', 'exercise', 'dashboard')),
    title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    model_provider TEXT,
    token_count INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    model_name TEXT NOT NULL,
    api_key TEXT NOT NULL,
    is_active INTEGER DEFAULT 0,
    config TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics(parent_id);
CREATE INDEX IF NOT EXISTS idx_topics_slug ON topics(slug);
CREATE INDEX IF NOT EXISTS idx_exercises_topic ON exercises(topic_id);
CREATE INDEX IF NOT EXISTS idx_learning_records_topic ON learning_records(topic_id);
CREATE INDEX IF NOT EXISTS idx_learning_records_exercise ON learning_records(exercise_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
`

// Seed data
const seedSQL = `
-- Topics
INSERT OR IGNORE INTO topics (id, parent_id, name, slug, description, key_points, difficulty, sort_order) VALUES
(1, NULL, '数据结构与算法', 'dsa', '软件工程师面试的核心考核内容', '["数组", "链表", "栈和队列", "树", "图", "排序算法", "搜索算法", "动态规划"]', 'beginner', 0),
(2, 1, '基础数据结构', 'data-structures', '最常用的线性数据结构', '["数组", "链表", "栈", "队列"]', 'beginner', 1),
(3, 2, '数组', 'array', '最基本的数据结构，内存连续分布，支持 O(1) 随机访问', '["数组索引", "二维数组", "前缀和", "双指针"]', 'beginner', 10),
(4, 2, '链表', 'linked-list', '节点通过指针连接，适合插入删除', '["单链表", "双链表", "反转链表", "环形链表检测"]', 'beginner', 11),
(5, 2, '栈', 'stack', 'LIFO，后进先出', '["单调栈", "括号匹配", "表达式求值"]', 'beginner', 12),
(6, 2, '队列', 'queue', 'FIFO，先进先出', '["普通队列", "双端队列", "BFS 广度优先"]', 'beginner', 13),
(7, 3, '二分查找', 'binary-search', '有序数组的高效搜索', '["左闭右闭", "左闭右开", "搜索边界"]', 'intermediate', 20),
(8, 1, '树结构', 'trees', '层级数据组织结构', '["二叉树", "BST", "平衡树", "线段树"]', 'intermediate', 2),
(9, 8, '二叉树', 'binary-tree', '每个节点最多两个子节点的树结构', '["前中后序遍历", "层序遍历", "深度计算", "递归与迭代"]', 'intermediate', 30),
(10, 8, 'BST', 'bst', '二叉搜索树，左小右大', '["插入", "删除", "搜索", "验证 BST"]', 'intermediate', 31),
(11, 1, '算法思想', 'algorithms', '解决问题的核心范式', '["排序算法", "搜索算法", "动态规划", "贪心算法"]', 'intermediate', 3),
(12, 11, '排序算法', 'sorting', '将数据按特定顺序排列', '["快速排序", "归并排序", "堆排序", "计数排序"]', 'intermediate', 40),
(13, 11, '动态规划', 'dynamic-programming', '最优子结构 + 状态转移', '["一维 DP", "二维 DP", "背包问题", "LIS"]', 'advanced', 41);

-- Exercises
INSERT OR IGNORE INTO exercises (id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected) VALUES
(1, 3, 'algorithm', '两数之和', '给定一个整数数组 nums 和一个目标值 target，找出数组中和为目标值的两个数的下标。\n\n示例：\n输入: nums = [2, 7, 11, 15], target = 9\n输出: [0, 1]\n解释: nums[0] + nums[1] = 2 + 7 = 9', 'easy', '["数组", "哈希表"]', '["尝试暴力解法 O(n²)", "考虑用哈希表优化到 O(n)", "在遍历时检查 target - nums[i] 是否已在哈希表中"]', '使用哈希表存储已遍历的元素，遍历时检查差值是否存在', 'O(n)', 'O(n)'),
(2, 3, 'algorithm', '三数之和', '给定一个数组 nums，判断是否能从中选出三个数使它们的和为零。\n\n示例：\n输入: nums = [-1, 0, 1, 2, -1, -4]\n输出: [[-1, -1, 2], [-1, 0, 1]]', 'medium', '["数组", "双指针"]', '["排序后处理", "固定一个数，双指针找另外两个", "去重技巧"]', '先排序，固定一个数后用双指针找两数之和为 target-nums[i]', 'O(n²)', 'O(1)'),
(3, 4, 'algorithm', '反转链表', '给定一个单链表，反转链表并返回反转后的链表头节点。', 'easy', '["链表", "递归"]', '["递归版本", "迭代版本（双指针）"]', '遍历时反转 next 指针方向', 'O(n)', 'O(1)'),
(4, 9, 'algorithm', '二叉树的中序遍历', '给定一个二叉树根节点，返回其中序遍历结果。', 'easy', '["二叉树", "遍历", "栈"]', '["递归版本（简洁）", "迭代版本（用栈模拟递归）"]', '左子树 -> 根节点 -> 右子树的顺序遍历', 'O(n)', 'O(h)'),
(5, 10, 'algorithm', '验证 BST', '给定一个二叉树的根节点，验证它是否为有效的二叉搜索树。', 'medium', '["BST", "递归"]', '["BST 的定义是左子树所有节点小于根，右子树所有节点大于根", "需要传递上下界约束"]', '递归时传递当前节点允许的最小值和最大值', 'O(n)', 'O(h)'),
(6, 13, 'algorithm', '爬楼梯', '假设你正在爬楼梯。需要 n 阶你才能到达楼顶。每次你可以爬 1 或 2 个台阶。有多少种不同的方法可以爬到楼顶？', 'easy', '["动态规划", "斐波那契"]', '["找规律：n=1:1, n=2:2, n=3:3, n=4:5...", "第 i 阶的方法数 = f(i-1) + f(i-2)", "这就是斐波那契数列"]', 'dp[i] = dp[i-1] + dp[i-2，可用滚动数组优化', 'O(n)', 'O(1)');
`

func initDB(dbPath string) error {
	// Create parent directory if needed
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	log.Println("Initializing database schema...")
	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}

	log.Println("Seeding data...")
	if _, err := db.Exec(seedSQL); err != nil {
		return err
	}

	return nil
}

func main() {
	// Determine db path
	exeDir, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to get executable path: %v", err)
	}
	backendDir := filepath.Dir(exeDir)
	if _, err := os.Stat(filepath.Join(backendDir, "go.mod")); os.IsNotExist(err) {
		backendDir = "."
	}

	dbPath := filepath.Join(backendDir, "learn-helper.db")

	// Check if db exists, if not initialize
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if err := initDB(dbPath); err != nil {
			log.Fatalf("failed to initialize database: %v", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	h := handler.NewHandler(db)
	aiHandler := handler.NewAIHandler(db)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.HealthCheck)

	r.Route("/api", func(r chi.Router) {
		r.Get("/topics", h.GetTopics)
		r.Get("/topics/{slug}", h.GetTopicBySlug)
		r.Get("/topics/{slug}/exercises", h.GetExercisesByTopic)

		r.Get("/exercises", h.GetExercises)
		r.Get("/exercises/{id}", h.GetExerciseByID)

		r.Get("/learning-records", h.GetLearningRecords)
		r.Post("/learning-records", h.UpsertLearningRecord)

		r.Route("/ai", func(r chi.Router) {
			r.Post("/chat", aiHandler.AIChat)
			r.Get("/conversations", aiHandler.GetConversations)
			r.Get("/conversations/{id}", aiHandler.GetConversation)
			r.Get("/conversations/{id}/messages", aiHandler.GetMessages)
			r.Get("/configs", aiHandler.GetAIConfigs)
			r.Post("/configs", aiHandler.UpsertAIConfig)
		})
	})

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}