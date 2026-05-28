import useSWR from 'swr'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import type { Topic } from '../../types'
import { StatusIcon } from '../../components/StatusIcon'
import { ProgressBar } from '../../components/ProgressBar'
import { EmptyState } from '../../components/EmptyState'

interface LearningRecord {
  topic_id: number
  status: string
  mastery_level: number
}

const fetcher = (url: string) => fetch(url).then((r) => r.json())

function TopicTree({ topics, onSelect, selectedSlug, statusMap, exerciseCounts }: {
  topics: Topic[]
  onSelect: (t: Topic) => void
  selectedSlug?: string
  statusMap: Map<number, string>
  exerciseCounts: Map<number, number>
}) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  const roots = topics.filter((t) => !t.parent_id || t.parent_id === 0)
  const getChildren = (pid: number) => topics.filter((t) => t.parent_id === pid)

  const toggleExpand = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const getStatus = (id: number): "not_started" | "in_progress" | "mastered" => {
    const s = statusMap.get(id)
    if (s === 'completed') return 'mastered'
    if (s === 'in_progress') return 'in_progress'
    return 'not_started'
  }

  const isAncestor = (topic: Topic): boolean => {
    if (!selectedSlug) return false
    const selected = topics.find(t => t.slug === selectedSlug)
    if (!selected) return false
    let current: Topic | null | undefined = selected
    while (current) {
      if (current.id === topic.id) return true
      const parent = topics.find(t => t.id === current.parent_id)
      current = parent || null
    }
    return false
  }

  const renderNode = (topic: Topic, depth: number = 0) => {
    const children = getChildren(topic.id)
    const hasChildren = children.length > 0
    const isExpanded = expanded.has(topic.id) || isAncestor(topic)
    const isSelected = selectedSlug === topic.slug
    const count = exerciseCounts.get(topic.id) || 0

    return (
      <div key={topic.id}>
        <div
          className={`flex items-center gap-2 px-3 py-2 rounded-md cursor-pointer ${
            isSelected ? 'bg-blue-100 text-blue-700' : isAncestor(topic) ? 'bg-blue-50' : 'hover:bg-gray-100'
          }`}
          style={{ paddingLeft: `${depth * 20 + 12}px` }}
          onClick={() => onSelect(topic)}
        >
          {hasChildren && (
            <button
              onClick={(e) => { e.stopPropagation(); toggleExpand(topic.id) }}
              className="text-gray-400 hover:text-gray-600 text-xs"
            >
              {isExpanded ? '▼' : '▶'}
            </button>
          )}
          {!hasChildren && <span className="w-4" />}
          <StatusIcon status={getStatus(topic.id)} />
          <span className={`flex-1 text-sm ${isSelected ? 'font-medium' : 'text-gray-700'}`}>
            {topic.name}
          </span>
          {count > 0 && <span className="text-xs text-gray-400">({count}题)</span>}
        </div>
        {isExpanded && children.map((c) => renderNode(c, depth + 1))}
      </div>
    )
  }

  return <div>{roots.map((r) => renderNode(r))}</div>
}

export default function LearnPage() {
  const navigate = useNavigate()
  const { data: topicsData } = useSWR<{ topics: Topic[] }>('/api/topics', fetcher)
  const { data: recordsData } = useSWR<{ records: LearningRecord[] }>('/api/learning-records', fetcher)

  const topics = topicsData?.topics || []
  const records = recordsData?.records || []

  // Build status map from learning records
  const statusMap = new Map<number, string>()
  for (const r of records) {
    statusMap.set(r.topic_id, r.status)
  }

  // Build exercise count map from topic data
  const exerciseCounts = new Map<number, number>()
  for (const t of topics) {
    if (t.exercise_count) exerciseCounts.set(t.id, t.exercise_count)
  }

  // Count mastered leaf topics
  const leafTopics = topics.filter(t => !topics.some(c => c.parent_id === t.id))
  const masteredCount = leafTopics.filter(t => {
    const s = statusMap.get(t.id)
    return s === 'completed'
  }).length

  return (
    <div className="flex h-[calc(100vh-3.5rem)]">
      <div className="w-72 border-r border-gray-200 bg-white overflow-y-auto p-4">
        <h2 className="text-sm font-semibold text-gray-700 mb-2">知识图谱</h2>
        <ProgressBar completed={masteredCount} total={leafTopics.length} label="学习进度" />
        <div className="mt-4">
          {topics.length === 0 ? (
            <p className="text-sm text-gray-400">暂无知识点</p>
          ) : (
            <TopicTree
              topics={topics}
              onSelect={(t) => navigate(`/learn/${t.slug}`)}
              statusMap={statusMap}
              exerciseCounts={exerciseCounts}
            />
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-8">
        <EmptyState
          title="开始你的学习之旅"
          description="建议从「数组」开始，逐步掌握数据结构与算法的核心知识"
          actionLabel="开始学习"
          actionTo="/learn/array"
        />
      </div>
    </div>
  )
}
