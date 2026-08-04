import { fmtCurrency } from '../../utils/format'

interface RangeGaugeProps {
  low: number
  high: number
  current: number
  width?: number
}

export function RangeGauge({ low, high, current, width = 90 }: RangeGaugeProps) {
  const pct = high > low ? Math.min(Math.max((current - low) / (high - low), 0), 1) : 0.5
  return (
    <div
      title={`Low ${fmtCurrency(low)} · Current ${fmtCurrency(current)} · High ${fmtCurrency(high)}`}
      style={{ display: 'flex', alignItems: 'center', gap: 5 }}
    >
      <span style={{ fontSize: 10, fontWeight: 700, color: '#888888' }}>L</span>
      <div style={{ position: 'relative', width, height: 10, borderRadius: 999 }}>
        <div
          style={{
            position: 'absolute',
            inset: 0,
            borderRadius: 999,
            background: 'linear-gradient(90deg, #5eead4, #fde68a, #fb923c, #f87171)',
          }}
        />
        <div
          style={{
            position: 'absolute',
            left: `${pct * 100}%`,
            top: -3,
            width: 3,
            height: 16,
            background: '#f5f5f5',
            borderRadius: 1,
            transform: 'translateX(-50%)',
          }}
        />
      </div>
      <span style={{ fontSize: 10, fontWeight: 700, color: '#888888' }}>H</span>
    </div>
  )
}
