import { detectModelProvider, providerLabels } from '../../config/modelCatalog'
import type { ModelPricing } from '../../types'

interface PricingCatalogOverviewProps {
  pricingList: ModelPricing[]
  lastUpdatedAt: string
}

const providerOrder = ['claude', 'openai', 'gemini', 'other'] as const

export default function PricingCatalogOverview({ pricingList, lastUpdatedAt }: PricingCatalogOverviewProps) {
  const counts = providerOrder.reduce<Record<string, number>>((acc, provider) => {
    acc[provider] = 0
    return acc
  }, {})

  for (const item of pricingList) {
    counts[detectModelProvider(item.model_id)] += 1
  }

  return (
    <div className="pricing-overview">
      <div className="pricing-overview-card">
        <span className="pricing-overview-label">模型总数</span>
        <strong>{pricingList.length}</strong>
      </div>
      {providerOrder.map((provider) => (
        <div key={provider} className={`pricing-overview-card provider-${provider}`}>
          <span className="pricing-overview-label">{providerLabels[provider]}</span>
          <strong>{counts[provider]}</strong>
        </div>
      ))}
      <div className="pricing-overview-meta">
        <span className="pricing-realtime-note">
          实时刷新中，每 3 秒同步一次{lastUpdatedAt ? `，最近更新 ${lastUpdatedAt}` : ''}
        </span>
        <span className="pricing-catalog-note">Gemini 展示基础文本档位价格；遇到供应商自定义价格可在此页直接覆盖。</span>
      </div>
    </div>
  )
}
