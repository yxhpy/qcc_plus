import type { FormEvent } from 'react'

interface PricingFormData {
  model_id: string
  model_name: string
  input_price_mtok: number
  output_price_mtok: number
  is_active: boolean
}

interface PricingFormProps {
  formData: PricingFormData
  editingId: string | null
  onChange: (patch: Partial<PricingFormData>) => void
  onCancel: () => void
  onSubmit: (e: FormEvent) => void
}

export default function PricingForm({ formData, editingId, onChange, onCancel, onSubmit }: PricingFormProps) {
  return (
    <form className="pricing-form" onSubmit={onSubmit}>
      <div className="form-row">
        <label>
          <span className="label-title">模型 ID *</span>
          <input
            type="text"
            value={formData.model_id}
            onChange={(e) => onChange({ model_id: e.target.value })}
            placeholder="gpt-5.1-codex-mini"
            disabled={!!editingId}
            required
          />
        </label>
        <label>
          <span className="label-title">显示名称</span>
          <input
            type="text"
            value={formData.model_name}
            onChange={(e) => onChange({ model_name: e.target.value })}
            placeholder="GPT-5.1 Codex mini"
          />
        </label>
      </div>
      <div className="form-row">
        <label>
          <span className="label-title">输入价格 ($/MTok)</span>
          <input
            type="number"
            step="0.01"
            min="0"
            value={formData.input_price_mtok}
            onChange={(e) => onChange({ input_price_mtok: parseFloat(e.target.value) || 0 })}
          />
        </label>
        <label>
          <span className="label-title">输出价格 ($/MTok)</span>
          <input
            type="number"
            step="0.01"
            min="0"
            value={formData.output_price_mtok}
            onChange={(e) => onChange({ output_price_mtok: parseFloat(e.target.value) || 0 })}
          />
        </label>
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={formData.is_active}
            onChange={(e) => onChange({ is_active: e.target.checked })}
          />
          <span>启用</span>
        </label>
      </div>
      <div className="form-actions">
        <button className="btn ghost" type="button" onClick={onCancel}>
          取消
        </button>
        <button className="btn primary" type="submit">
          {editingId ? '更新' : '添加'}
        </button>
      </div>
    </form>
  )
}
