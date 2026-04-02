import { detectModelProvider, providerLabels } from '../../config/modelCatalog'
import type { ModelPricing } from '../../types'

interface PricingTableProps {
  pricingList: ModelPricing[]
  onEdit: (pricing: ModelPricing) => void
  onDelete: (modelId: string) => void
}

export default function PricingTable({ pricingList, onEdit, onDelete }: PricingTableProps) {
  return (
    <div className="pricing-table-wrapper">
      <table className="pricing-table">
        <thead>
          <tr>
            <th>模型</th>
            <th>输入价格</th>
            <th>输出价格</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {pricingList.map((pricing) => {
            const provider = detectModelProvider(pricing.model_id)
            return (
              <tr key={pricing.model_id} className={!pricing.is_active ? 'inactive' : ''}>
                <td>
                  <div className="model-info">
                    <div className="model-head">
                      <span className="model-name">{pricing.model_name}</span>
                      <span className={`provider-badge provider-${provider}`}>{providerLabels[provider]}</span>
                    </div>
                    <span className="model-id">{pricing.model_id}</span>
                  </div>
                </td>
                <td className="price-cell">${pricing.input_price_mtok.toFixed(2)}/MTok</td>
                <td className="price-cell">${pricing.output_price_mtok.toFixed(2)}/MTok</td>
                <td>
                  <span className={`status-badge ${pricing.is_active ? 'active' : 'inactive'}`}>
                    {pricing.is_active ? '启用' : '禁用'}
                  </span>
                </td>
                <td className="actions-cell">
                  <button className="btn-small ghost" type="button" onClick={() => onEdit(pricing)}>
                    编辑
                  </button>
                  <button className="btn-small danger" type="button" onClick={() => onDelete(pricing.model_id)}>
                    删除
                  </button>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
