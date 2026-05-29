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
	_ "modernc.org/sqlite"
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
CREATE TABLE IF NOT EXISTS wiki_pages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    title           TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    page_type       TEXT NOT NULL DEFAULT 'entity',
    content         TEXT NOT NULL DEFAULT '',
    tags            TEXT DEFAULT '[]',
    parent_id       INTEGER REFERENCES wiki_pages(id),
    content_status  TEXT NOT NULL DEFAULT 'empty',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_slug ON wiki_pages(slug);
`

// Seed data
const seedSQL = `
-- Topics
INSERT OR IGNORE INTO topics (id, parent_id, name, slug, description, key_points, difficulty, sort_order) VALUES
(1, 0, '鏁版嵁缁撴瀯涓庣畻娉?, 'dsa', '杞欢宸ョ▼甯堥潰璇曠殑鏍稿績鑰冩牳鍐呭', '["鏁扮粍", "閾捐〃", "鏍堝拰闃熷垪", "鏍?, "鍥?, "鎺掑簭绠楁硶", "鎼滅储绠楁硶", "鍔ㄦ€佽鍒?]', 'beginner', 0),
(2, 1, '鍩虹鏁版嵁缁撴瀯', 'data-structures', '鏈€甯哥敤鐨勭嚎鎬ф暟鎹粨鏋?, '["鏁扮粍", "閾捐〃", "鏍?, "闃熷垪"]', 'beginner', 1),
(3, 2, '鏁扮粍', 'array', '鏈€鍩烘湰鐨勬暟鎹粨鏋勶紝鍐呭瓨杩炵画鍒嗗竷锛屾敮鎸?O(1) 闅忔満璁块棶', '["鏁扮粍绱㈠紩", "浜岀淮鏁扮粍", "鍓嶇紑鍜?, "鍙屾寚閽?]', 'beginner', 10),
(4, 2, '閾捐〃', 'linked-list', '鑺傜偣閫氳繃鎸囬拡杩炴帴锛岄€傚悎鎻掑叆鍒犻櫎', '["鍗曢摼琛?, "鍙岄摼琛?, "鍙嶈浆閾捐〃", "鐜舰閾捐〃妫€娴?]', 'beginner', 11),
(5, 2, '鏍?, 'stack', 'LIFO锛屽悗杩涘厛鍑?, '["鍗曡皟鏍?, "鎷彿鍖归厤", "琛ㄨ揪寮忔眰鍊?]', 'beginner', 12),
(6, 2, '闃熷垪', 'queue', 'FIFO锛屽厛杩涘厛鍑?, '["鏅€氶槦鍒?, "鍙岀闃熷垪", "BFS 骞垮害浼樺厛"]', 'beginner', 13),
(7, 3, '浜屽垎鏌ユ壘', 'binary-search', '鏈夊簭鏁扮粍鐨勯珮鏁堟悳绱?, '["宸﹂棴鍙抽棴", "宸﹂棴鍙冲紑", "鎼滅储杈圭晫"]', 'intermediate', 20),
(8, 1, '鏍戠粨鏋?, 'trees', '灞傜骇鏁版嵁缁勭粐缁撴瀯', '["浜屽弶鏍?, "BST", "骞宠　鏍?, "绾挎鏍?]', 'intermediate', 2),
(9, 8, '浜屽弶鏍?, 'binary-tree', '姣忎釜鑺傜偣鏈€澶氫袱涓瓙鑺傜偣鐨勬爲缁撴瀯', '["鍓嶄腑鍚庡簭閬嶅巻", "灞傚簭閬嶅巻", "娣卞害璁＄畻", "閫掑綊涓庤凯浠?]', 'intermediate', 30),
(10, 8, 'BST', 'bst', '浜屽弶鎼滅储鏍戯紝宸﹀皬鍙冲ぇ', '["鎻掑叆", "鍒犻櫎", "鎼滅储", "楠岃瘉 BST"]', 'intermediate', 31),
(11, 1, '绠楁硶鎬濇兂', 'algorithms', '瑙ｅ喅闂鐨勬牳蹇冭寖寮?, '["鎺掑簭绠楁硶", "鎼滅储绠楁硶", "鍔ㄦ€佽鍒?, "璐績绠楁硶"]', 'intermediate', 3),
(12, 11, '鎺掑簭绠楁硶', 'sorting', '灏嗘暟鎹寜鐗瑰畾椤哄簭鎺掑垪', '["蹇€熸帓搴?, "褰掑苟鎺掑簭", "鍫嗘帓搴?, "璁℃暟鎺掑簭"]', 'intermediate', 40),
(13, 11, '鍔ㄦ€佽鍒?, 'dynamic-programming', '鏈€浼樺瓙缁撴瀯 + 鐘舵€佽浆绉?, '["涓€缁?DP", "浜岀淮 DP", "鑳屽寘闂", "LIS"]', 'advanced', 41);

-- Exercises
INSERT OR IGNORE INTO exercises (id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected) VALUES
(1, 3, 'algorithm', '涓ゆ暟涔嬪拰', '缁欏畾涓€涓暣鏁版暟缁?nums 鍜屼竴涓洰鏍囧€?target锛屾壘鍑烘暟缁勪腑鍜屼负鐩爣鍊肩殑涓や釜鏁扮殑涓嬫爣銆俓n\n绀轰緥锛歕n杈撳叆: nums = [2, 7, 11, 15], target = 9\n杈撳嚭: [0, 1]\n瑙ｉ噴: nums[0] + nums[1] = 2 + 7 = 9', 'easy', '["鏁扮粍", "鍝堝笇琛?]', '["灏濊瘯鏆村姏瑙ｆ硶 O(n虏)", "鑰冭檻鐢ㄥ搱甯岃〃浼樺寲鍒?O(n)", "鍦ㄩ亶鍘嗘椂妫€鏌?target - nums[i] 鏄惁宸插湪鍝堝笇琛ㄤ腑"]', '浣跨敤鍝堝笇琛ㄥ瓨鍌ㄥ凡閬嶅巻鐨勫厓绱狅紝閬嶅巻鏃舵鏌ュ樊鍊兼槸鍚﹀瓨鍦?, 'O(n)', 'O(n)'),
(2, 3, 'algorithm', '涓夋暟涔嬪拰', '缁欏畾涓€涓暟缁?nums锛屽垽鏂槸鍚﹁兘浠庝腑閫夊嚭涓変釜鏁颁娇瀹冧滑鐨勫拰涓洪浂銆俓n\n绀轰緥锛歕n杈撳叆: nums = [-1, 0, 1, 2, -1, -4]\n杈撳嚭: [[-1, -1, 2], [-1, 0, 1]]', 'medium', '["鏁扮粍", "鍙屾寚閽?]', '["鎺掑簭鍚庡鐞?, "鍥哄畾涓€涓暟锛屽弻鎸囬拡鎵惧彟澶栦袱涓?, "鍘婚噸鎶€宸?]', '鍏堟帓搴忥紝鍥哄畾涓€涓暟鍚庣敤鍙屾寚閽堟壘涓ゆ暟涔嬪拰涓?target-nums[i]', 'O(n虏)', 'O(1)'),
(3, 4, 'algorithm', '鍙嶈浆閾捐〃', '缁欏畾涓€涓崟閾捐〃锛屽弽杞摼琛ㄥ苟杩斿洖鍙嶈浆鍚庣殑閾捐〃澶磋妭鐐广€?, 'easy', '["閾捐〃", "閫掑綊"]', '["閫掑綊鐗堟湰", "杩唬鐗堟湰锛堝弻鎸囬拡锛?]', '閬嶅巻鏃跺弽杞?next 鎸囬拡鏂瑰悜', 'O(n)', 'O(1)'),
(4, 9, 'algorithm', '浜屽弶鏍戠殑涓簭閬嶅巻', '缁欏畾涓€涓簩鍙夋爲鏍硅妭鐐癸紝杩斿洖鍏朵腑搴忛亶鍘嗙粨鏋溿€?, 'easy', '["浜屽弶鏍?, "閬嶅巻", "鏍?]', '["閫掑綊鐗堟湰锛堢畝娲侊級", "杩唬鐗堟湰锛堢敤鏍堟ā鎷熼€掑綊锛?]', '宸﹀瓙鏍?-> 鏍硅妭鐐?-> 鍙冲瓙鏍戠殑椤哄簭閬嶅巻', 'O(n)', 'O(h)'),
(5, 10, 'algorithm', '楠岃瘉 BST', '缁欏畾涓€涓簩鍙夋爲鐨勬牴鑺傜偣锛岄獙璇佸畠鏄惁涓烘湁鏁堢殑浜屽弶鎼滅储鏍戙€?, 'medium', '["BST", "閫掑綊"]', '["BST 鐨勫畾涔夋槸宸﹀瓙鏍戞墍鏈夎妭鐐瑰皬浜庢牴锛屽彸瀛愭爲鎵€鏈夎妭鐐瑰ぇ浜庢牴", "闇€瑕佷紶閫掍笂涓嬬晫绾︽潫"]', '閫掑綊鏃朵紶閫掑綋鍓嶈妭鐐瑰厑璁哥殑鏈€灏忓€煎拰鏈€澶у€?, 'O(n)', 'O(h)'),
(6, 13, 'algorithm', '鐖ゼ姊?, '鍋囪浣犳鍦ㄧ埇妤兼銆傞渶瑕?n 闃朵綘鎵嶈兘鍒拌揪妤奸《銆傛瘡娆′綘鍙互鐖?1 鎴?2 涓彴闃躲€傛湁澶氬皯绉嶄笉鍚岀殑鏂规硶鍙互鐖埌妤奸《锛?, 'easy', '["鍔ㄦ€佽鍒?, "鏂愭尝閭ｅ"]', '["鎵捐寰嬶細n=1:1, n=2:2, n=3:3, n=4:5...", "绗?i 闃剁殑鏂规硶鏁?= f(i-1) + f(i-2)", "杩欏氨鏄枑娉㈤偅濂戞暟鍒?]', 'dp[i] = dp[i-1] + dp[i-2锛屽彲鐢ㄦ粴鍔ㄦ暟缁勪紭鍖?, 'O(n)', 'O(1)');
`

func initDB(dbPath string) error {
	// Create parent directory if needed
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", dbPath)
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
	// Use current working directory as backend root
	exeDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %v", err)
	}
	backendDir := exeDir
	if _, err := os.Stat(filepath.Join(backendDir, "go.mod")); os.IsNotExist(err) {
		// Not in backend dir; try looking for it
		log.Fatalf("must be run from or with backend directory as cwd: %v", err)
	}

	dbPath := filepath.Join(backendDir, "learn-helper.db")

	// Check if db exists, if not initialize
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if err := initDB(dbPath); err != nil {
			log.Fatalf("failed to initialize database: %v", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
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
		r.Put("/topics/{slug}/content", h.UpdateTopicContent)
		r.Post("/topics/batch-content", h.BatchUpdateTopicContent)
		r.Get("/topics/{slug}/exercises", h.GetExercisesByTopic)

		r.Get("/exercises", h.GetExercises)
		r.Get("/exercises/{id}", h.GetExerciseByID)
		r.Put("/exercises/{id}/solution", h.UpdateExerciseSolution)
		r.Put("/exercises/{id}/errors", h.UpdateExerciseErrors)

		r.Get("/learning-records", h.GetLearningRecords)
		r.Post("/learning-records", h.UpsertLearningRecord)

		r.Route("/ai", func(r chi.Router) {
			r.Post("/chat", aiHandler.AIChat)
			r.Get("/conversations", aiHandler.ListConversations)
			r.Get("/conversations/{id}", aiHandler.UpdateConversationTitle)
			r.Get("/conversations/{id}/messages", aiHandler.GetConversationMessages)
			r.Get("/configs", aiHandler.GetAIConfigs)
			r.Post("/configs", aiHandler.UpsertAIConfig)
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
