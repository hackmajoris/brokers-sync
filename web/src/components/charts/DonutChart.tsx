interface Slice {
  value: number
  color: string
}

interface DonutChartProps {
  slices: Slice[]
  size?: number
}

export function DonutChart({ slices, size = 96 }: DonutChartProps) {
  const r = 36, cx = 48, cy = 48, strokeW = 14
  const circ = 2 * Math.PI * r
  const total = slices.reduce((s, d) => s + d.value, 0)

  let offset = 0
  const segs = slices.map(d => {
    const dash = (d.value / total) * circ
    const seg = { ...d, dash, offset, gap: circ - dash }
    offset += dash
    return seg
  })

  return (
    <svg viewBox="0 0 96 96" width={size} height={size}>
      {segs.map((s, i) => (
        <circle
          key={i}
          cx={cx} cy={cy} r={r}
          fill="none"
          stroke={s.color}
          strokeWidth={strokeW}
          strokeDasharray={`${s.dash} ${s.gap}`}
          strokeDashoffset={-s.offset + circ * 0.25}
          style={{ transform: 'rotate(-90deg)', transformOrigin: '50% 50%' }}
        />
      ))}
      <circle cx={cx} cy={cy} r={r - strokeW / 2 - 1} fill="#080808" />
    </svg>
  )
}
