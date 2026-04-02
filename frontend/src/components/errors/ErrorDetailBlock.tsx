import { hasExpandedErrorDetail, parseErrorDetail } from '../../utils/errorDetail'
import './ErrorDetailBlock.css'

interface ErrorDetailBlockProps {
  detail?: string
  summaryOnly?: boolean
  className?: string
}

export default function ErrorDetailBlock({ detail, summaryOnly = false, className = '' }: ErrorDetailBlockProps) {
  const text = (detail || '').trim()
  if (!text) return null

  const parsed = parseErrorDetail(text)
  const summary = parsed.summary
  const expanded = hasExpandedErrorDetail(text) && !summaryOnly
  const rawContent = parsed.rawResponse || text

  return (
    <div className={`error-detail-block ${className}`.trim()}>
      {(parsed.statusCode || parsed.code || parsed.severity || parsed.rawType) && (
        <div className="error-detail-meta">
          {parsed.statusCode && <span className="error-detail-chip status">状态码 {parsed.statusCode}</span>}
          {parsed.code && <span className="error-detail-chip">错误码 {parsed.code}</span>}
          {parsed.severity && <span className="error-detail-chip">级别 {parsed.severity}</span>}
          {parsed.rawType && <span className="error-detail-chip">类型 {parsed.rawType}</span>}
        </div>
      )}
      {summary && <div className="error-detail-summary" title={summary}>{summary}</div>}
      {expanded && (
        <div className="error-detail-raw-section">
          <div className="error-detail-raw-label">{parsed.rawResponse ? '原始 JSON' : '原始响应'}</div>
          <pre className="error-detail-raw">{rawContent}</pre>
        </div>
      )}
    </div>
  )
}
