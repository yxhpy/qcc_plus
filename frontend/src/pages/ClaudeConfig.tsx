import { useEffect, useMemo, useRef, useState } from 'react'
import Card from '../components/Card'
import IntegrationGuide from '../components/claude-config/IntegrationGuide'
import Toast from '../components/Toast'
import api from '../services/api'
import type { ClaudeConfigTemplate } from '../types'
import './ClaudeConfig.css'

type Tab = 'unix' | 'windows'

type ToastState = { message: string; type?: 'success' | 'error' } | null

const parseList = (input: string): string[] =>
  input
    .split(/[,\n;]+/)
    .map((v) => v.trim())
    .filter(Boolean)

const formatTime = (iso?: string | null): string => {
  if (!iso) return '--'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '--'
  return `${date.getMonth() + 1}/${date.getDate()} ${date.toLocaleTimeString()}`
}

export default function ClaudeConfig() {
  const [template, setTemplate] = useState<ClaudeConfigTemplate | null>(null)
  const [proxyUrl, setProxyUrl] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState('')
  const [allowInput, setAllowInput] = useState('')
  const [denyInput, setDenyInput] = useState('')
  const [commandTab, setCommandTab] = useState<Tab>('unix')
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [toast, setToast] = useState<ToastState>(null)
  const [updatedAt, setUpdatedAt] = useState<string | null>(null)
  const initializedRef = useRef(false)
  const debounceRef = useRef<number | undefined>(undefined)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    window.setTimeout(() => setToast(null), 2000)
  }

  const hydrateInitial = async () => {
    setLoading(true)
    try {
      const data = await api.getClaudeConfigTemplate({})
      setTemplate(data)
      setProxyUrl(data.proxy_url || (typeof window !== 'undefined' ? window.location.origin : ''))
      setApiKey(data.api_key || '')
      setUpdatedAt(new Date().toISOString())
    } catch (err) {
      showToast((err as Error)?.message || '加载配置模板失败', 'error')
    } finally {
      setLoading(false)
      initializedRef.current = true
    }
  }

  const refreshTemplate = async (silent = false) => {
    if (!initializedRef.current) return
    if (!silent) setSyncing(true)
    try {
      const data = await api.getClaudeConfigTemplate({
        proxy_url: proxyUrl.trim(),
        api_key: apiKey.trim(),
        model: model.trim() || undefined,
        allow: parseList(allowInput),
        deny: parseList(denyInput),
      })
      setTemplate(data)
      setUpdatedAt(new Date().toISOString())
    } catch (err) {
      showToast((err as Error)?.message || '刷新失败', 'error')
    } finally {
      if (!silent) setSyncing(false)
    }
  }

  useEffect(() => {
    hydrateInitial()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!initializedRef.current) return
    if (debounceRef.current) window.clearTimeout(debounceRef.current)
    debounceRef.current = window.setTimeout(() => refreshTemplate(true), 420)
    return () => {
      if (debounceRef.current) window.clearTimeout(debounceRef.current)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [proxyUrl, apiKey, model, allowInput, denyInput])

  const installCommand = useMemo(() => {
    if (!template) return ''
    return commandTab === 'windows' ? template.install_cmd.windows : template.install_cmd.unix
  }, [commandTab, template])

  const copyText = async (text: string, label: string) => {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      showToast(`${label} 已复制到剪贴板`)
    } catch (err) {
      showToast('复制失败，请手动复制', 'error')
    }
  }

  const downloadJSON = () => {
    if (!template?.config_id) return
    window.open(`/api/claude-config/download/${template.config_id}`, '_blank')
  }

  const resetPermissions = () => {
    setAllowInput('')
    setDenyInput('')
  }

  return (
    <div className="claude-config-page">
      <div className="claude-config-header">
        <div>
          <h1>Claude Code 快速配置</h1>
          <p className="sub">生成 settings.json、复制安装命令，一步完成本地接入。</p>
        </div>
        <div className="header-actions">
          <span className="pill">当前账号：{template?.account_name || '—'}</span>
          {updatedAt && <span className="pill ghost">更新于 {formatTime(updatedAt)}</span>}
        </div>
      </div>

      <Card className="install-card">
        <div className="install-top">
          <div>
            <div className="eyebrow">一键安装命令</div>
            <div className="title-line">复制到终端直接生成 ~/.claude/settings.json</div>
          </div>
          <div className="install-actions">
            <div className="tabs" role="tablist">
              <button
                className={`tab ${commandTab === 'unix' ? 'active' : ''}`}
                onClick={() => setCommandTab('unix')}
                type="button"
              >
                macOS / Linux
              </button>
              <button
                className={`tab ${commandTab === 'windows' ? 'active' : ''}`}
                onClick={() => setCommandTab('windows')}
                type="button"
              >
                Windows PowerShell
              </button>
            </div>
            <button
              className="btn primary large"
              type="button"
              onClick={() => copyText(installCommand, '安装命令')}
              disabled={!installCommand}
            >
              复制命令
            </button>
            <button className="btn secondary" type="button" onClick={downloadJSON} disabled={!template?.config_id}>
              下载 JSON
            </button>
          </div>
        </div>
        <div className="command-block">
          <pre className="command-text">{installCommand || '加载中...'}</pre>
          <div className="command-meta">
            <div className="badge">Config ID: {template?.config_id || '—'}</div>
            <div className="badge ghost">直连：{template?.proxy_url || proxyUrl || '—'}</div>
          </div>
        </div>
      </Card>

      <div className="config-grid">
        <Card className="config-card">
          <div className="section-head">
            <div>
              <div className="eyebrow">基础信息</div>
              <h3 className="section-title">代理与密钥</h3>
            </div>
            <div className="tiny-actions">
              <button className="btn ghost small" type="button" onClick={() => refreshTemplate(false)} disabled={syncing || loading}>
                {syncing ? '同步中…' : '重新生成' }
              </button>
            </div>
          </div>
          <div className="form-grid">
            <label className="field">
              <span>代理地址</span>
              <input
                value={proxyUrl}
                onChange={(e) => setProxyUrl(e.target.value)}
                placeholder="https://your-proxy.example.com"
              />
              <small>默认取当前服务器地址，可自定义。</small>
            </label>
            <label className="field">
              <span>Proxy API Key</span>
              <input
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="复制账号的 proxy_api_key"
              />
              <small>将写入 ANTHROPIC_API_KEY。</small>
            </label>
            <label className="field">
              <span>默认模型（可选）</span>
              <input
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="例如 claude-3-5-sonnet-20241022"
              />
              <small>不填则使用 CLI 默认模型。</small>
            </label>
          </div>

          <div className="section-head compact">
            <div>
              <div className="eyebrow">权限配置（可选）</div>
              <h3 className="section-title">允许 / 拒绝列表</h3>
            </div>
            <button className="btn ghost small" type="button" onClick={resetPermissions}>
              清空
            </button>
          </div>
          <div className="perm-grid">
            <label className="field">
              <span>允许执行</span>
              <textarea
                rows={3}
                value={allowInput}
                onChange={(e) => setAllowInput(e.target.value)}
                placeholder="例如: git status\nnpm run lint"
              />
              <small>多行或逗号分隔。</small>
            </label>
            <label className="field">
              <span>拒绝执行</span>
              <textarea
                rows={3}
                value={denyInput}
                onChange={(e) => setDenyInput(e.target.value)}
                placeholder="例如: rm -rf /"
              />
            </label>
          </div>
        </Card>

        <Card className="preview-card">
          <div className="section-head">
            <div>
              <div className="eyebrow">实时预览</div>
              <h3 className="section-title">settings.json</h3>
            </div>
            <div className="tiny-actions">
              <button className="btn ghost small" type="button" onClick={() => copyText(template?.config_json || '', '配置文件')}>
                复制 JSON
              </button>
              <button className="btn ghost small" type="button" onClick={downloadJSON} disabled={!template?.config_id}>
                下载
              </button>
            </div>
          </div>
          <div className="json-preview">
            <pre>{template?.config_json || '{\n  "env": {\n    "ANTHROPIC_BASE_URL": "…"\n  }\n}'}</pre>
          </div>
          <div className="footnote">
            运行命令后将生成于 <code>~/.claude/settings.json</code>（Windows: <code>%USERPROFILE%\.claude\settings.json</code>）。
          </div>
        </Card>
      </div>

      <Card className="integration-doc-card">
        <div className="section-head">
          <div>
            <div className="eyebrow">接入文档</div>
            <h3 className="section-title">三种协议入口</h3>
          </div>
          <div className="tiny-actions">
            <span className="badge ghost">支持标准 /v1 与 /v1beta 路径</span>
          </div>
        </div>
        <IntegrationGuide proxyUrl={proxyUrl} apiKey={apiKey} />
      </Card>

      {(loading || syncing) && <div className="loading-line">{loading ? '加载中…' : '正在同步配置…'}</div>}
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
