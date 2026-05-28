import { useParams, useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { useState } from 'react'
import { DifficultyBadge } from '../../components/DifficultyBadge'
import { StatusIcon } from '../../components/StatusIcon'
import { MarkdownRenderer } from '../../components/MarkdownRenderer'
import type { Exercise } from '../../types'

const fetcher = (url: string) => fetch(url).then((r) => r.json())

const DIFFICULTY_OPTIONS = [
  { label: '简单', value: 'easy' },
  { label: '中等', value: 'medium' },
  { label: '困难', value: 'hard' },
]

function getExerciseStatus(status?: string): "not_started" | "in_progress" | "mastered" {
  if (status === 'completed') return 'mastered'
  if (status === 'in_progress') return 'in_progress'
  return 'not_started'
}

export default function PracticePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [selectedDifficulty, setSelectedDifficulty] = useState<string | null>(null)

  const { data: listData } = useSWR<{ exercises: Exercise[] }>(
    !id ? '/api/exercises' : null,
    fetcher
  )
  const { data: detailData } = useSWR<Exercise>(
    id ? `/api/exercises/${id}` : null,
    fetcher
  )

  const exercises = listData?.exercises || []
  const exercise = detailData ?? null

  // Detail view
  if (id && exercise) {
    const hints: string[] = exercise.hints
      ? (typeof exercise.hints === 'string' ? JSON.parse(exercise.hints) : exercise.hints)
      : []
    const commonErrors: string[] = exercise.common_errors
      ? (typeof exercise.common_errors === 'string' ? JSON.parse(exercise.common_errors) : exercise.common_errors)
      : []

    return (
      <div className="p-8">
        <div className="max-w-3xl mx-auto">
          <button onClick={() => navigate('/practice')} className="text-sm text-gray-500 hover:text-gray-700 mb-4">← 返回练习列表</button>

          <div className="flex items-center gap-3 mb-6">
            <h1 className="text-2xl font-bold text-gray-900">{exercise.title}</h1>
            <DifficultyBadge difficulty={exercise.difficulty as any} />
            <span className="text-sm px-2 py-0.5 rounded bg-blue-100 text-blue-700 border border-blue-200">{exercise.type}</span>
          </div>

          <div className="bg-white border border-gray-200 rounded-xl p-5 mb-4 shadow-sm">
            <h3 className="flex items-center gap-2 text-base font-semibold text-gray-800 mb-3">
              <span>📝</span>题目描述
            </h3>
            <MarkdownRenderer content={exercise.description} />
          </div>

          {hints.length > 0 && (
            <div className="bg-yellow-50 border border-yellow-200 rounded-xl p-5 mb-4 shadow-sm">
              <h3 className="flex items-center gap-2 text-base font-semibold text-gray-800 mb-3">
                <span>💡</span>提示
              </h3>
              <ul className="space-y-2">
                {hints.map((h, i) => (
                  <li key={i} className="flex items-start gap-2 text-gray-700">
                    <span className="text-yellow-500 font-bold">{i + 1}</span>
                    {h}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {exercise.solution_outline && (
            <div className="bg-gray-50 border border-gray-200 rounded-xl p-5 mb-4 shadow-sm">
              <h3 className="flex items-center gap-2 text-base font-semibold text-gray-800 mb-3">
                <span>🧠</span>解题思路
              </h3>
              <p className="text-gray-600">{exercise.solution_outline}</p>
            </div>
          )}

          {exercise.solution_detail && (
            <div className="bg-blue-50 border border-blue-200 rounded-xl p-5 mb-4 shadow-sm">
              <h3 className="flex items-center gap-2 text-base font-semibold text-gray-800 mb-3">
                <span>📖</span>详细解答
              </h3>
              <MarkdownRenderer content={exercise.solution_detail} />
            </div>
          )}

          {commonErrors.length > 0 && (
            <div className="bg-red-50 border border-red-200 rounded-xl p-5 mb-4 shadow-sm">
              <h3 className="flex items-center gap-2 text-base font-semibold text-gray-800 mb-3">
                <span>⚠️</span>常见错误
              </h3>
              <ul className="space-y-2">
                {commonErrors.map((e, i) => (
                  <li key={i} className="flex items-start gap-2 text-red-700">
                    <span className="text-red-400">✕</span>
                    {e}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    )
  }

  // List view
  const filtered = selectedDifficulty
    ? exercises.filter((e) => e.difficulty === selectedDifficulty)
    : exercises

  // Parse tags for display
  const getTags = (exercise: Exercise): string[] => {
    try {
      const tags = typeof exercise.tags === 'string' ? JSON.parse(exercise.tags) : exercise.tags
      return Array.isArray(tags) ? tags : []
    } catch { return [] }
  }

  return (
    <div className="p-8">
      <div className="max-w-4xl mx-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold text-gray-900">练习题库</h1>
          <div className="flex gap-2">
            {DIFFICULTY_OPTIONS.map(opt => (
              <button
                key={opt.value}
                onClick={() => setSelectedDifficulty(selectedDifficulty === opt.value ? null : opt.value)}
                className={`px-3 py-1 rounded-full text-sm border transition-colors ${
                  selectedDifficulty === opt.value
                    ? 'bg-blue-100 text-blue-700 border-blue-300'
                    : 'bg-gray-50 text-gray-600 border-gray-200 hover:bg-gray-100'
                }`}
              >
                {opt.label}
              </button>
            ))}
            {selectedDifficulty && (
              <button onClick={() => setSelectedDifficulty(null)} className="text-xs text-gray-400 hover:text-gray-600 ml-2">
                清除筛选
              </button>
            )}
          </div>
        </div>

        {filtered.length === 0 ? (
          <div className="text-center text-gray-400 py-16">暂无练习题</div>
        ) : (
          <div className="grid gap-4">
            {filtered.map((e) => (
              <div
                key={e.id}
                className="border border-gray-200 rounded-xl p-4 hover:shadow-md transition-shadow cursor-pointer"
                onClick={() => navigate(`/practice/${e.id}`)}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <StatusIcon status={getExerciseStatus(e.status)} size="md" />
                    <h3 className="font-medium text-gray-800">{e.title}</h3>
                  </div>
                  <DifficultyBadge difficulty={e.difficulty as any} />
                </div>
                <p className="text-sm text-gray-600 line-clamp-2 mb-2">{e.description}</p>
                <div className="flex gap-1 flex-wrap">
                  {getTags(e).map((tag: string) => (
                    <span key={tag} className="text-xs bg-blue-50 text-blue-600 px-2 py-0.5 rounded">{tag}</span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
