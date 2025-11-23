import { useEffect, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import api from '../services/api'
import type { TunnelState } from '../types'
import './TunnelSettings.css'

export default function TunnelSettings() {
	const [config, setConfig] = useState<TunnelState | null>(null)
	const [apiToken, setApiToken] = useState('')
	const [subdomain, setSubdomain] = useState('')
	const [zone, setZone] = useState('')
	const [enabled, setEnabled] = useState(false)
	const [zones, setZones] = useState<string[]>([])
	const [loading, setLoading] = useState(false)
	const [saving, setSaving] = useState(false)
	const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

	const showToast = (message: string, type: 'success' | 'error' = 'success') => {
		setToast({ message, type })
		setTimeout(() => setToast(null), 2200)
	}

	const syncForm = (data: TunnelState) => {
		setConfig(data)
		setSubdomain(data.subdomain || '')
		setZone(data.zone || '')
		setEnabled(!!data.enabled)
		setApiToken('')
	}

	const loadConfig = async () => {
		setLoading(true)
		try {
			const res = await api.getTunnel()
			syncForm(res)
		} catch (err) {
			showToast((err as Error).message || '加载失败', 'error')
		} finally {
			setLoading(false)
		}
	}

	useEffect(() => {
		loadConfig()
	}, [])

	const handleSave = async () => {
		if (!subdomain.trim()) {
			showToast('子域名前缀必填', 'error')
			return
		}
		setSaving(true)
		try {
			const payload: Record<string, string | boolean> = {
				subdomain: subdomain.trim(),
				enabled,
				zone: zone.trim(),
			}
			if (apiToken.trim()) {
				payload.api_token = apiToken.trim()
			}
			const res = await api.saveTunnel(payload)
			syncForm(res)
			showToast('已保存')
		} catch (err) {
			showToast((err as Error).message || '保存失败', 'error')
		} finally {
			setSaving(false)
			setApiToken('')
		}
	}

	const handleStart = async () => {
		setSaving(true)
		try {
			const res = await api.startTunnel()
			syncForm(res)
			showToast('隧道已启动')
		} catch (err) {
			showToast((err as Error).message || '启动失败', 'error')
		} finally {
			setSaving(false)
		}
	}

	const handleStop = async () => {
		setSaving(true)
		try {
			const res = await api.stopTunnel()
			syncForm(res)
			showToast('隧道已停止')
		} catch (err) {
			showToast((err as Error).message || '停止失败', 'error')
		} finally {
			setSaving(false)
		}
	}

	const fetchZones = async () => {
		if (!config?.api_token_set && !apiToken.trim()) {
			showToast('请先保存 API Token', 'error')
			return
		}
		try {
			const list = await api.listZones()
			setZones(list)
			showToast('域名列表已刷新')
		} catch (err) {
			showToast((err as Error).message || '获取域名失败', 'error')
		}
	}

	const status = config?.status || 'stopped'
	const statusLabel = status === 'running' ? '运行中' : status === 'error' ? '错误' : '已停止'
	const statusClass = status === 'running' ? 'ok' : status === 'error' ? 'error' : 'muted'

	return (
		<div className="tunnel-page">
			<div className="tunnel-header">
				<h1>隧道设置</h1>
				<p className="sub">配置 Cloudflare Tunnel，动态暴露代理服务。</p>
			</div>

			<Card>
				<div className="tunnel-status-line">
					<span className={`status-badge ${statusClass}`}>{statusLabel}</span>
					{config?.public_url && (
						<a className="public-url" href={config.public_url} target="_blank" rel="noreferrer">
							{config.public_url}
						</a>
					)}
					{config?.last_error && <span className="last-error">最后错误：{config.last_error}</span>}
					<div className="spacer" />
					<button className="btn ghost" type="button" onClick={loadConfig} disabled={loading || saving}>
						刷新
					</button>
					<button
						className="btn primary"
						type="button"
						onClick={status === 'running' ? handleStop : handleStart}
						disabled={saving || loading}
					>
						{status === 'running' ? '停止隧道' : '启动隧道'}
					</button>
				</div>

				<div className="tunnel-grid">
					<label>
						Cloudflare API Token
						<input
							type="password"
							value={apiToken}
							onChange={(e) => setApiToken(e.target.value)}
							placeholder={config?.api_token_set ? '已设置，留空不变' : 'cf token'}
						/>
						<small className="hint">Token 仅用于 Cloudflare API 调用，存储时会加密。</small>
					</label>

					<label>
						子域名前缀
						<input
							type="text"
							value={subdomain}
							onChange={(e) => setSubdomain(e.target.value)}
							placeholder="例如 myproxy"
							required
						/>
					</label>

					<label>
						域名
						<div className="zone-field">
							<input
								type="text"
								list="zone-options"
								value={zone}
								onChange={(e) => setZone(e.target.value)}
								placeholder="留空自动选择账户下首个域名"
							/>
							<datalist id="zone-options">
								{zones.map((z) => (
									<option key={z} value={z} />
								))}
							</datalist>
							<button className="btn ghost" type="button" onClick={fetchZones} disabled={saving}>
								获取域名
							</button>
						</div>
						<small className="hint">从 Cloudflare 帐号中选择根域名，子域名前缀将自动拼接。</small>
					</label>

					<label className="toggle-line">
						<input
							type="checkbox"
							checked={enabled}
							onChange={(e) => setEnabled(e.target.checked)}
						/>
						<span>启用隧道（保存配置后，可点击启动）</span>
					</label>
				</div>

				<div className="tunnel-actions">
					<button className="btn primary" type="button" onClick={handleSave} disabled={saving}>
						保存配置
					</button>
					<button className="btn ghost" type="button" onClick={() => setApiToken('')} disabled={saving}>
						清空 Token 输入
					</button>
				</div>
			</Card>
			<Toast message={toast?.message} type={toast?.type} />
		</div>
	)
}
