#!/usr/bin/env python3
"""Generate enriched seed data for Learn Helper using Claude API."""

import json
import os
import re
import sys
import time
from pathlib import Path

import anthropic

OUTPUT_DIR = Path(__file__).parent / "output"
TOPICS_DIR = OUTPUT_DIR / "topics"
EXERCISES_DIR = OUTPUT_DIR / "exercises"
SEED_FILE = Path(__file__).parent.parent / "backend" / "db" / "seed" / "seed.sql"

TOPIC_TREE = [
    {"slug": "dsa", "name": "数据结构与算法", "parent_slug": None, "difficulty": "beginner", "sort_order": 1, "key_points": ["数据结构", "算法", "时间复杂度", "空间复杂度"]},
    {"slug": "basic-ds", "name": "基础数据结构", "parent_slug": "dsa", "difficulty": "beginner", "sort_order": 1, "key_points": ["线性结构", "存储方式", "基本操作"]},
    {"slug": "array", "name": "数组", "parent_slug": "basic-ds", "difficulty": "beginner", "sort_order": 1, "key_points": ["连续存储", "随机访问", "插入删除", "双指针"]},
    {"slug": "binary-search", "name": "二分查找", "parent_slug": "array", "difficulty": "intermediate", "sort_order": 2, "key_points": ["有序区间", "中点划分", "边界条件", "变体问题"]},
    {"slug": "linked-list", "name": "链表", "parent_slug": "basic-ds", "difficulty": "beginner", "sort_order": 2, "key_points": ["节点", "指针", "插入删除", "虚拟头节点"]},
    {"slug": "stack", "name": "栈", "parent_slug": "basic-ds", "difficulty": "beginner", "sort_order": 3, "key_points": ["后进先出", "单调栈", "括号匹配", "函数调用栈"]},
    {"slug": "queue", "name": "队列", "parent_slug": "basic-ds", "difficulty": "beginner", "sort_order": 4, "key_points": ["先进先出", "双端队列", "BFS", "优先队列"]},
    {"slug": "hash-table", "name": "哈希表", "parent_slug": "basic-ds", "difficulty": "beginner", "sort_order": 5, "key_points": ["哈希函数", "冲突处理", "负载因子", "O(1)查找"]},
    {"slug": "tree-structure", "name": "树结构", "parent_slug": "dsa", "difficulty": "intermediate", "sort_order": 2, "key_points": ["层次结构", "递归", "遍历"]},
    {"slug": "binary-tree", "name": "二叉树", "parent_slug": "tree-structure", "difficulty": "intermediate", "sort_order": 1, "key_points": ["前中后序遍历", "层序遍历", "深度", "递归"]},
    {"slug": "bst", "name": "BST", "parent_slug": "tree-structure", "difficulty": "intermediate", "sort_order": 2, "key_points": ["有序性", "查找", "插入删除", "中序遍历"]},
    {"slug": "heap", "name": "堆", "parent_slug": "tree-structure", "difficulty": "intermediate", "sort_order": 3, "key_points": ["完全二叉树", "大顶堆", "小顶堆", "优先队列", "堆排序"]},
    {"slug": "trie", "name": "字典树 Trie", "parent_slug": "tree-structure", "difficulty": "intermediate", "sort_order": 4, "key_points": ["前缀匹配", "字符串搜索", "自动补全", "空间换时间"]},
    {"slug": "graph", "name": "图结构", "parent_slug": "dsa", "difficulty": "intermediate", "sort_order": 3, "key_points": ["顶点", "边", "有向图", "无向图"]},
    {"slug": "graph-traversal", "name": "图的表示与遍历", "parent_slug": "graph", "difficulty": "intermediate", "sort_order": 1, "key_points": ["邻接矩阵", "邻接表", "DFS", "BFS"]},
    {"slug": "shortest-path", "name": "最短路径", "parent_slug": "graph", "difficulty": "advanced", "sort_order": 2, "key_points": ["Dijkstra", "Floyd", "Bellman-Ford", "负权边"]},
    {"slug": "topological-sort", "name": "拓扑排序", "parent_slug": "graph", "difficulty": "intermediate", "sort_order": 3, "key_points": ["有向无环图", "入度", "Kahn算法", "DFS后序"]},
    {"slug": "algorithms", "name": "算法思想", "parent_slug": "dsa", "difficulty": "intermediate", "sort_order": 4, "key_points": ["算法设计", "问题分解", "最优子结构"]},
    {"slug": "sorting", "name": "排序算法", "parent_slug": "algorithms", "difficulty": "intermediate", "sort_order": 1, "key_points": ["比较排序", "非比较排序", "稳定性", "O(nlogn)"]},
    {"slug": "dp", "name": "动态规划", "parent_slug": "algorithms", "difficulty": "advanced", "sort_order": 2, "key_points": ["最优子结构", "重叠子问题", "状态转移", "备忘录"]},
    {"slug": "knapsack", "name": "背包问题", "parent_slug": "dp", "difficulty": "advanced", "sort_order": 1, "key_points": ["0-1背包", "完全背包", "状态压缩", "初始化技巧"]},
    {"slug": "greedy", "name": "贪心算法", "parent_slug": "algorithms", "difficulty": "intermediate", "sort_order": 3, "key_points": ["局部最优", "贪心选择性质", "区间问题", "证明策略"]},
    {"slug": "backtracking", "name": "回溯算法", "parent_slug": "algorithms", "difficulty": "intermediate", "sort_order": 4, "key_points": ["递归", "剪枝", "全排列", "N皇后"]},
    {"slug": "divide-conquer", "name": "分治算法", "parent_slug": "algorithms", "difficulty": "intermediate", "sort_order": 5, "key_points": ["分解", "解决", "合并", "主定理"]},
    {"slug": "techniques", "name": "高频技巧", "parent_slug": "dsa", "difficulty": "intermediate", "sort_order": 5, "key_points": ["面试高频", "套路识别", "模板"]},
    {"slug": "sliding-window", "name": "滑动窗口", "parent_slug": "techniques", "difficulty": "intermediate", "sort_order": 1, "key_points": ["双指针", "窗口收缩", "窗口扩展", "最值更新"]},
    {"slug": "two-pointers", "name": "双指针", "parent_slug": "techniques", "difficulty": "beginner", "sort_order": 2, "key_points": ["对撞指针", "快慢指针", "有序数组", "链表操作"]},
    {"slug": "bit-manipulation", "name": "位运算", "parent_slug": "techniques", "difficulty": "beginner", "sort_order": 3, "key_points": ["异或", "与或", "位掩码", "状态压缩"]},
]

EXERCISES = [
    {"title": "两数之和", "topic_slug": "array", "difficulty": "easy", "sort_order": 1},
    {"title": "合并两个有序数组", "topic_slug": "array", "difficulty": "easy", "sort_order": 2},
    {"title": "三数之和", "topic_slug": "array", "difficulty": "medium", "sort_order": 3},
    {"title": "搜索插入位置", "topic_slug": "binary-search", "difficulty": "easy", "sort_order": 1},
    {"title": "在排序数组中查找元素位置", "topic_slug": "binary-search", "difficulty": "medium", "sort_order": 2},
    {"title": "反转链表", "topic_slug": "linked-list", "difficulty": "easy", "sort_order": 1},
    {"title": "合并两个有序链表", "topic_slug": "linked-list", "difficulty": "easy", "sort_order": 2},
    {"title": "链表中的环检测", "topic_slug": "linked-list", "difficulty": "medium", "sort_order": 3},
    {"title": "有效的括号", "topic_slug": "stack", "difficulty": "easy", "sort_order": 1},
    {"title": "每日温度", "topic_slug": "stack", "difficulty": "medium", "sort_order": 2},
    {"title": "用栈实现队列", "topic_slug": "stack", "difficulty": "easy", "sort_order": 3},
    {"title": "最近的请求次数", "topic_slug": "queue", "difficulty": "easy", "sort_order": 1},
    {"title": "滑动窗口最大值", "topic_slug": "queue", "difficulty": "hard", "sort_order": 2},
    {"title": "两数之和(哈希表版)", "topic_slug": "hash-table", "difficulty": "easy", "sort_order": 1},
    {"title": "字母异位词分组", "topic_slug": "hash-table", "difficulty": "medium", "sort_order": 2},
    {"title": "最长连续序列", "topic_slug": "hash-table", "difficulty": "medium", "sort_order": 3},
    {"title": "二叉树的最大深度", "topic_slug": "binary-tree", "difficulty": "easy", "sort_order": 1},
    {"title": "二叉树的层序遍历", "topic_slug": "binary-tree", "difficulty": "medium", "sort_order": 2},
    {"title": "翻转二叉树", "topic_slug": "binary-tree", "difficulty": "easy", "sort_order": 3},
    {"title": "验证BST", "topic_slug": "bst", "difficulty": "medium", "sort_order": 1},
    {"title": "BST中第K小的元素", "topic_slug": "bst", "difficulty": "medium", "sort_order": 2},
    {"title": "Top K频繁元素", "topic_slug": "heap", "difficulty": "medium", "sort_order": 1},
    {"title": "合并K个升序链表", "topic_slug": "heap", "difficulty": "hard", "sort_order": 2},
    {"title": "实现Trie", "topic_slug": "trie", "difficulty": "medium", "sort_order": 1},
    {"title": "单词搜索II", "topic_slug": "trie", "difficulty": "hard", "sort_order": 2},
    {"title": "岛屿数量", "topic_slug": "graph-traversal", "difficulty": "medium", "sort_order": 1},
    {"title": "克隆图", "topic_slug": "graph-traversal", "difficulty": "medium", "sort_order": 2},
    {"title": "网络延迟时间", "topic_slug": "shortest-path", "difficulty": "medium", "sort_order": 1},
    {"title": "课程表", "topic_slug": "topological-sort", "difficulty": "medium", "sort_order": 1},
    {"title": "课程表II", "topic_slug": "topological-sort", "difficulty": "medium", "sort_order": 2},
    {"title": "数组中的第K个最大元素", "topic_slug": "sorting", "difficulty": "medium", "sort_order": 1},
    {"title": "排序链表", "topic_slug": "sorting", "difficulty": "medium", "sort_order": 2},
    {"title": "爬楼梯", "topic_slug": "dp", "difficulty": "easy", "sort_order": 1},
    {"title": "最长递增子序列", "topic_slug": "dp", "difficulty": "medium", "sort_order": 2},
    {"title": "零钱兑换", "topic_slug": "dp", "difficulty": "medium", "sort_order": 3},
    {"title": "0-1背包问题", "topic_slug": "knapsack", "difficulty": "medium", "sort_order": 1},
    {"title": "分割等和子集", "topic_slug": "knapsack", "difficulty": "medium", "sort_order": 2},
    {"title": "跳跃游戏", "topic_slug": "greedy", "difficulty": "medium", "sort_order": 1},
    {"title": "无重叠区间", "topic_slug": "greedy", "difficulty": "medium", "sort_order": 2},
    {"title": "全排列", "topic_slug": "backtracking", "difficulty": "medium", "sort_order": 1},
    {"title": "组合总和", "topic_slug": "backtracking", "difficulty": "medium", "sort_order": 2},
    {"title": "N皇后", "topic_slug": "backtracking", "difficulty": "hard", "sort_order": 3},
    {"title": "合并K个排序链表(分治)", "topic_slug": "divide-conquer", "difficulty": "hard", "sort_order": 1},
    {"title": "最大子数组和", "topic_slug": "divide-conquer", "difficulty": "medium", "sort_order": 2},
    {"title": "无重复字符的最长子串", "topic_slug": "sliding-window", "difficulty": "medium", "sort_order": 1},
    {"title": "最小覆盖子串", "topic_slug": "sliding-window", "difficulty": "hard", "sort_order": 2},
    {"title": "盛最多水的容器", "topic_slug": "two-pointers", "difficulty": "medium", "sort_order": 1},
    {"title": "删除排序数组中的重复项", "topic_slug": "two-pointers", "difficulty": "easy", "sort_order": 2},
    {"title": "只出现一次的数字", "topic_slug": "bit-manipulation", "difficulty": "easy", "sort_order": 1},
    {"title": "汉明距离", "topic_slug": "bit-manipulation", "difficulty": "easy", "sort_order": 2},
]

TOPIC_PROMPT = """你是一位数据结构与算法的教学专家。请为以下知识点生成详细的教学内容。

知识点名称：{name}
难度级别：{difficulty}
上级知识点：{parent_name}
关键要点：{key_points}

请严格按以下 JSON 格式返回，不要添加任何其他文字：
{{
  "content": "500-800字 Markdown 格式的概念讲解，包含：概念定义、核心原理、应用场景、与其他数据结构或算法的对比",
  "code_examples": [{{"lang": "Python", "code": "代码内容", "explanation": "代码说明"}}],
  "common_mistakes": ["错误1的简明描述", "错误2的简明描述"]
}}

要求：
- content 用中文撰写，代码注释可用中文
- 代码示例 2-3 个，优先 Python
- 常见错误 3-5 个，每个一句话描述"""

EXERCISE_PROMPT = """你是一位算法面试题设计专家。请为以下知识点设计一道算法练习题。

关联知识点：{topic_name}
难度级别：{difficulty}
题目标题：{title}

请严格按以下 JSON 格式返回，不要添加任何其他文字：
{{
  "description": "题目描述，含示例输入和输出，Markdown 格式",
  "hints": ["提示1：思路方向", "提示2：具体方法", "提示3：实现要点"],
  "solution_outline": "解题思路概要，2-3句话",
  "solution_detail": "详细解答，Markdown 格式，含代码和复杂度分析",
  "sample_code": [{{"lang": "Python", "code": "代码内容"}}],
  "common_errors": ["常见错误1", "常见错误2"],
  "time_complexity_expected": "O(...)",
  "space_complexity_expected": "O(...)"
}}

要求：
- description 含至少一个示例输入/输出
- hints 从方向到具体，逐步递进
- sample_code 优先 Python
- common_errors 2-3 个"""


def get_client():
    return anthropic.Anthropic(
        base_url="https://maas-coding-api.cn-huabei-1.xf-yun.com/anthropic",
        api_key="536de9613a405fbfbab0ce9bc5d537ed:YTg4MjVjYjVmYzZhNDYxYTUzODBkZGEw",
    )


def slug_to_id_map(topics):
    mapping = {}
    for i, t in enumerate(topics, start=1):
        mapping[t["slug"]] = i
    return mapping


def clean_json_text(text):
    """Strip markdown fences and remove invalid control characters from JSON."""
    if text.startswith("```"):
        lines = text.split("\n")
        lines = [l for l in lines if not l.startswith("```")]
        text = "\n".join(lines)
    # Remove control characters except newline, carriage return, tab
    text = re.sub(r'[\x00-\x08\x0b\x0c\x0e-\x1f]', '', text)
    return text


def generate_topic_content(client, topic, parent_name):
    prompt = TOPIC_PROMPT.format(
        name=topic["name"],
        difficulty=topic["difficulty"],
        parent_name=parent_name,
        key_points=", ".join(topic["key_points"]),
    )
    resp = client.messages.create(
        model="xopkimik26",
        max_tokens=4000,
        messages=[{"role": "user", "content": prompt}],
    )
    text = resp.content[0].text.strip()
    text = clean_json_text(text)
    for attempt in range(3):
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            match = re.search(r'\{[\s\S]*\}', text)
            if match:
                try:
                    return json.loads(match.group())
                except json.JSONDecodeError:
                    pass
            if attempt < 2:
                print(f"    [retry] JSON parse failed, regenerating...")
                time.sleep(1)
                resp = client.messages.create(
                    model="xopkimik26",
                    max_tokens=4000,
                    messages=[{"role": "user", "content": prompt}],
                )
                text = resp.content[0].text.strip()
                text = clean_json_text(text)
            else:
                raise


def generate_exercise_content(client, exercise, topic_name):
    prompt = EXERCISE_PROMPT.format(
        topic_name=topic_name,
        difficulty=exercise["difficulty"],
        title=exercise["title"],
    )
    resp = client.messages.create(
        model="xopkimik26",
        max_tokens=4000,
        messages=[{"role": "user", "content": prompt}],
    )
    text = resp.content[0].text.strip()
    text = clean_json_text(text)
    for attempt in range(3):
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            match = re.search(r'\{[\s\S]*\}', text)
            if match:
                try:
                    return json.loads(match.group())
                except json.JSONDecodeError:
                    pass
            if attempt < 2:
                print(f"    [retry] JSON parse failed, regenerating...")
                time.sleep(1)
                resp = client.messages.create(
                    model="xopkimik26",
                    max_tokens=4000,
                    messages=[{"role": "user", "content": prompt}],
                )
                text = resp.content[0].text.strip()
                text = clean_json_text(text)
            else:
                raise


def generate_all_topics(client, topics):
    TOPICS_DIR.mkdir(parents=True, exist_ok=True)
    results = {}
    for topic in topics:
        out_file = TOPICS_DIR / f"{topic['slug']}.json"
        if out_file.exists():
            print(f"  [skip] {topic['name']} (already generated)")
            results[topic["slug"]] = json.loads(out_file.read_text(encoding="utf-8"))
            continue
        parent_name = next(
            (t["name"] for t in topics if t["slug"] == topic["parent_slug"]),
            "无（顶级节点）",
        )
        print(f"  [gen]  {topic['name']}...")
        content = generate_topic_content(client, topic, parent_name)
        out_file.write_text(json.dumps(content, ensure_ascii=False, indent=2), encoding="utf-8")
        results[topic["slug"]] = content
        time.sleep(1)
    return results


def generate_all_exercises(client, exercises, topics):
    EXERCISES_DIR.mkdir(parents=True, exist_ok=True)
    slug_map = {t["slug"]: t["name"] for t in topics}
    results = {}
    for i, ex in enumerate(exercises, start=1):
        out_file = EXERCISES_DIR / f"ex-{i:03d}.json"
        if out_file.exists():
            print(f"  [skip] {ex['title']} (already generated)")
            results[i] = json.loads(out_file.read_text(encoding="utf-8"))
            continue
        topic_name = slug_map.get(ex["topic_slug"], ex["topic_slug"])
        print(f"  [gen]  {ex['title']}...")
        content = generate_exercise_content(client, ex, topic_name)
        out_file.write_text(json.dumps(content, ensure_ascii=False, indent=2), encoding="utf-8")
        results[i] = content
        time.sleep(1)
    return results


def escape_sql(s):
    return s.replace("'", "''").replace("\\", "\\\\") if s else ""


def build_seed_sql(topics, topic_contents, exercises, exercise_contents):
    slug_id = slug_to_id_map(topics)
    lines = []
    lines.append("-- Learn Helper Seed Data (AI-generated)")
    lines.append("-- Generated by scripts/generate_seeds.py")
    lines.append("")
    lines.append("DELETE FROM learning_records;")
    lines.append("DELETE FROM exercises;")
    lines.append("DELETE FROM topics;")
    lines.append("")

    for topic in topics:
        tc = topic_contents.get(topic["slug"], {})
        parent_id = slug_id.get(topic["parent_slug"])
        parent_id_sql = str(parent_id) if parent_id else "NULL"
        content = escape_sql(tc.get("content", ""))
        key_points = json.dumps(topic["key_points"], ensure_ascii=False)
        key_points = escape_sql(key_points)
        code_examples = json.dumps(tc.get("code_examples", []), ensure_ascii=False)
        code_examples = escape_sql(code_examples)
        common_mistakes = json.dumps(tc.get("common_mistakes", []), ensure_ascii=False)
        common_mistakes = escape_sql(common_mistakes)
        description = escape_sql(tc.get("content", "")[:100])

        lines.append(
            f"INSERT INTO topics (id, slug, name, description, difficulty, parent_id, sort_order, content, key_points, code_examples, common_mistakes) "
            f"VALUES ({slug_id[topic['slug']]}, '{topic['slug']}', '{escape_sql(topic['name'])}', "
            f"'{description}', '{topic['difficulty']}', {parent_id_sql}, {topic['sort_order']}, "
            f"'{content}', '{key_points}', '{code_examples}', '{common_mistakes}');"
        )

    lines.append("")

    slug_map = {t["slug"]: t["name"] for t in topics}
    for i, ex in enumerate(exercises, start=1):
        ec = exercise_contents.get(i, {})
        topic_id = slug_id.get(ex["topic_slug"])
        description = escape_sql(ec.get("description", ""))
        hints = json.dumps(ec.get("hints", []), ensure_ascii=False)
        hints = escape_sql(hints)
        solution_outline = escape_sql(ec.get("solution_outline", ""))
        solution_detail = escape_sql(ec.get("solution_detail", ""))
        sample_code = json.dumps(ec.get("sample_code", []), ensure_ascii=False)
        sample_code = escape_sql(sample_code)
        common_errors = json.dumps(ec.get("common_errors", []), ensure_ascii=False)
        common_errors = escape_sql(common_errors)

        lines.append(
            f"INSERT INTO exercises (id, topic_id, title, description, difficulty, sort_order, hints, solution_outline, solution_detail, sample_code, common_errors) "
            f"VALUES ({i}, {topic_id}, '{escape_sql(ex['title'])}', "
            f"'{description}', '{ex['difficulty']}', {ex['sort_order']}, "
            f"'{hints}', '{solution_outline}', '{solution_detail}', '{sample_code}', '{common_errors}');"
        )

    lines.append("")
    return "\n".join(lines)


def main():
    client = get_client()

    print("=== Generating topic content ===")
    topic_contents = generate_all_topics(client, TOPIC_TREE)

    print("\n=== Generating exercise content ===")
    exercise_contents = generate_all_exercises(client, EXERCISES, TOPIC_TREE)

    print("\n=== Building seed.sql ===")
    sql = build_seed_sql(TOPIC_TREE, topic_contents, EXERCISES, exercise_contents)
    SEED_FILE.parent.mkdir(parents=True, exist_ok=True)
    SEED_FILE.write_text(sql, encoding="utf-8")
    print(f"Written to {SEED_FILE}")
    print(f"  Topics: {len(TOPIC_TREE)}")
    print(f"  Exercises: {len(EXERCISES)}")


if __name__ == "__main__":
    main()
