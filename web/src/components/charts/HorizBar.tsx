interface HorizBarProps {
  value: number
  total: number
  color: string
  height?: number
  bg?: string
}

export function HorizBar({ value, total, color, height = 6, bg = '#1a1a1a' }: HorizBarProps) {
  const pct = total === 0 ? 0 : Math.min((Math.abs(value) / Math.abs(total)) * 100, 100)
  return (
    <div style={{ background: bg, borderRadius: 999, height, overflow: 'hidden', width: '100%' }}>
      <div
        style={{
          background: color,
          width: `${pct}%`,
          height: '100%',
          borderRadius: 999,
          transition: 'width 0.5s ease',
        }}
      />
    </div>
  )
}
