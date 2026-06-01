import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTheme } from '../../contexts/ThemeContext'

export default function SettingsPage() {
  const navigate = useNavigate()
  const { theme, toggleTheme } = useTheme()
  const [provider, setProvider] = useState('opencode')
  const [model, setModel] = useState('deepseek-v4-pro')
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
    <div className="min-h-screen bg-th-bg-primary flex flex-col">
      <header className="bg-th-bg-secondary/70 backdrop-blur-md border-b border-th-separator h-14 flex items-center px-4 shrink-0">
        <button
          onClick={() => navigate('/wiki')}
          className="inline-flex items-center gap-2 text-th-text-muted hover:text-th-text-primary transition-colors group"
        >
          <svg className="w-4 h-4 group-hover:-translate-x-0.5 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M15 19l-7-7 7-7" />
          </svg>
          <span className="text-sm font-semibold text-th-text-primary">LLM Wiki</span>
        </button>
        <div className="flex-1" />
        <button
          onClick={toggleTheme}
          className="p-2 rounded-md text-th-text-muted hover:text-th-text-primary hover:bg-th-hover transition-all duration-150 active:scale-90"
          title={theme === 'warm' ? '切换深色主题' : '切换暖色主题'}
        >
          {theme === 'warm' ? (
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
            </svg>
          ) : (
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
          )}
        </button>
      </header>

      <main className="flex-1 overflow-y-auto">
        <div className="max-w-2xl mx-auto px-6 py-12">
          <div className="mb-8">
            <p className="text-[11px] font-semibold text-th-text-muted tracking-[0.18em] uppercase mb-2">
              settings
            </p>
            <h1 className="font-display text-3xl font-bold text-th-text-primary tracking-tight">
              配置
            </h1>
            <p className="text-sm text-th-text-secondary mt-2">
              管理你的 AI 模型和 API 凭据。所有密钥仅存储在本地。
            </p>
          </div>

          <section className="bg-th-bg-secondary border border-th-border rounded-lg shadow-th p-6 space-y-5">
            <h2 className="font-display text-lg font-semibold text-th-text-primary">
              AI 模型
            </h2>

            <div>
              <label className="block text-[11px] font-semibold text-th-text-muted tracking-[0.14em] uppercase mb-1.5">
                Provider
              </label>
              <select
                value={provider}
                onChange={(e) => {
                  setProvider(e.target.value)
                  if (e.target.value === 'opencode') {
                    setModel('deepseek-v4-pro')
                  } else {
                    setModel('deepseek-v4-flash')
                  }
                }}
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent/30 focus:border-th-accent transition-colors"
              >
                <option value="opencode">OpenCode Go</option>
                <option value="deepseek">DeepSeek</option>
              </select>
            </div>

            <div>
              <label className="block text-[11px] font-semibold text-th-text-muted tracking-[0.14em] uppercase mb-1.5">
                Model
              </label>
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent/30 focus:border-th-accent transition-colors"
              >
                {provider === 'opencode' ? (
                  <>
                    <option value="deepseek-v4-pro">DeepSeek V4 Pro</option>
                    <option value="deepseek-v4-flash">DeepSeek V4 Flash</option>
                    <option value="qwen3.7-max">Qwen 3.7 Max</option>
                    <option value="kimi-k2.6">Kimi K2.6</option>
                    <option value="mimo-v2.5-pro">MiMo V2.5 Pro</option>
                    <option value="minimax-m3">MiniMax M3</option>
                    <option value="glm-5.1">GLM 5.1</option>
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
              <label className="block text-[11px] font-semibold text-th-text-muted tracking-[0.14em] uppercase mb-1.5">
                API Key
              </label>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="sk-..."
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-th-accent/30 focus:border-th-accent transition-colors"
              />
            </div>

            <div>
              <label className="block text-[11px] font-semibold text-th-text-muted tracking-[0.14em] uppercase mb-1.5">
                Tavily API Key <span className="text-th-text-muted normal-case font-normal tracking-normal">（可选，用于联网搜索）</span>
              </label>
              <input
                type="password"
                value={tavilyApiKey}
                onChange={(e) => setTavilyApiKey(e.target.value)}
                placeholder="tvly-..."
                className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-th-accent/30 focus:border-th-accent transition-colors"
              />
            </div>

            <div className="pt-3 flex items-center gap-3">
              <button
                onClick={handleSave}
                disabled={!apiKey.trim()}
                className="px-4 py-2 bg-th-accent text-white rounded-md text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed active:scale-[0.98] transition-all duration-150 shadow-th"
              >
                保存配置
              </button>
              {saved && (
                <span className="text-sm text-th-success flex items-center gap-1.5 animate-fade-in">
                  <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                  </svg>
                  已保存
                </span>
              )}
            </div>
          </section>

          <p className="text-xs text-th-text-muted mt-6 text-center">
            配置更改立即生效。密钥仅存储在本地数据库中。
          </p>
        </div>
      </main>
    </div>
  )
}
