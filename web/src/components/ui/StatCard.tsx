interface StatCardProps {
  label: string
  value: string
  sub?: string
  valueColor?: string
}

export function StatCard({ label, value, sub, valueColor }: StatCardProps) {
  return (
    <div
      style={{
        background: '#0f0f0f',
        borderRadius: 10,
        padding: '14px 16px',
        border: '1px solid #1a1a1a',
        display: 'flex',
        flexDirection: 'column',
        gap: 4,
      }}
    >
      <div
        style={{
          fontSize: 10,
          fontWeight: 500,
          letterSpacing: '0.08em',
          color: '#555555',
          textTransform: 'uppercase',
        }}
      >
        {label}
      </div>
      <div
        style={{
          fontSize: 22,
          fontWeight: 700,
          color: valueColor ?? '#ffffff',
          lineHeight: 1.1,
        }}
      >
        {value}
      </div>
      {sub && <div style={{ fontSize: 11, color: '#888888' }}>{sub}</div>}
    </div>
  )
}
