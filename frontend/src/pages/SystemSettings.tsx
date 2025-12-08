import { useState, useEffect } from 'react'
import Card from '../components/Card'
import api from '../services/api'
import type { EnvVarCategory, EnvVarDefinition } from '../services/api'
import './SystemSettings.css'

export default function SystemSettings() {
  const [categories, setCategories] = useState<EnvVarCategory[]>([])
  const [envvars, setEnvvars] = useState<EnvVarDefinition[]>([])
  const [activeCategory, setActiveCategory] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [searchTerm, setSearchTerm] = useState('')

  // 加载分类
  useEffect(() => {
    const loadCategories = async () => {
      try {
        const cats = await api.getEnvVarCategories()
        setCategories(cats)
        if (cats.length > 0) {
          setActiveCategory(cats[0].key)
        }
      } catch (err) {
        console.error('Failed to load categories:', err)
      }
    }
    loadCategories()
  }, [])

  // 加载环境变量（仅在有效分类时触发，避免初次渲染重复请求）
  useEffect(() => {
    if (!activeCategory) return
    const loadEnvVars = async () => {
      setLoading(true)
      try {
        const vars = await api.getEnvVars(activeCategory)
        setEnvvars(vars)
      } catch (err) {
        console.error('Failed to load env vars:', err)
      } finally {
        setLoading(false)
      }
    }
    loadEnvVars()
  }, [activeCategory])

  // 搜索过滤
  const filteredEnvvars = envvars.filter(v => {
    if (!searchTerm) return true
    const term = searchTerm.toLowerCase()
    return (
      v.name.toLowerCase().includes(term) ||
      v.description.toLowerCase().includes(term) ||
      v.current_value.toLowerCase().includes(term)
    )
  })

  const currentCategoryInfo = categories.find(c => c.key === activeCategory)

  return (
    <div className="system-settings-page">
      <div className="system-settings-header">
        <h1>环境变量配置</h1>
        <p className="sub">查看当前系统的环境变量配置。这些值在服务启动时读取，修改需重启服务生效。</p>
      </div>

      <Card className="settings-card tabs-card">
        <div className="settings-toolbar">
          <div className="tab-group">
            {categories.map(cat => (
              <button
                key={cat.key}
                type="button"
                className={`tab-btn ${activeCategory === cat.key ? 'active' : ''}`}
                onClick={() => setActiveCategory(cat.key)}
                title={cat.description}
              >
                {cat.label}
              </button>
            ))}
          </div>
          <div className="spacer" />
          <div className="search-box">
            <input
              type="text"
              placeholder="搜索变量名或说明..."
              value={searchTerm}
              onChange={e => setSearchTerm(e.target.value)}
            />
            {searchTerm && (
              <button className="clear-btn" onClick={() => setSearchTerm('')}>
                &times;
              </button>
            )}
          </div>
        </div>
      </Card>

      {currentCategoryInfo && (
        <div className="category-description">
          {currentCategoryInfo.description}
        </div>
      )}

      <Card className="settings-card">
        <div className="envvar-table-container">
          {loading ? (
            <div className="settings-loading">加载中...</div>
          ) : filteredEnvvars.length === 0 ? (
            <div className="no-settings">
              {searchTerm ? '没有匹配的环境变量' : '该分类暂无环境变量'}
            </div>
          ) : (
            <table className="envvar-table">
              <thead>
                <tr>
                  <th className="col-name">变量名</th>
                  <th className="col-value">当前值</th>
                  <th className="col-default">默认值</th>
                  <th className="col-desc">说明</th>
                </tr>
              </thead>
              <tbody>
                {filteredEnvvars.map(v => (
                  <tr key={v.name} className={v.is_set ? 'is-set' : ''}>
                    <td className="col-name">
                      <code className="envvar-name">{v.name}</code>
                      {v.is_set && <span className="set-badge">已设置</span>}
                    </td>
                    <td className="col-value">
                      {v.is_secret ? (
                        <span className="secret-value">{v.current_value || '(未设置)'}</span>
                      ) : (
                        <code className={`envvar-value ${!v.current_value ? 'empty' : ''}`}>
                          {v.current_value || '(空)'}
                        </code>
                      )}
                    </td>
                    <td className="col-default">
                      <code className={`default-value ${!v.default_value ? 'empty' : ''}`}>
                        {v.default_value || '(无)'}
                      </code>
                    </td>
                    <td className="col-desc">
                      <span className="envvar-desc">{v.description}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      <Card className="settings-card info-card">
        <div className="info-section">
          <h3>使用说明</h3>
          <ul>
            <li><strong>修改环境变量</strong>：编辑 <code>.env</code> 文件或在 Docker Compose 中设置，然后重启服务</li>
            <li><strong>敏感值</strong>：API Key 等敏感信息会脱敏显示（仅显示首尾各 4 位）</li>
            <li><strong>已设置标记</strong>：表示该变量已在环境中显式设置，非使用默认值</li>
            <li><strong>优先级</strong>：环境变量 &gt; 配置文件 &gt; 默认值</li>
          </ul>
        </div>
      </Card>
    </div>
  )
}
