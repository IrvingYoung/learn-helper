import { useParams } from 'react-router-dom'
import useSWR from 'swr'
import { useState } from 'react'

interface Topic {
  id: number
  parent_id: number
  name: string
  slug: string
  description: string
  key_points: string
  difficulty: string
  sort_order: number
}

const fetcher = (url: string) => fetch(url).then((r) => r.json())

function TopicTree({ topics, onSelect, selectedSlug }: { topics: Topic[]; onSelect: (t: Topic) => void; selectedSlug?: string }) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  const roots = topics.filter((t) => !t.parent_id)
  const getChildren = (pid: number) => topics.filter((t) => t.parent_id === pid)

  const toggleExpand = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const renderNode = (topic: Topic, depth: number = 0) => {
    const children = getChildren(topic.id)
    const hasChildren = children.length > 0
    const isExpanded = expanded.has(topic.id)
    const isSelected = selectedSlug === topic.slug

    return (
      <div key={topic.id}>
        <div
          className={`flex items-center gap-2 px-3 py-2 rounded-md cursor-pointer ${
            isSelected ? 'bg-blue-100' : 'hover:bg-gray-100'
          }`}
          style={{ paddingLeft: `${depth * 20 + 12}px` }}
          onClick={() => onSelect(topic)}
        >
          {hasChildren && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                toggleExpand(topic.id)
              }}
              className="text-gray-400 hover:text-gray-600"
            >
              {isExpanded ? '▼' : '▶'}
            </button>
          )}
          {!hasChildren && <span className="w-4" />}
          <span className={`text-sm ${isSelected ? 'text-blue-700 font-medium' : 'text-gray-700'}`}>
            {topic.name}
          </span>
          <span className={`text-xs px-1.5 py-0.5 rounded ${
            topic.difficulty === 'advanced' ? 'bg-red-100 text-red-700' :
            topic.difficulty === 'intermediate' ? 'bg-yellow-100 text-yellow-700' :
            'bg-green-100 text-green-700'
          }`}>
            {topic.difficulty}
          </span>
        </div>
        {isExpanded && children.map((c) => renderNode(c, depth + 1))}
      </div>
    )
  }

  return <div>{roots.map((r) => renderNode(r))}</div>
}

export default function LearnPage() {
  const { slug } = useParams()
  const { data } = useSWR<{ topics: Topic[] }>('/api/topics', fetcher)
  const [selectedTopic, setSelectedTopic] = useState<Topic | null>(null)

  const topics = data?.topics || []

  return (
    <div className="flex h-[calc(100vh-3.5rem)]">
      <div className="w-72 border-r border-gray-200 bg-white overflow-y-auto p-4">
        <h2 className="text-sm font-semibold text-gray-700 mb-4">知识图谱</h2>
        {topics.length === 0 ? (
          <p className="text-sm text-gray-400">暂无知识点</p>
        ) : (
          <TopicTree topics={topics} onSelect={setSelectedTopic} selectedSlug={slug} />
        )}
      </div>

      <div className="flex-1 overflow-y-auto p-8">
        {!selectedTopic && !slug ? (
          <div className="text-center text-gray-400 mt-20">
            <p className="text-lg">选择左侧知识点开始学习</p>
          </div>
        ) : selectedTopic ? (
          <div className="max-w-3xl mx-auto">
            <div className="flex items-center gap-3 mb-6">
              <h1 className="text-2xl font-bold text-gray-900">{selectedTopic.name}</h1>
              <span className={`text-sm px-2 py-1 rounded ${
                selectedTopic.difficulty === 'advanced' ? 'bg-red-100 text-red-700' :
                selectedTopic.difficulty === 'intermediate' ? 'bg-yellow-100 text-yellow-700' :
                'bg-green-100 text-green-700'
              }`}>
                {selectedTopic.difficulty}
              </span>
            </div>
            <div className="prose max-w-none">
              <p className="text-gray-600 mb-6">{selectedTopic.description}</p>
              {selectedTopic.key_points && (
                <div className="bg-gray-50 rounded-lg p-4">
                  <h3 className="font-semibold text-gray-800 mb-2">关键要点</h3>
                  <ul className="list-disc list-inside text-gray-600 space-y-1">
                    {(() => {
                      try {
                        const points = JSON.parse(selectedTopic.key_points)
                        return Array.isArray(points) ? points.map((p: string, i: number) => (
                          <li key={i}>{p}</li>
                        )) : null
                      } catch {
                        return <li>{selectedTopic.key_points}</li>
                      }
                    })()}
                  </ul>
                </div>
              )}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  )
}