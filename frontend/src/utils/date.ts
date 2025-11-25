const BJ_TZ = 'Asia/Shanghai'
const BJ_FULL_RE = /^(\d{4})年(\d{2})月(\d{2})日\s+(\d{2})时(\d{2})分(\d{2})秒$/
const BJ_SHORT_RE = /^(\d{2})时(\d{2})分(\d{2})秒$/

type PartToken = Intl.DateTimeFormatPartTypes | string

function parseBeijingFormatted(raw: string): Date | null {
  const m = BJ_FULL_RE.exec(raw)
  if (!m) return null
  const [year, month, day, hour, minute, second] = m.slice(1).map((v) => Number(v))
  if ([year, month, day, hour, minute, second].some((v) => Number.isNaN(v))) return null
  const d = new Date(Date.UTC(year, month - 1, day, hour - 8, minute, second))
  return Number.isNaN(d.getTime()) ? null : d
}

function normalizeDateInput(input?: Date | string | number | null): Date | null {
  if (input === undefined || input === null) return null

  // Numbers/Date objects can直接交给 Date 构造器
  if (input instanceof Date || typeof input === 'number') {
    const d = new Date(input)
    return Number.isNaN(d.getTime()) ? null : d
  }

  if (typeof input !== 'string') return null

  const raw = input.trim()
  if (!raw) return null

  const bj = parseBeijingFormatted(raw)
  if (bj) return bj

  // 如果字符串没有任何时区信息（既不包含 Z 也不包含 +/-HH:mm），默认按 UTC 处理，避免被浏览器按本地时区解析。
  const hasTZSuffix = /[zZ]|([+-]\d{2}:?\d{2})$/.test(raw)

  if (!hasTZSuffix) {
    // 统一替换空格为 T，补全缺失的秒和时区标识
    const withT = raw.replace(' ', 'T')

    // 仅含日期的情况：yyyy-MM-dd -> 当天 00:00:00 UTC
    if (/^\d{4}-\d{2}-\d{2}$/.test(withT)) {
      const d = new Date(`${withT}T00:00:00Z`)
      return Number.isNaN(d.getTime()) ? null : d
    }

    // 不带时区的 ISO 样式时间：补一个 Z，按 UTC 解析
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d{1,6})?)?$/.test(withT)) {
      const normalized = withT.endsWith('Z') ? withT : `${withT}Z`
      const d = new Date(normalized)
      return Number.isNaN(d.getTime()) ? null : d
    }
  }

  const d = new Date(raw)
  return Number.isNaN(d.getTime()) ? null : d
}

function formatParts(date: Date, parts: PartToken[]) {
  const fmt = new Intl.DateTimeFormat('zh-CN', {
    timeZone: BJ_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
  const map = fmt.formatToParts(date).reduce<Record<string, string>>((acc, cur) => {
    acc[cur.type] = cur.value
    return acc
  }, {})
  const safe = (key: string) => map[key] || key
  return parts.map((key) => safe(key)).join('')
}

export function formatBeijingTime(input?: Date | string | number | null): string {
  if (typeof input === 'string') {
    const raw = input.trim()
    if (!raw) return '--'
    if (BJ_FULL_RE.test(raw)) return raw
  }
  const date = normalizeDateInput(input)
  if (!date) return '--'
  const parts = formatParts(date, ['year', '年', 'month', '月', 'day', '日', ' ', 'hour', '时', 'minute', '分', 'second', '秒'])
  return parts
}

export function formatBeijingTimeShort(input?: Date | string | number | null): string {
  if (typeof input === 'string') {
    const raw = input.trim()
    if (!raw) return '--'
    if (BJ_SHORT_RE.test(raw)) return raw
  }
  const date = normalizeDateInput(input)
  if (!date) return '--'
  const parts = formatParts(date, ['hour', '时', 'minute', '分', 'second', '秒'])
  return parts
}

// 对外暴露日期解析，便于排序等逻辑复用统一的解析规则。
export function parseToDate(input?: Date | string | number | null): Date | null {
  return normalizeDateInput(input)
}
