import { useState, useEffect } from 'react'

export default function SettingsPage() {
  const [provider, setProvider] = useState('claude')
  const [model, setModel] = useState('claude-sonnet-4-7-20250514')
  const [apiKey, setApiKey] = useState('')
  const [tavilyApiKey, setTavilyApiKey] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    fetch('/api/ai/configs')
      .then(r => r.json())
      .then(data => {
        if (data.configs?.length > 0) {
          const cfg = data.configs[0]
          setProvider(cfg.provider)
          setModel(cfg.model_name)
          setApiKey(cfg.api_key || '')
          if (cfg.tavily_api_key) {
            setTavilyApiKey(cfg.tavily_api_key)
          }
        }
      })
      .catch(() => {})
  }, [])

  const handleSave = async () => {
    const resp = await fetch('/api/ai/configs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        provider,
        model_name: model,
        api_key: apiKey,
        tavily_api_key: tavilyApiKey,
        is_active: true,
      }),
    })
    if (resp.ok) {
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    }
  }

  return (
    <div className="min-h-screen bg-th-bg-primary">
      <div className="p-8 max-w-xl mx-auto">
        <h1 className="text-2xl font-bold text-th-text-primary mb-6">设置</h1>
        <div className="bg-th-bg-secondary rounded-lg border border-th-border shadow-th p-6">
          <h2 className="text-lg font-semibold text-th-text-primary mb-4">AI 模型配置</h2>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Provider</label>
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
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
              >
                <option value="claude">Claude</option>
                <option value="deepseek">DeepSeek</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Model</label>
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
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
              <label className="block text-sm font-medium text-th-text-secondary mb-1">API Key</label>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={provider === 'deepseek' ? 'sk-...' : 'sk-ant-...'}
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Tavily API Key</label>
              <input
                type="password"
                value={tavilyApiKey}
                onChange={(e) => setTavilyApiKey(e.target.value)}
                placeholder="sk-..."
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
              />
              <p className="text-xs text-th-text-muted mt-1">用于 websearch 联网搜索功能</p>
            </div>
            <button
              onClick={handleSave}
              disabled={!apiKey.trim()}
              className="px-4 py-2 bg-th-accent text-white rounded-md hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed text-sm transition-all active:scale-[0.98]"
            >
              保存配置
            </button>
            {saved && <span className="ml-3 text-th-success text-sm animate-pulse">配置已保存</span>}
          </div>
        </div>
      </div>
    </div>
  )
}
