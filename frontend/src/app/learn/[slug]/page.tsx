import { useParams, Link } from "react-router-dom";
import useSWR from "swr";
import { fetchTopicBySlug } from "../../../lib/api";
import { Breadcrumb } from "../../../components/Breadcrumb";
import { DifficultyBadge } from "../../../components/DifficultyBadge";
import { TopicCard } from "../../../components/TopicCard";
import { MarkdownRenderer } from "../../../components/MarkdownRenderer";

export default function TopicDetailPage() {
  const { slug } = useParams<{ slug: string }>();
  const { data: topic, error, isLoading } = useSWR(
    slug ? `/api/topics/${slug}` : null,
    () => fetchTopicBySlug(slug!)
  );

  if (isLoading) return <div className="p-8 text-center text-gray-400">加载中...</div>;
  if (error || !topic) return <div className="p-8 text-center text-red-400">加载失败</div>;

  const keyPoints: string[] = topic.key_points
    ? (typeof topic.key_points === "string" ? JSON.parse(topic.key_points) : topic.key_points)
    : [];
  const codeExamples: { lang: string; code: string; explanation?: string }[] = topic.code_examples
    ? (typeof topic.code_examples === "string" ? JSON.parse(topic.code_examples) : topic.code_examples)
    : [];
  const commonMistakes: string[] = topic.common_mistakes
    ? (typeof topic.common_mistakes === "string" ? JSON.parse(topic.common_mistakes) : topic.common_mistakes)
    : [];

  return (
    <div className="max-w-4xl mx-auto px-4 py-6">
      <Breadcrumb items={topic.breadcrumb ?? []} />

      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">{topic.name}</h1>
        <DifficultyBadge difficulty={topic.difficulty as any} />
      </div>

      {topic.content && (
        <TopicCard icon="📖" title="概念讲解">
          <MarkdownRenderer content={topic.content} />
        </TopicCard>
      )}

      {keyPoints.length > 0 && (
        <TopicCard icon="💡" title="关键要点">
          <ul className="space-y-1">
            {keyPoints.map((p, i) => (
              <li key={i} className="flex items-start gap-2 text-gray-700">
                <span className="text-yellow-500 mt-0.5">●</span>
                {p}
              </li>
            ))}
          </ul>
        </TopicCard>
      )}

      {codeExamples.length > 0 && (
        <TopicCard icon="💻" title="代码示例">
          <div className="space-y-4">
            {codeExamples.map((ex, i) => (
              <div key={i}>
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-xs font-medium text-gray-500 bg-gray-100 px-2 py-0.5 rounded">{ex.lang}</span>
                  {ex.explanation && <span className="text-sm text-gray-600">{ex.explanation}</span>}
                </div>
                <MarkdownRenderer content={"```" + ex.lang + "\n" + ex.code + "\n```"} />
              </div>
            ))}
          </div>
        </TopicCard>
      )}

      {commonMistakes.length > 0 && (
        <TopicCard icon="⚠️" title="常见错误">
          <ul className="space-y-2">
            {commonMistakes.map((m, i) => (
              <li key={i} className="flex items-start gap-2 text-gray-700">
                <span className="text-red-400 mt-0.5">✕</span>
                {m}
              </li>
            ))}
          </ul>
        </TopicCard>
      )}

      {topic.exercise_count > 0 && (
        <TopicCard icon="🎯" title={`关联练习 (${topic.exercise_count}题)`}>
          <Link
            to={`/practice?topic=${topic.slug}`}
            className="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 transition-colors"
          >
            查看相关练习题 →
          </Link>
        </TopicCard>
      )}

      <div className="flex justify-between items-center mt-8 pt-4 border-t border-gray-200">
        {topic.prev_topic ? (
          <Link to={`/learn/${topic.prev_topic.slug}`} className="flex items-center gap-1 text-gray-600 hover:text-blue-600 transition-colors">
            ← {topic.prev_topic.name}
          </Link>
        ) : <span />}
        {topic.next_topic ? (
          <Link to={`/learn/${topic.next_topic.slug}`} className="flex items-center gap-1 text-gray-600 hover:text-blue-600 transition-colors">
            {topic.next_topic.name} →
          </Link>
        ) : <span />}
      </div>
    </div>
  );
}
