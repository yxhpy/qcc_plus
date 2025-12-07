import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import api from '../services/api'
import type { ModelPricing } from '../types'
import './Pricing.css'

export default function Pricing() {
  const [pricingList, setPricingList] = useState<ModelPricing[]>([])
  const [loading, setLoading] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)

  // 表单状态
  const [formData, setFormData] = useState({
    model_id: '',
    model_name: '',
    input_price_mtok: 0,
    output_price_mtok: 0,
    is_active: true,
  })

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const loadPricing = async () => {
    setLoading(true)
    try {
      const list = await api.getPricingList()
      setPricingList(list)
    } catch (err) {
      showToast((err as Error).message || '加载失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadPricing()
  }, [])

  const resetForm = () => {
    setFormData({
      model_id: '',
      model_name: '',
      input_price_mtok: 0,
      output_price_mtok: 0,
      is_active: true,
    })
    setEditingId(null)
    setShowAddForm(false)
  }

  const handleEdit = (pricing: ModelPricing) => {
    setFormData({
      model_id: pricing.model_id,
      model_name: pricing.model_name,
      input_price_mtok: pricing.input_price_mtok,
      output_price_mtok: pricing.output_price_mtok,
      is_active: pricing.is_active,
    })
    setEditingId(pricing.model_id)
    setShowAddForm(true)
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!formData.model_id.trim()) {
      showToast('请输入模型 ID', 'error')
      return
    }
    try {
      await api.savePricing(formData)
      showToast(editingId ? '定价已更新' : '定价已添加')
      resetForm()
      loadPricing()
    } catch (err) {
      showToast((err as Error).message || '保存失败', 'error')
    }
  }

  const handleDelete = async (modelId: string) => {
    if (!confirm(`确定要删除模型 "${modelId}" 的定价配置吗？`)) return
    try {
      await api.deletePricing(modelId)
      showToast('已删除')
      loadPricing()
    } catch (err) {
      showToast((err as Error).message || '删除失败', 'error')
    }
  }

  const formatPrice = (price: number) => {
    return `$${price.toFixed(2)}`
  }

  return (
    <div className="pricing-page">
      <div className="pricing-header">
        <h1>模型定价</h1>
        <p className="sub">管理 Claude 模型的定价配置，用于计算 API 使用成本。</p>
      </div>

      <Card>
        <div className="toolbar">
          <span className="toolbar-title">定价列表</span>
          <div className="spacer" />
          <button className="btn ghost" type="button" onClick={loadPricing} disabled={loading}>
            刷新
          </button>
          <button
            className="btn primary"
            type="button"
            onClick={() => {
              resetForm()
              setShowAddForm(true)
            }}
          >
            添加定价
          </button>
        </div>

        {showAddForm && (
          <form className="pricing-form" onSubmit={handleSubmit}>
            <div className="form-row">
              <label>
                <span className="label-title">模型 ID *</span>
                <input
                  type="text"
                  value={formData.model_id}
                  onChange={(e) => setFormData({ ...formData, model_id: e.target.value })}
                  placeholder="claude-sonnet-4-5-20250929"
                  disabled={!!editingId}
                  required
                />
              </label>
              <label>
                <span className="label-title">显示名称</span>
                <input
                  type="text"
                  value={formData.model_name}
                  onChange={(e) => setFormData({ ...formData, model_name: e.target.value })}
                  placeholder="Claude Sonnet 4.5"
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
                  onChange={(e) => setFormData({ ...formData, input_price_mtok: parseFloat(e.target.value) || 0 })}
                />
              </label>
              <label>
                <span className="label-title">输出价格 ($/MTok)</span>
                <input
                  type="number"
                  step="0.01"
                  min="0"
                  value={formData.output_price_mtok}
                  onChange={(e) => setFormData({ ...formData, output_price_mtok: parseFloat(e.target.value) || 0 })}
                />
              </label>
              <label className="checkbox-label">
                <input
                  type="checkbox"
                  checked={formData.is_active}
                  onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
                />
                <span>启用</span>
              </label>
            </div>
            <div className="form-actions">
              <button className="btn ghost" type="button" onClick={resetForm}>
                取消
              </button>
              <button className="btn primary" type="submit">
                {editingId ? '更新' : '添加'}
              </button>
            </div>
          </form>
        )}

        {loading ? (
          <div className="loading-text">加载中...</div>
        ) : pricingList.length === 0 ? (
          <div className="empty-text">暂无定价配置</div>
        ) : (
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
                {pricingList.map((pricing) => (
                  <tr key={pricing.model_id} className={!pricing.is_active ? 'inactive' : ''}>
                    <td>
                      <div className="model-info">
                        <span className="model-name">{pricing.model_name}</span>
                        <span className="model-id">{pricing.model_id}</span>
                      </div>
                    </td>
                    <td className="price-cell">{formatPrice(pricing.input_price_mtok)}/MTok</td>
                    <td className="price-cell">{formatPrice(pricing.output_price_mtok)}/MTok</td>
                    <td>
                      <span className={`status-badge ${pricing.is_active ? 'active' : 'inactive'}`}>
                        {pricing.is_active ? '启用' : '禁用'}
                      </span>
                    </td>
                    <td className="actions-cell">
                      <button className="btn-small ghost" onClick={() => handleEdit(pricing)}>
                        编辑
                      </button>
                      <button className="btn-small danger" onClick={() => handleDelete(pricing.model_id)}>
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <div className="notice">
          价格单位：美元/百万 Token（MTok）。例如 $3.00/MTok 表示每处理 100 万个 Token 收费 3 美元。
        </div>
      </Card>
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
