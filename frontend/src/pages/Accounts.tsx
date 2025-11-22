import type { FormEvent } from 'react'
import { useEffect, useRef, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import useDialog from '../hooks/useDialog'
import api from '../services/api'
import type { Account } from '../types'

export default function Accounts() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [editingId, setEditingId] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [key, setKey] = useState('')
  const [isAdmin, setIsAdmin] = useState('false')
  const [loading, setLoading] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const dialog = useDialog()
  const formRef = useRef<HTMLFormElement | null>(null)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const resetForm = () => {
    setEditingId('')
    setName('')
    setPassword('')
    setKey('')
    setIsAdmin('false')
  }

  const scrollToForm = () => {
    if (formRef.current) {
      formRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }

  const loadAccounts = async () => {
    setLoading(true)
    try {
      const list = await api.getAccounts()
      setAccounts(list)
    } catch (err) {
      showToast('加载账号失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAccounts()
  }, [])

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !key.trim()) {
      showToast('名称和 Proxy API Key 必填', 'error')
      return
    }
    if (!editingId && password.trim().length < 6) {
      showToast('新账号密码至少 6 位', 'error')
      return
    }
    if (editingId && password && password.trim().length > 0 && password.trim().length < 6) {
      showToast('密码至少 6 位', 'error')
      return
    }
    try {
      if (editingId) {
        await api.updateAccount(editingId, {
          name: name.trim(),
          password: password.trim() || undefined,
          proxy_api_key: key.trim(),
          is_admin: isAdmin === 'true',
        })
        showToast('账号已更新')
      } else {
        await api.createAccount({
          name: name.trim(),
          password: password.trim(),
          proxy_api_key: key.trim(),
          is_admin: isAdmin === 'true',
        })
        showToast('账号已创建')
      }
      resetForm()
      loadAccounts()
    } catch (err) {
      showToast((err as Error).message || '保存失败', 'error')
    }
  }

  const handleEdit = (acc: Account) => {
    setEditingId(acc.id)
    setName(acc.name)
    setPassword('')
    setKey(acc.proxy_api_key)
    setIsAdmin(acc.is_admin ? 'true' : 'false')
    scrollToForm()
  }

  const handleDelete = async (id: string) => {
    const ok = await dialog.confirm({ title: '确认删除', message: '确定删除该账号？' })
    if (!ok) return
    try {
      await api.deleteAccount(id)
      showToast('账号已删除')
      loadAccounts()
    } catch (err) {
      showToast((err as Error).message || '删除失败', 'error')
    }
  }

  return (
    <div>
      <h1>账号管理</h1>
      <p className="sub">仅管理员可见：管理租户账号、后台登录密码与代理密钥。</p>

      <Card title={editingId ? `编辑账号: ${name || '未命名'}` : '新增账号'}>
        <form className="inline" onSubmit={onSubmit} autoComplete="off" ref={formRef}>
          <input type="hidden" value={editingId} />
          <label>
            账号名称
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="输入账号名称"
              autoComplete="username"
              required
            />
          </label>
          <label>
            密码
            <input
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              type="password"
              placeholder="设置密码（6位及以上）"
              autoComplete="new-password"
            />
          </label>
          <label>
            Proxy API Key
            <input
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="用于代理调用的 x-api-key"
              required
              autoComplete="off"
            />
          </label>
          <label>
            管理员权限
            <select value={isAdmin} onChange={(e) => setIsAdmin(e.target.value)}>
              <option value="false">普通账号</option>
              <option value="true">管理员</option>
            </select>
          </label>
          <div className="form-actions">
            <button className="btn primary" type="submit" disabled={loading}>
              {editingId ? '更新账号' : '创建账号'}
            </button>
            <button className="btn ghost" type="button" onClick={resetForm}>
              {editingId ? '取消编辑' : '清空'}
            </button>
          </div>
        </form>
        <small className="muted">
          {editingId ? `正在编辑账号 ${name || '当前账号'}，密码留空则不修改` : '新账号请设置 6 位及以上密码'}
        </small>
      </Card>

      <Card
        title="账号列表"
        extra={<small className="muted">账号密码用于后台登录，Proxy API Key 仍用于 API 调用。</small>}
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
            ➕ 新增账号
          </button>
        </div>
        <table>
          <thead>
            <tr>
              <th>名称</th>
              <th>Proxy API Key</th>
              <th>角色</th>
              <th style={{ minWidth: 160 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={4}>加载中...</td>
              </tr>
            ) : accounts.length === 0 ? (
              <tr>
                <td colSpan={4}>暂无账号</td>
              </tr>
            ) : (
              accounts.map((acc) => {
                const isEditing = editingId === acc.id
                const displayKey = acc.proxy_api_key && acc.proxy_api_key.length > 20
                  ? `${acc.proxy_api_key.slice(0, 8)}...${acc.proxy_api_key.slice(-8)}`
                  : acc.proxy_api_key || '-'

                const copyKey = async () => {
                  try {
                    await navigator.clipboard.writeText(acc.proxy_api_key)
                    showToast('已复制 Proxy API Key')
                  } catch (err) {
                    showToast('复制失败，请手动复制', 'error')
                  }
                }

                return (
                  <tr
                    key={acc.id}
                    className={isEditing ? 'editing' : undefined}
                    style={isEditing ? { backgroundColor: '#fff7e6' } : undefined}
                  >
                    <td>{acc.name}</td>
                    <td>
                      <code>{displayKey}</code>
                      {acc.proxy_api_key ? (
                        <button className="btn ghost" type="button" onClick={copyKey} style={{ marginLeft: 8 }}>
                          复制
                        </button>
                      ) : null}
                    </td>
                    <td>{acc.is_admin ? '管理员' : '普通'}</td>
                    <td>
                      <div className="table-actions">
                        <button className="btn ghost" type="button" onClick={() => handleEdit(acc)}>
                          编辑
                        </button>
                        <button className="btn danger" type="button" onClick={() => handleDelete(acc.id)}>
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
      </Card>
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
