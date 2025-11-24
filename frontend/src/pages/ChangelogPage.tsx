import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import type { Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import api from '../services/api'

import './ChangelogPage.css'

const markdownComponents: Components = {
  a({ node, ...props }) {
    return <a target="_blank" rel="noreferrer" {...props} />
  },
  code({ inline, className, children, ...props }: any) {
    const language = className?.replace('language-', '')
    if (inline) {
      return (
        <code className="inline-code" {...props}>
          {children}
        </code>
      )
    }
    return (
      <pre className={`code-block${language ? ` lang-${language}` : ''}`}>
        <code {...props}>{children}</code>
      </pre>
    )
  },
}

export default function ChangelogPage() {
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchChangelog = async () => {
    setLoading(true)
    setError('')
    try {
      const text = await api.getChangelog()
      setContent(text)
    } catch (err) {
      setError((err as Error).message || '加载更新日志失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchChangelog()
  }, [])

  const pageTitle = useMemo(() => {
    if (loading) return '更新日志 · 加载中'
    if (error) return '更新日志 · 加载失败'
    return '更新日志'
  }, [loading, error])

  return (
    <div className="changelog-page">
      <div className="changelog-header">
        <div>
          <div className="breadcrumb">
            <Link to="/admin/dashboard">仪表盘</Link>
            <span className="breadcrumb-sep">/</span>
            <span className="breadcrumb-current">更新日志</span>
          </div>
          <h1 className="changelog-title">{pageTitle}</h1>
          <p className="changelog-sub">追踪版本变更与新功能，保持与后端发布同步。</p>
        </div>
        <div className="changelog-actions">
          <button className="btn-refresh" type="button" onClick={fetchChangelog} disabled={loading}>
            {loading ? '加载中...' : '重新加载'}
          </button>
        </div>
      </div>

      <div className="changelog-card">
        {loading && <div className="changelog-loading">正在读取 CHANGELOG.md ...</div>}
        {!loading && error && <div className="changelog-error">{error}</div>}
        {!loading && !error && (
          <div className="changelog-markdown">
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
              {content}
            </ReactMarkdown>
          </div>
        )}
      </div>
    </div>
  )
}
