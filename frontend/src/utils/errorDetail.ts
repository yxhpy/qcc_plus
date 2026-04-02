export interface ParsedErrorDetail {
  rawText: string
  summary: string
  statusCode: string
  severity: string
  code: string
  rawType: string
  rawResponse: string
}

function formatJsonIfPossible(text: string): string {
  const value = text.trim()
  if (!value) return ''

  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

function extractJsonMessage(text: string): string {
  try {
    const payload = JSON.parse(text)
    if (typeof payload?.error?.message === 'string' && payload.error.message.trim()) {
      return payload.error.message.trim()
    }
    if (typeof payload?.message === 'string' && payload.message.trim()) {
      return payload.message.trim()
    }
  } catch {
    return ''
  }
  return ''
}

export function parseErrorDetail(detail?: string): ParsedErrorDetail {
  const rawText = (detail || '').trim()
  const parsed: ParsedErrorDetail = {
    rawText,
    summary: '',
    statusCode: '',
    severity: '',
    code: '',
    rawType: '',
    rawResponse: '',
  }

  if (!rawText) return parsed

  const lines = rawText.split(/\r?\n/)
  const rawIndex = lines.findIndex((line) => line.trim() === '原始响应:')

  const metaLines = (rawIndex >= 0 ? lines.slice(0, rawIndex) : lines)
    .map((line) => line.trim())
    .filter(Boolean)

  for (const line of metaLines) {
    if (line.startsWith('状态码:')) {
      parsed.statusCode = line.replace(/^状态码:\s*/, '').trim()
      continue
    }
    if (line.startsWith('级别:')) {
      parsed.severity = line.replace(/^级别:\s*/, '').trim()
      continue
    }
    if (line.startsWith('错误码:')) {
      parsed.code = line.replace(/^错误码:\s*/, '').trim()
      continue
    }
    if (line.startsWith('原始类型:')) {
      parsed.rawType = line.replace(/^原始类型:\s*/, '').trim()
      continue
    }
    if (line.startsWith('摘要:')) {
      parsed.summary = line.replace(/^摘要:\s*/, '').trim()
    }
  }

  if (rawIndex >= 0) {
    parsed.rawResponse = formatJsonIfPossible(lines.slice(rawIndex + 1).join('\n').trim())
  }

  const legacyUpstream = rawText.match(/^上游错误\s*\((\d+)\):\s*(.+)$/s)
  if (!parsed.statusCode && legacyUpstream) {
    parsed.statusCode = legacyUpstream[1]
    const tail = legacyUpstream[2].trim()
    parsed.rawResponse = formatJsonIfPossible(tail)
    parsed.summary = extractJsonMessage(tail) || `上游错误 (${legacyUpstream[1]})`
  }

  const legacyStatus = rawText.match(/^status\s+(\d+)(?:\s+\[([^\]]+)\])?(?::\s*(.+))?$/is)
  if (!parsed.statusCode && legacyStatus) {
    parsed.statusCode = legacyStatus[1]
    if (!parsed.code && legacyStatus[2]) parsed.code = legacyStatus[2].trim()
    if (!parsed.summary && legacyStatus[3]) parsed.summary = legacyStatus[3].trim()
  }

  if (!parsed.rawResponse) {
    const jsonStart = rawText.search(/[\[{]/)
    if (jsonStart >= 0) {
      const maybeJson = rawText.slice(jsonStart).trim()
      if (maybeJson) {
        parsed.rawResponse = formatJsonIfPossible(maybeJson)
        if (!parsed.summary) {
          parsed.summary = extractJsonMessage(maybeJson) || rawText.slice(0, jsonStart).trim().replace(/[:：]\s*$/, '')
        }
      }
    }
  }

  if (!parsed.summary) {
    const keyExhausted = metaLines.find((line) => line.startsWith('所有 API Key 已失效'))
    if (keyExhausted) {
      parsed.summary = keyExhausted
    } else {
      parsed.summary = metaLines[0] || rawText
    }
  }

  return parsed
}

export function summarizeErrorDetail(detail?: string): string {
  return parseErrorDetail(detail).summary
}

export function hasExpandedErrorDetail(detail?: string): boolean {
  const parsed = parseErrorDetail(detail)
  if (!parsed.rawText) return false
  return !!parsed.rawResponse || parsed.rawText.includes('\n')
}
