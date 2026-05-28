import { useParams } from 'react-router-dom'
import useSWR from 'swr'
import { useState } from 'react'

const fetcher = (url: string) => fetch(url).then((r) => r.json())

interface Exercise {
  id: number
  topic_id: number
  type: string
  title: string
  description: string
  difficulty: string
  tags: string
  hints: string
  status: string
  mastery_level: number
}

export default function PracticePage() {
  const { id } = useParams()
  const [selectedDifficulty, setSelectedDifficulty] = useState('全部')
  const { data } = useSWR<{ exercises: Exercise[] }>(
    id ? null : '/api/exercises',
    fetcher
  )
  const { data: exerciseData } = useSWR(
    id ? `/api/exercises/${id}` : null,
    fetcher
  )

  const exercises = data?.exercises || []
  const exercise = id ? exerciseData : null

  const difficultyColors: Record<string, string> = {
    easy: 'bg-green-100 text-green-700',
    medium: 'bg-yellow-100 text-yellow-700',
    hard: 'bg-red-100 text-red-700',
  }

  const statusLabels: Record<string, string> = {
    not_started: '未开始',
    in_progress: '进行中',
    completed: '已完成',
  }

  if (id && exercise) {
    const e = exercise as unknown as Exercise
    return (
      <div className="p-8">
        <div className="max-w-3xl mx-auto">
          <h1 className="text-2xl font-bold mb-4">{e.title}</h1>
          <div className="flex gap-2 mb-4">
            <span className={`text-sm px-2 py-1 rounded ${difficultyColors[e.difficulty] || ''}`}>
              {e.difficulty}
            </span>
            <span className="text-sm px-2 py-1 rounded bg-blue-100 text-blue-700">{e.type}</span>
            <span className={`text-sm px-2 py-1 rounded ${e.status === 'completed' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
              {statusLabels[e.status] || e.status}
            </span>
          </div>
          <div className="prose max-w-none">
            <p className="text-gray-700 whitespace-pre-wrap">{e.description}</p>
            {e.hints && (
              <div className="mt-6 bg-yellow-50 rounded-lg p-4">
                <h3 className="font-semibold text-gray-800 mb-2">提示</h3>
                <p className="text-gray-600">{e.hints}</p>
              </div>
            )}
          </div>
        </div>
      </div>
    )
  }

  const filtered = selectedDifficulty === '全部'
    ? exercises
    : exercises.filter((e) => e.difficulty === selectedDifficulty)

  return (
    <div className="p-8">
      <div className="max-w-4xl mx-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold">练习题库</h1>
          <select
            value={selectedDifficulty}
            onChange={(e) => setSelectedDifficulty(e.target.value)}
            className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
          >
            <option value="全部">全部难度</option>
            <option value="easy">简单</option>
            <option value="medium">中等</option>
            <option value="hard">困难</option>
          </select>
        </div>
        {filtered.length === 0 ? (
          <p className="text-gray-400">暂无练习题</p>
        ) : (
          <div className="grid gap-4">
            {filtered.map((e) => (
              <div key={e.id} className="bg-white rounded-lg border border-gray-200 p-4 hover:border-blue-300 transition-colors">
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="font-medium text-gray-900">{e.title}</h3>
                    <div className="flex gap-2 mt-2">
                      <span className={`text-xs px-2 py-1 rounded ${difficultyColors[e.difficulty] || ''}`}>
                        {e.difficulty}
                      </span>
                      <span className="text-xs text-gray-500">{e.type}</span>
                    </div>
                  </div>
                  <span className={`text-xs px-2 py-1 rounded ${
                    e.status === 'completed' ? 'bg-green-100 text-green-700' :
                    e.status === 'in_progress' ? 'bg-yellow-100 text-yellow-700' :
                    'bg-gray-100 text-gray-500'
                  }`}>
                    {statusLabels[e.status] || e.status}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}