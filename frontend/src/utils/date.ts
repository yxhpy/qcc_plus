const BJ_TZ = 'Asia/Shanghai'

type PartToken = Intl.DateTimeFormatPartTypes | string

function toDate(input?: Date | string | number | null): Date | null {
  if (input === undefined || input === null) return null
  const d = input instanceof Date ? input : new Date(input)
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
  const date = toDate(input)
  if (!date) return '--'
  const parts = formatParts(date, ['year', '年', 'month', '月', 'day', '日', ' ', 'hour', '时', 'minute', '分', 'second', '秒'])
  return parts
}

export function formatBeijingTimeShort(input?: Date | string | number | null): string {
  const date = toDate(input)
  if (!date) return '--'
  const parts = formatParts(date, ['hour', '时', 'minute', '分', 'second', '秒'])
  return parts
}
