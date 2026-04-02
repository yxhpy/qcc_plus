export type SupportedSourceProtocol = 'claude' | 'openai' | 'gemini'
export type ModelProvider = SupportedSourceProtocol | 'other'

export const defaultHealthCheckModels: Record<SupportedSourceProtocol, string> = {
  claude: 'claude-haiku-4-5-20251001',
  openai: 'gpt-5.1-codex-mini',
  gemini: 'gemini-2.5-flash',
}

const legacyDefaultHealthCheckModels: Partial<Record<SupportedSourceProtocol, string[]>> = {
  openai: ['gpt-5.4'],
}

export const healthCheckModelOptions: Record<SupportedSourceProtocol, string[]> = {
  claude: [
    'claude-haiku-4-5-20251001',
    'claude-sonnet-4-5-20250929',
    'claude-opus-4-1-20250805',
    'claude-sonnet-4-20250514',
    'claude-3-7-sonnet-20250219',
  ],
  openai: [
    'gpt-5.1-codex-mini',
    'gpt-5.1-codex',
    'gpt-5.4',
    'gpt-5.4-mini',
    'gpt-5.4-nano',
    'gpt-5-mini',
  ],
  gemini: [
    'gemini-2.5-flash',
    'gemini-2.5-flash-lite',
    'gemini-2.5-pro',
    'gemini-2.0-flash',
  ],
}

export const allKnownModelIds = Array.from(
  new Set([
    ...Object.values(healthCheckModelOptions).flat(),
    'claude-3-5-sonnet-20241022',
    'claude-3-5-haiku-20241022',
    'claude-3-haiku-20240307',
    'gpt-5',
    'gpt-5.1',
    'gpt-5-codex',
  ]),
)

export const providerLabels: Record<ModelProvider, string> = {
  claude: 'Anthropic',
  openai: 'OpenAI',
  gemini: 'Gemini',
  other: 'Custom',
}

export function normalizeHealthCheckModel(protocol: SupportedSourceProtocol, model?: string | null): string {
  const trimmed = (model || '').trim()
  if (!trimmed) return defaultHealthCheckModels[protocol]

  const legacyModels = legacyDefaultHealthCheckModels[protocol] || []
  if (legacyModels.includes(trimmed)) {
    return defaultHealthCheckModels[protocol]
  }

  return trimmed
}

export function detectModelProvider(modelId?: string | null): ModelProvider {
  const value = (modelId || '').trim().toLowerCase()
  if (!value) return 'other'
  if (value.startsWith('claude-')) return 'claude'
  if (value.startsWith('gpt-') || value.startsWith('o1') || value.startsWith('o3') || value.startsWith('o4') || value.startsWith('codex-')) {
    return 'openai'
  }
  if (value.startsWith('gemini-')) return 'gemini'
  return 'other'
}

export const integrationRoutes = [
  {
    title: 'Claude / Claude Code',
    protocol: 'Anthropic Messages',
    route: '/v1/messages',
    bodyHint: '传标准 Claude Messages 请求体',
    note: '继续兼容 Claude Code 默认接入方式。',
  },
  {
    title: 'OpenAI 兼容',
    protocol: 'Chat Completions / Responses',
    route: '/v1/chat/completions 或 /v1/responses',
    bodyHint: '按 OpenAI 原生格式直连',
    note: '支持标准 /v1 入口，不再要求额外 /openai 前缀。',
  },
  {
    title: 'Gemini 兼容',
    protocol: 'Generate Content',
    route: '/v1beta/models/{model}:generateContent',
    bodyHint: '按 Gemini 原生 generateContent 请求体直连',
    note: '支持标准 /v1beta 入口，不再要求额外 /gemini 前缀。',
  },
] as const
