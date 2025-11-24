import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import useDialog from '../hooks/useDialog'
import api from '../services/api'
import type { EventType, NotificationChannel, NotificationSubscription } from '../types'
import './Notifications.css'

const channelTypes = [
  { value: 'wechat_work', label: '企业微信' },
  { value: 'dingtalk', label: '钉钉（待支持）' },
  { value: 'email', label: '邮件（待支持）' },
]

const eventCategories: Record<string, string> = {
  node: '节点相关',
  request: '请求相关',
  account: '账号相关',
  system: '系统相关',
}

export default function Notifications() {
  const [channels, setChannels] = useState<NotificationChannel[]>([])
  const [channelLoading, setChannelLoading] = useState(false)
  const [channelSaving, setChannelSaving] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [name, setName] = useState('')
  const [channelType, setChannelType] = useState(channelTypes[0]?.value || 'wechat_work')
  const [webhook, setWebhook] = useState('')
  const [enabled, setEnabled] = useState('true')

  const [eventTypes, setEventTypes] = useState<EventType[]>([])
  const [eventLoading, setEventLoading] = useState(false)
  const [subscriptions, setSubscriptions] = useState<NotificationSubscription[]>([])
  const [selectedEvents, setSelectedEvents] = useState<Set<string>>(new Set())
  const [selectedChannelId, setSelectedChannelId] = useState('')
  const [savingSubs, setSavingSubs] = useState(false)

  const [testChannelId, setTestChannelId] = useState('')
  const [testTitle, setTestTitle] = useState('')
  const [testContent, setTestContent] = useState('')
  const [testing, setTesting] = useState(false)

  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const dialog = useDialog()
  const formRef = useRef<HTMLFormElement | null>(null)
  const subscriptionRef = useRef<HTMLDivElement | null>(null)
  const testRef = useRef<HTMLDivElement | null>(null)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const resetForm = () => {
    setEditingId('')
    setName('')
    setChannelType(channelTypes[0]?.value || 'wechat_work')
    setWebhook('')
    setEnabled('true')
  }

  const scrollToForm = () => {
    if (formRef.current) {
      formRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }

  const scrollToSubscription = () => {
    if (subscriptionRef.current) {
      subscriptionRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }

  const scrollToTest = () => {
    if (testRef.current) {
      testRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }

  const loadChannels = async () => {
    setChannelLoading(true)
    try {
      const list = await api.getNotificationChannels()
      setChannels(list)
      setSelectedChannelId((prev) => prev || (list[0]?.id ?? ''))
      setTestChannelId((prev) => prev || (list[0]?.id ?? ''))
    } catch (err) {
      showToast('加载渠道失败', 'error')
    } finally {
      setChannelLoading(false)
    }
  }

  const loadEventTypes = async () => {
    setEventLoading(true)
    try {
      const list = await api.getEventTypes()
      setEventTypes(list)
    } catch (err) {
      showToast('加载事件类型失败', 'error')
    } finally {
      setEventLoading(false)
    }
  }

  const loadSubscriptions = async (channelId: string) => {
    if (!channelId) return
    setEventLoading(true)
    try {
      const list = await api.getNotificationSubscriptions(channelId)
      setSubscriptions(list)
      setSelectedEvents(new Set(list.filter((s) => s.enabled).map((s) => s.event_type)))
    } catch (err) {
      showToast('加载订阅失败', 'error')
    } finally {
      setEventLoading(false)
    }
  }

  useEffect(() => {
    loadChannels()
    loadEventTypes()
  }, [])

  useEffect(() => {
    if (selectedChannelId) {
      loadSubscriptions(selectedChannelId)
    } else {
      setSubscriptions([])
      setSelectedEvents(new Set())
    }
  }, [selectedChannelId])

  const onSubmitChannel = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      showToast('请输入渠道名称', 'error')
      return
    }
    if (!editingId && !webhook.trim()) {
      showToast('Webhook URL 必填', 'error')
      return
    }

    setChannelSaving(true)
    try {
      if (editingId) {
        const payload: Partial<{ name: string; channel_type: string; enabled: boolean; config: { webhook_url?: string } }> = {
          name: name.trim(),
          channel_type: channelType,
          enabled: enabled === 'true',
        }
        if (webhook.trim()) {
          payload.config = { webhook_url: webhook.trim() }
        }
        await api.updateNotificationChannel(editingId, payload)
        showToast('渠道已更新')
      } else {
        await api.createNotificationChannel({
          name: name.trim(),
          channel_type: channelType,
          enabled: enabled === 'true',
          config: { webhook_url: webhook.trim() },
        })
        showToast('渠道已创建')
      }
      resetForm()
      loadChannels()
    } catch (err) {
      showToast((err as Error).message || '保存失败', 'error')
    } finally {
      setChannelSaving(false)
    }
  }

  const handleEditChannel = (ch: NotificationChannel) => {
    setEditingId(ch.id)
    setName(ch.name)
    setChannelType(ch.channel_type)
    setEnabled(ch.enabled ? 'true' : 'false')
    setWebhook('')
    scrollToForm()
  }

  const handleDeleteChannel = async (id: string) => {
    const ok = await dialog.confirm({ title: '确认删除', message: '删除后将停止发送通知，是否继续？' })
    if (!ok) return
    try {
      await api.deleteNotificationChannel(id)
      showToast('渠道已删除')
      if (selectedChannelId === id) {
        setSelectedChannelId('')
        setSubscriptions([])
        setSelectedEvents(new Set())
      }
      loadChannels()
    } catch (err) {
      showToast((err as Error).message || '删除失败', 'error')
    }
  }

  const groupedEventTypes = useMemo(() => {
    const groups: Record<string, EventType[]> = {}
    eventTypes.forEach((et) => {
      const key = et.category || 'system'
      if (!groups[key]) groups[key] = []
      groups[key].push(et)
    })
    return groups
  }, [eventTypes])

  const handleToggleEvent = (eventType: string) => {
    setSelectedEvents((prev) => {
      const next = new Set(prev)
      if (next.has(eventType)) {
        next.delete(eventType)
      } else {
        next.add(eventType)
      }
      return next
    })
  }

  const handleSaveSubscriptions = async () => {
    if (!selectedChannelId) {
      showToast('请选择一个渠道', 'error')
      return
    }
    const enabledSet = new Set(selectedEvents)
    const currentMap = new Map(subscriptions.map((s) => [s.event_type, s]))
    const toCreate = Array.from(enabledSet).filter((et) => !currentMap.has(et))
    const toEnable = subscriptions.filter((s) => enabledSet.has(s.event_type) && !s.enabled)
    const toDisable = subscriptions.filter((s) => !enabledSet.has(s.event_type) && s.enabled)

    setSavingSubs(true)
    try {
      if (toCreate.length) {
        await api.createNotificationSubscriptions({
          channel_id: selectedChannelId,
          event_types: toCreate,
          enabled: true,
        })
      }
      if (toEnable.length) {
        await Promise.all(toEnable.map((s) => api.updateNotificationSubscription(s.id, true)))
      }
      if (toDisable.length) {
        await Promise.all(toDisable.map((s) => api.updateNotificationSubscription(s.id, false)))
      }
      showToast('订阅已保存')
      loadSubscriptions(selectedChannelId)
    } catch (err) {
      showToast((err as Error).message || '保存订阅失败', 'error')
    } finally {
      setSavingSubs(false)
    }
  }

  const handleTestNotification = async () => {
    if (!testChannelId) {
      showToast('请选择渠道', 'error')
      return
    }
    if (!testTitle.trim() || !testContent.trim()) {
      showToast('请输入标题和内容', 'error')
      return
    }
    setTesting(true)
    try {
      await api.testNotification({
        channel_id: testChannelId,
        title: testTitle.trim(),
        content: testContent.trim(),
      })
      showToast('测试通知已发送')
    } catch (err) {
      showToast((err as Error).message || '发送失败', 'error')
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="notifications-page">
      <div className="notifications-header">
        <h1>通知管理</h1>
        <p className="sub">配置通知渠道，选择要订阅的事件并发送测试通知。</p>
      </div>

      <Card title={editingId ? `编辑渠道：${name || '未命名'}` : '创建通知渠道'}>
        <form className="inline-form" onSubmit={onSubmitChannel} ref={formRef} autoComplete="off">
          <input type="hidden" value={editingId} />
          <label>
            渠道名称
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="输入渠道名称"
              required
            />
          </label>
          <label>
            渠道类型
            <select value={channelType} onChange={(e) => setChannelType(e.target.value)}>
              {channelTypes.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Webhook URL {editingId ? <span className="muted">（留空则不修改）</span> : null}
            <input
              value={webhook}
              onChange={(e) => setWebhook(e.target.value)}
              placeholder={editingId ? '留空保持不变' : 'https://example.com/webhook'}
              type="url"
              required={!editingId}
            />
          </label>
          <label>
            启用状态
            <select value={enabled} onChange={(e) => setEnabled(e.target.value)}>
              <option value="true">启用</option>
              <option value="false">停用</option>
            </select>
          </label>
          <div className="form-actions">
            <button className="btn primary" type="submit" disabled={channelSaving}>
              {editingId ? '更新渠道' : '创建渠道'}
            </button>
            <button className="btn ghost" type="button" onClick={resetForm}>
              {editingId ? '取消编辑' : '清空'}
            </button>
          </div>
        </form>
        <small className="muted" style={{ display: 'block', marginTop: 12 }}>
          创建时需填写 Webhook URL，编辑时不显示原值以保护敏感信息。
        </small>
      </Card>

      <Card
        title="渠道列表"
        extra={<small className="muted">编辑、删除或一键跳转到测试/订阅区域。</small>}
      >
        <div className="toolbar">
          <button
            className="btn primary"
            type="button"
            onClick={() => {
              resetForm()
              scrollToForm()
            }}
          >
            ➕ 新建渠道
          </button>
          <div className="spacer" />
          <button className="btn ghost" type="button" onClick={loadChannels}>
            刷新
          </button>
        </div>
        <div className="table-wrapper">
          <table className="channels-table">
            <thead>
              <tr>
                <th style={{ minWidth: 140 }}>ID</th>
                <th>名称</th>
                <th>类型</th>
                <th>状态</th>
                <th style={{ minWidth: 220 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {channelLoading ? (
                <tr>
                  <td colSpan={5}>
                    <div className="skeleton" style={{ height: 24 }}></div>
                  </td>
                </tr>
              ) : channels.length === 0 ? (
                <tr>
                  <td colSpan={5}>暂无渠道</td>
                </tr>
              ) : (
                channels.map((ch) => {
                  const label = channelTypes.find((t) => t.value === ch.channel_type)?.label || ch.channel_type
                  return (
                    <tr key={ch.id} className={selectedChannelId === ch.id ? 'selected' : undefined}>
                      <td>
                        <code className="mono">{ch.id}</code>
                      </td>
                      <td>{ch.name}</td>
                      <td>{label}</td>
                      <td>
                        <span className={`status-badge ${ch.enabled ? 'on' : 'off'}`}>
                          <span className="dot" />
                          {ch.enabled ? '启用' : '停用'}
                        </span>
                      </td>
                      <td>
                        <div className="table-actions">
                          <button className="btn ghost" type="button" onClick={() => handleEditChannel(ch)}>
                            编辑
                          </button>
                          <button className="btn ghost" type="button" onClick={() => {
                            setSelectedChannelId(ch.id)
                            scrollToSubscription()
                          }}>
                            订阅
                          </button>
                          <button
                            className="btn ghost"
                            type="button"
                            onClick={() => {
                              setTestChannelId(ch.id)
                              scrollToTest()
                            }}
                          >
                            测试
                          </button>
                          <button className="btn danger" type="button" onClick={() => handleDeleteChannel(ch.id)}>
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </Card>

      <div className="notifications-grid">
        <Card
          title="事件订阅管理"
          extra={<small className="muted">选择渠道后，勾选需要的事件类型。</small>}
        >
          <div ref={subscriptionRef}>
          <div className="subscription-toolbar">
            <label>
              选择渠道
              <select
                value={selectedChannelId}
                onChange={(e) => setSelectedChannelId(e.target.value)}
              >
                <option value="">请选择</option>
                {channels.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </label>
            <button className="btn ghost" type="button" onClick={() => selectedChannelId && loadSubscriptions(selectedChannelId)}>
              刷新订阅
            </button>
          </div>

          {!selectedChannelId ? (
            <div className="empty-box">先选择一个渠道以管理订阅。</div>
          ) : eventLoading ? (
            <div className="empty-box">加载中...</div>
          ) : (
            <div className="event-groups">
              {Object.entries(eventCategories).map(([key, label]) => {
                const list = groupedEventTypes[key] || []
                return (
                  <div className="event-group" key={key}>
                    <div className="group-head">
                      <div>
                        <h4>{label}</h4>
                        <small className="muted">{list.length ? `共 ${list.length} 项` : '暂无事件'}</small>
                      </div>
                      <span className="pill light">{key}.*</span>
                    </div>
                    {list.length === 0 ? (
                      <div className="empty-box small">暂未提供该分类事件</div>
                    ) : (
                      <div className="event-list">
                        {list.map((evt) => (
                          <label className="event-item" key={evt.type}>
                            <input
                              type="checkbox"
                              checked={selectedEvents.has(evt.type)}
                              onChange={() => handleToggleEvent(evt.type)}
                            />
                            <div>
                              <div className="event-type">{evt.type}</div>
                              <div className="event-desc">{evt.description}</div>
                            </div>
                          </label>
                        ))}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}
          <div className="form-actions" style={{ marginTop: 16 }}>
            <button className="btn primary" type="button" onClick={handleSaveSubscriptions} disabled={!selectedChannelId || savingSubs}>
              保存订阅
            </button>
            <button className="btn ghost" type="button" onClick={() => selectedChannelId && loadSubscriptions(selectedChannelId)}>
              重置
            </button>
          </div>
          </div>
        </Card>

        <Card title="发送测试通知" extra={<small className="muted">用于验证渠道配置是否可用。</small>}>
          <div className="test-card" ref={testRef}>
            <label>
              选择渠道
              <select value={testChannelId} onChange={(e) => setTestChannelId(e.target.value)}>
                <option value="">请选择</option>
                {channels.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              标题
              <input
                value={testTitle}
                onChange={(e) => setTestTitle(e.target.value)}
                placeholder="测试通知标题"
              />
            </label>
            <label>
              内容
              <textarea
                value={testContent}
                onChange={(e) => setTestContent(e.target.value)}
                placeholder="测试通知内容"
                rows={4}
              />
            </label>
            <div className="form-actions">
              <button className="btn primary" type="button" onClick={handleTestNotification} disabled={testing}>
                发送测试
              </button>
              <button className="btn ghost" type="button" onClick={() => { setTestTitle(''); setTestContent('') }}>
                清空
              </button>
            </div>
          </div>
        </Card>
      </div>

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
