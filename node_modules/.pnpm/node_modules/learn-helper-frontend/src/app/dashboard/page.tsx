import useSWR from 'swr'

const fetcher = (url: string) => fetch(url).then((r) => r.json())

export default function DashboardPage() {
  const { data: stats } = useSWR('/api/learning-records', fetcher)
  const records = stats?.records || []

  const completed = records.filter((r: any) => r.status === 'completed').length
  const inProgress = records.filter((r: any) => r.status === 'in_progress').length
  const avgMastery = records.length > 0
    ? Math.round(records.reduce((sum: number, r: any) => sum + (r.mastery_level || 0), 0) / records.length * 10) / 10
    : 0

  return (
    <div className="p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-bold mb-6">学习仪表盘</h1>
        <div className="grid grid-cols-3 gap-4 mb-8">
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <p className="text-gray-500 text-sm">已完成</p>
            <p className="text-3xl font-bold text-green-600">{completed}</p>
          </div>
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <p className="text-gray-500 text-sm">进行中</p>
            <p className="text-3xl font-bold text-yellow-600">{inProgress}</p>
          </div>
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <p className="text-gray-500 text-sm">平均掌握程度</p>
            <p className="text-3xl font-bold text-blue-600">{avgMastery || '--'}</p>
          </div>
        </div>
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <h2 className="text-lg font-semibold mb-4">学习记录</h2>
          {records.length === 0 ? (
            <p className="text-gray-400">暂无学习记录，开始学习吧！</p>
          ) : (
            <ul className="space-y-2">
              {records.map((r: any) => (
                <li key={r.id} className="flex justify-between text-sm items-center py-2 border-b border-gray-100 last:border-0">
                  <span className="text-gray-700">Topic #{r.topic_id} / Exercise #{r.exercise_id}</span>
                  <span className={`px-2 py-0.5 rounded text-xs ${
                    r.status === 'completed' ? 'bg-green-100 text-green-700' :
                    r.status === 'in_progress' ? 'bg-yellow-100 text-yellow-700' :
                    'bg-gray-100 text-gray-500'
                  }`}>
                    {r.status === 'not_started' ? '未开始' : r.status === 'in_progress' ? '进行中' : '已完成'}
                    {r.mastery_level > 0 && ` (Lv.${r.mastery_level})`}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}