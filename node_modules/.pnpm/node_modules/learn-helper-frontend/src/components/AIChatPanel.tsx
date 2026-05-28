import { useState, useRef, useEffect } from 'react'

type Role = 'knowledge_explain' | 'problem_solving'

interface Message {
  role: 'user' | 'assistant'
  content: string
}

interface AIChatPanelProps {
  onClose: () => void
}

export default function AIChatPanel({ onClose }: AIChatPanelProps) {
  const [role, setRole] = useState<Role>('knowledge_explain')
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const sendMessage = async () => {
    if (!input.trim() || loading) return

    const userMessage = input.trim()
    setInput('')
    setMessages((prev) => [...prev, { role: 'user', content: userMessage }])
    setLoading(true)

    try {
      const resp = await fetch('/api/ai/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          role,
          message: userMessage,
          context_type: role === 'knowledge_explain' ? 'topic' : 'exercise',
          context: '',
          topic_id: null,
          exercise_id: null,
        }),
      })

      if (!resp.ok) throw new Error('API error')

      const reader = resp.body?.getReader()
      const decoder = new TextDecoder()
      let fullContent = ''

      setMessages((prev) => [...prev, { role: 'assistant', content: '' }])

      while (reader) {
        const { done, value } = await reader.read()
        if (done) break

        const chunk = decoder.decode(value)
        const lines = chunk.split('\n')
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data === '[DONE]') continue
            fullContent += data
            setMessages((prev) => {
              const updated = [...prev]
              updated[updated.length - 1] = { role: 'assistant', content: fullContent }
              return updated
            })
          }
        }
      }
    } catch {
      setMessages((prev) => [
        ...prev,
        { role: 'assistant', content: '抱歉，AI 暂时不可用，请稍后重试。' },
      ])
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed right-0 top-14 bottom-0 w-96 bg-white border-l border-gray-200 flex flex-col z-50">
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
        <select
          value={role}
          onChange={(e) => setRole(e.target.value as Role)}
          className="text-sm border border-gray-300 rounded px-2 py-1"
        >
          <option value="knowledge_explain">知识讲解</option>
          <option value="problem_solving">解题辅导</option>
        </select>
        <button onClick={onClose} className="text-gray-500 hover:text-gray-700">
          ✕
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 0 && (
          <div className="text-center text-gray-500 mt-8 text-sm">
            <p>选择角色后输入问题开始对话</p>
            <p className="mt-2 text-xs">
              {role === 'knowledge_explain'
                ? 'AI 会帮你解释概念、举例说明'
                : 'AI 会引导你思考，不直接给答案'}
            </p>
          </div>
        )}
        {messages.map((msg, i) => (
          <div
            key={i}
            className={`rounded-lg p-3 ${
              msg.role === 'user'
                ? 'bg-blue-100 ml-8'
                : 'bg-gray-100 mr-8'
            }`}
          >
            <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
          </div>
        ))}
        {loading && (
          <div className="bg-gray-100 mr-8 rounded-lg p-3">
            <p className="text-sm text-gray-500">思考中...</p>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      <div className="p-4 border-t border-gray-200">
        <div className="flex gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
            placeholder="输入你的问题..."
            className="flex-1 border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            onClick={sendMessage}
            disabled={!input.trim() || loading}
            className="px-4 py-2 bg-blue-600 text-white rounded-md text-sm hover:bg-blue-700 disabled:opacity-50"
          >
            发送
          </button>
        </div>
      </div>
    </div>
  )
}