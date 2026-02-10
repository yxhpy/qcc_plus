import { useModelRecovery } from '../contexts/ModelRecoveryContext'
import Tooltip from './Tooltip'

interface RecoveryBadgeProps {
  nodeId: string
}

export default function RecoveryBadge({ nodeId }: RecoveryBadgeProps) {
  const { byNode } = useModelRecovery()
  const info = byNode[nodeId]
  if (!info || info.count === 0) return null

  const tooltipContent = (
    <div style={{ fontSize: 12, lineHeight: 1.6 }}>
      <div style={{ fontWeight: 600, marginBottom: 2 }}>恢复中模型 ({info.count})</div>
      {info.models.map((m, i) => (
        <div key={i} style={{ color: 'var(--color-danger-text, #B91C1C)' }}>{m}</div>
      ))}
    </div>
  )

  return (
    <Tooltip content={tooltipContent} trigger="both" maxWidth="280px">
      <span className="recovery-badge" title="">
        {info.count}
      </span>
    </Tooltip>
  )
}
