import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTheme } from '../../contexts/ThemeContext'
import {
  listTwitterAccounts,
  createTwitterAccount,
  updateTwitterAccount,
  deleteTwitterAccount,
  getTwitterConfig,
  setTwitterConfig,
  bulkImportTwitterAccounts,
  type TrackedAccount,
} from '../../lib/api'

export default function SettingsPage() {
  const navigate = useNavigate()
  const { theme, toggleTheme } = useTheme()
  const [provider, setProvider] = useState('opencode')
  const [model, setModel] = useState('deepseek-v4-pro')
  const [apiKey, setApiKey] = useState('')
  const [tavilyApiKey, setTavilyApiKey] = useState('')
  const [saved, setSaved] = useState(false)

  // Twitter / RSSHub state
  const [accounts, setAccounts] = useState<TrackedAccount[]>([])
  const [newHandle, setNewHandle] = useState('')
  const [rsshubURL, setRsshubURL] = useState('https://rsshub.app')
  const [bulkImportUrl, setBulkImportUrl] = useState('')
  const [bulkImportStatus, setBulkImportStatus] = useState<{ kind: 'idle' | 'loading' | 'ok' | 'error'; message: string }>({ kind: 'idle', message: '' })

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

  useEffect(() => {
    listTwitterAccounts()
      .then(data => setAccounts(data ?? []))
      .catch(() => {})
    getTwitterConfig()
      .then(c => setRsshubURL(c.rsshub_base_url))
      .catch(() => {})
  }, [])

  const reloadAccounts = async () => {
    try {
      const data = await listTwitterAccounts()
      setAccounts(data ?? [])
    } catch {
      /* ignore */
    }
  }

  const handleAddAccount = async () => {
    const handle = newHandle.trim()
    if (!handle) return
    try {
      await createTwitterAccount(handle)
      setNewHandle('')
      await reloadAccounts()
    } catch (e) {
      alert(`添加失败: ${(e as Error).message}`)
    }
  }

  const handleToggleAccount = async (a: TrackedAccount, enabled: boolean) => {
    try {
      await updateTwitterAccount(a.id, { enabled })
      await reloadAccounts()
    } catch (e) {
      alert(`更新失败: ${(e as Error).message}`)
    }
  }

  const handleDeleteAccount = async (a: TrackedAccount) => {
    if (!confirm(`删除 @${a.handle}？`)) return
    try {
      await deleteTwitterAccount(a.id)
      await reloadAccounts()
    } catch (e) {
      alert(`删除失败: ${(e as Error).message}`)
    }
  }

  const handleSaveRsshub = async () => {
    try {
      await setTwitterConfig(rsshubURL.trim())
      alert('已保存')
    } catch (e) {
      alert(`保存失败: ${(e as Error).message}`)
    }
  }

  const handleBulkImportBuiltin = async () => {
    setBulkImportStatus({ kind: 'loading', message: '正在拉取 follow-builders 列表...' })
    try {
      const r = await bulkImportTwitterAccounts()
      setBulkImportStatus({
        kind: 'ok',
        message: `✅ 找到 ${r.total_found} 个，新增 ${r.added} 个，跳过 ${r.skipped_existing} 个已存在`,
      })
      await reloadAccounts()
    } catch (e) {
      setBulkImportStatus({ kind: 'error', message: `❌ 导入失败: ${(e as Error).message}` })
    }
  }

  const handleBulkImportCustom = async () => {
    const url = bulkImportUrl.trim()
    if (!url) return
    setBulkImportStatus({ kind: 'loading', message: `正在拉取 ${url} ...` })
    try {
      const r = await bulkImportTwitterAccounts(url)
      setBulkImportStatus({
        kind: 'ok',
        message: `✅ 找到 ${r.total_found} 个，新增 ${r.added} 个，跳过 ${r.skipped_existing} 个已存在`,
      })
      setBulkImportUrl('')
      await reloadAccounts()
    } catch (e) {
      setBulkImportStatus({ kind: 'error', message: `❌ 导入失败: ${(e as Error).message}` })
    }
  }

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

          <section className="bg-th-bg-secondary border border-th-border rounded-lg shadow-th p-6 space-y-5 mt-6">
            <div>
              <h2 className="font-display text-lg font-semibold text-th-text-primary">
                推文账号
              </h2>
              <p className="text-xs text-th-text-muted mt-1">
                追踪你关心的 Twitter 账号，定时拉取并交给 AI 整理为知识。
              </p>
            </div>

            <div className='space-y-2 pb-4 border-b border-th-border'>
              <div className='text-xs font-semibold text-th-text-muted tracking-[0.14em] uppercase font-display'>批量导入</div>
              <button
                onClick={handleBulkImportBuiltin}
                disabled={bulkImportStatus.kind === 'loading'}
                className='px-3 py-2 bg-th-accent text-white rounded-md text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed active:scale-[0.98] transition-all duration-150 shadow-th'
              >
                📥 一键导入 follow-builders (26 人)
              </button>
              <div className='flex gap-2'>
                <input
                  type='url'
                  value={bulkImportUrl}
                  onChange={e => setBulkImportUrl(e.target.value)}
                  placeholder='或粘贴 GitHub raw URL (每行一个 handle)'
                  className='flex-1 border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent/30 focus:border-th-accent transition-colors'
                />
                <button
                  onClick={handleBulkImportCustom}
                  disabled={!bulkImportUrl.trim() || bulkImportStatus.kind === 'loading'}
                  className='px-3 py-2 bg-th-bg-primary border border-th-border text-th-text-primary rounded-md text-sm font-medium hover:bg-th-bg-secondary disabled:opacity-50 disabled:cursor-not-allowed transition-all'
                >
                  从 URL 导入
                </button>
              </div>
              {bulkImportStatus.kind !== 'idle' && (
                <p className={`text-xs ${bulkImportStatus.kind === 'error' ? 'text-red-500' : 'text-th-text-muted'}`}>
                  {bulkImportStatus.message}
                </p>
              )}
            </div>

            <div className="flex gap-2">
              <input
                value={newHandle}
                onChange={(e) => setNewHandle(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleAddAccount()
                }}
                placeholder="@handle (例如 karpathy)"
                className="flex-1 border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent/30 focus:border-th-accent transition-colors"
              />
              <button
                onClick={handleAddAccount}
                disabled={!newHandle.trim()}
                className="px-4 py-2 bg-th-accent text-white rounded-md text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed active:scale-[0.98] transition-all duration-150 shadow-th"
              >
                + 添加
              </button>
            </div>

            {accounts.length === 0 ? (
              <p className="text-sm text-th-text-muted py-4 text-center border border-dashed border-th-border rounded-md">
                尚未添加任何账号
              </p>
            ) : (
              <ul className="divide-y divide-th-border border border-th-border rounded-md overflow-hidden">
                {accounts.map((a) => (
                  <li key={a.id} className="flex items-center gap-3 px-3 py-2 bg-th-bg-primary">
                    <input
                      type="checkbox"
                      checked={a.enabled}
                      onChange={(e) => handleToggleAccount(a, e.target.checked)}
                      className="w-4 h-4 accent-th-accent"
                    />
                    <span className="flex-1 text-sm text-th-text-primary">
                      <span className="font-mono">@{a.handle}</span>
                      {a.display_name ? (
                        <span className="text-th-text-muted ml-2">({a.display_name})</span>
                      ) : null}
                      {a.notes ? (
                        <span className="text-th-text-muted ml-2">— {a.notes}</span>
                      ) : null}
                    </span>
                    <button
                      onClick={() => handleDeleteAccount(a)}
                      className="text-xs text-red-500 hover:text-red-600 transition-colors"
                    >
                      删除
                    </button>
                  </li>
                ))}
              </ul>
            )}

            <div className="pt-2 border-t border-th-border">
              <label className="block text-[11px] font-semibold text-th-text-muted tracking-[0.14em] uppercase mb-1.5">
                RSSHub Base URL
              </label>
              <div className="flex gap-2">
                <input
                  value={rsshubURL}
                  onChange={(e) => setRsshubURL(e.target.value)}
                  placeholder="https://rsshub.app"
                  className="flex-1 border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-th-accent/30 focus:border-th-accent transition-colors"
                />
                <button
                  onClick={handleSaveRsshub}
                  disabled={!rsshubURL.trim()}
                  className="px-4 py-2 bg-th-accent text-white rounded-md text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed active:scale-[0.98] transition-all duration-150 shadow-th"
                >
                  保存
                </button>
              </div>
              <p className="text-xs text-th-text-muted mt-2">
                默认 https://rsshub.app（公网实例常被 X 屏蔽）。建议自部署 RSSHub。
              </p>
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
