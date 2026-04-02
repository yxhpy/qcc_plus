import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import Card from '../components/Card'
import PricingCatalogOverview from '../components/pricing/PricingCatalogOverview'
import PricingForm from '../components/pricing/PricingForm'
import PricingTable from '../components/pricing/PricingTable'
import Toast from '../components/Toast'
import api from '../services/api'
import type { ModelPricing } from '../types'
import './Pricing.css'

export default function Pricing() {
  const [pricingList, setPricingList] = useState<ModelPricing[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string>('')

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

  const loadPricing = async (silent = false) => {
    if (!silent) {
      setLoading(true)
    }
    try {
      const list = await api.getPricingList()
      setPricingList(list)
      setLastUpdatedAt(new Date().toLocaleTimeString())
    } catch (err) {
      if (!silent) {
        showToast((err as Error).message || '加载失败', 'error')
      }
    } finally {
      if (!silent) {
        setLoading(false)
      }
    }
  }

  useEffect(() => {
    void loadPricing()
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible') {
        void loadPricing(true)
      }
    }, 3000)
    return () => window.clearInterval(timer)
  }, [])

  const handleSync = async () => {
    if (!confirm('确定要从官方同步定价数据吗？已有模型的价格将被更新为官方最新价格。')) return
    setSyncing(true)
    try {
      const result = await api.syncOfficialPricing()
      showToast(`同步完成，已更新 ${result.synced} 个模型定价`)
      await loadPricing(true)
    } catch (err) {
      showToast((err as Error).message || '同步失败', 'error')
    } finally {
      setSyncing(false)
    }
  }

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
      await loadPricing(true)
    } catch (err) {
      showToast((err as Error).message || '保存失败', 'error')
    }
  }

  const handleDelete = async (modelId: string) => {
    if (!confirm(`确定要删除模型 "${modelId}" 的定价配置吗？`)) return
    try {
      await api.deletePricing(modelId)
      showToast('已删除')
      await loadPricing(true)
    } catch (err) {
      showToast((err as Error).message || '删除失败', 'error')
    }
  }

  return (
    <div className="pricing-page">
      <div className="pricing-header">
        <h1>模型定价</h1>
        <p className="sub">统一管理 Anthropic、OpenAI、Gemini 与自定义渠道模型定价，并实时同步页面状态。</p>
      </div>

      <Card>
        <div className="toolbar">
          <span className="toolbar-title">定价列表</span>
          <div className="spacer" />
          <button className="btn ghost" type="button" onClick={() => void loadPricing()} disabled={loading}>
            刷新
          </button>
          <button className="btn ghost" type="button" onClick={handleSync} disabled={syncing}>
            {syncing ? '同步中...' : '同步官方定价'}
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

        <PricingCatalogOverview pricingList={pricingList} lastUpdatedAt={lastUpdatedAt} />

        {showAddForm && (
          <PricingForm
            formData={formData}
            editingId={editingId}
            onChange={(patch) => setFormData((prev) => ({ ...prev, ...patch }))}
            onCancel={resetForm}
            onSubmit={handleSubmit}
          />
        )}

        {loading ? (
          <div className="loading-text">加载中...</div>
        ) : pricingList.length === 0 ? (
          <div className="empty-text">暂无定价配置</div>
        ) : (
          <PricingTable pricingList={pricingList} onEdit={handleEdit} onDelete={handleDelete} />
        )}

        <div className="notice">
          价格单位：美元/百万 Token（MTok）。Gemini 若存在长上下文分档、音视频或缓存价格差异，这里默认展示基础文本档位，必要时可按渠道实际账单手动覆盖。
        </div>
      </Card>
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
