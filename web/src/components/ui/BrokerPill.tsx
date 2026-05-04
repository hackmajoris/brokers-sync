import { BROKER_COLORS, BROKER_LABELS } from '../../constants'

interface BrokerPillProps {
  name: string
}

export function BrokerPill({ name }: BrokerPillProps) {
  const color = BROKER_COLORS[name] ?? '#94a3b8'
  const label = BROKER_LABELS[name] ?? name
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 5,
        padding: '2px 8px',
        borderRadius: 999,
        background: color + '22',
        border: `1px solid ${color}44`,
        fontSize: 10,
        fontWeight: 600,
        color,
      }}
    >
      <span style={{ width: 5, height: 5, borderRadius: '50%', background: color }} />
      {label}
    </span>
  )
}
