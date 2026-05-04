interface BarChartProps {
  data: Array<Record<string, number | string>>
  keyX: string
  keyY: string
  colorFn?: (v: number) => string
  height?: number
}

export function BarChart({ data, keyX, keyY, colorFn, height = 120 }: BarChartProps) {
  const values = data.map(d => d[keyY] as number)
  const max = Math.max(...values.map(Math.abs))
  const hasNeg = values.some(v => v < 0)
  const n = data.length
  const slotW = 100 / n
  const barW = slotW * 0.35

  return (
    <div style={{ width: '100%', position: 'relative' }}>
      <svg
        viewBox={`0 0 100 ${height}`}
        preserveAspectRatio="none"
        style={{ width: '100%', height, display: 'block' }}
      >
        {hasNeg && (
          <line x1="0" y1={height / 2} x2="100" y2={height / 2} stroke="#334155" strokeWidth="0.4" />
        )}
        {data.map((d, i) => {
          const v = d[keyY] as number
          const barH = (Math.abs(v) / max) * (height * (hasNeg ? 0.46 : 0.84))
          const x = i * slotW + slotW * 0.325
          const y = v >= 0
            ? hasNeg ? height / 2 - barH : height - barH - height * 0.08
            : height / 2
          return (
            <rect
              key={i}
              x={x} y={y}
              width={barW} height={Math.max(barH, 0.5)}
              fill={colorFn ? colorFn(v) : '#818cf8'}
              rx="0.6" opacity="0.9"
            />
          )
        })}
      </svg>
      <div style={{ display: 'flex', width: '100%', marginTop: 4 }}>
        {data.map((d, i) => (
          <div
            key={i}
            style={{
              flex: 1, textAlign: 'center', fontSize: 10,
              color: '#94a3b8', fontWeight: 500,
              fontFamily: "'DM Sans', sans-serif",
            }}
          >
            {String(d[keyX])}
          </div>
        ))}
      </div>
    </div>
  )
}
