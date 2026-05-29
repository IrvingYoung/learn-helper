import { useState } from 'react'

export default function SettingsPage() {
  const [provider, setProvider] = useState('claude')
  const [model, setModel] = useState('claude-sonnet-4-7-20250514')
  const [apiKey, setApiKey] = useState('')
  const [saved, setSaved] = useState(false)

  const handleSave = async () => {
    const resp = await fetch('/api/ai/configs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        provider,
        model_name: model,
        api_key: apiKey,
        is_active: true,
      }),
    })
    if (resp.ok) {
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    }
  }

  return (
    <div className="p-8">
      <div className="max-w-xl mx-auto">
        <h1 className="text-2xl font-bold mb-6">设置</h1>
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <h2 className="text-lg font-semibold mb-4">AI 模型配置</h2>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Provider</label>
              <select
                value={provider}
                onChange={(e) => {
                  setProvider(e.target.value)
                  if (e.target.value === 'deepseek') {
                    setModel('deepseek-v4-flash')
                  } else {
                    setModel('claude-sonnet-4-7-20250514')
                  }
                }}
                className="w-full border border-gray-300 rounded-md px-3 py-2"
              >
                <option value="claude">Claude</option>
                <option value="deepseek">DeepSeek</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Model</label>
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="w-full border border-gray-300 rounded-md px-3 py-2"
              >
                {provider === 'claude' ? (
                  <>
                    <option value="claude-sonnet-4-7-20250514">Claude Sonnet 4.7</option>
                    <option value="claude-opus-4-7-20250514">Claude Opus 4.7</option>
                    <option value="claude-haiku-4-5-20250501">Claude Haiku 4.5</option>
                  </>
                ) : (
                  <>
                    <option value="deepseek-v4-flash">DeepSeek V4 Flash</option>
                    <option value="deepseek-chat">DeepSeek Chat</option>
                  </>
                )}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={provider === 'deepseek' ? 'sk-...' : 'sk-ant-...'}
                className="w-full border border-gray-300 rounded-md px-3 py-2"
              />
            </div>
            <button
              onClick={handleSave}
              className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
            >
              保存配置
            </button>
            {saved && <span className="ml-3 text-green-600 text-sm">配置已保存</span>}
          </div>
        </div>
      </div>
    </div>
  )
}