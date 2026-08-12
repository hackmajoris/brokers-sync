import { useState } from 'react'
import type { Candle } from '../../services/portfolioService'
import { fmt } from '../../utils/format'

interface Props {
  candles: Candle[]
  ma?: (number | null)[]
}

const UP = '#34d399'
const DOWN = '#f87171'
const MA_COLOR = '#60a5fa'

const W = 900
const H = 260
const PAD_L = 8
const PAD_R = 52
const PAD_T = 10
const PAD_B = 22

// Candlestick renders OHLC bars in a responsive SVG (no charting dependency).
export function Candlestick({ candles, ma }: Props) {
  const [hover, setHover] = useState<number | null>(null)

  if (candles.length === 0) {
    return <div style={{ color: '#555555', fontSize: 13, padding: '30px 0', textAlign: 'center' }}>No history available</div>
  }

  const plotW = W - PAD_L - PAD_R
  const plotH = H - PAD_T - PAD_B

  let lo = Infinity
  let hi = -Infinity
  for (const c of candles) {
    if (c.l < lo) lo = c.l
    if (c.h > hi) hi = c.h
  }
  if (ma) {
    for (const m of ma) {
      if (m == null) continue
      if (m < lo) lo = m
      if (m > hi) hi = m
    }
  }
  if (hi <= lo) hi = lo + 1
  const pad = (hi - lo) * 0.05
  lo -= pad
  hi += pad

  const n = candles.length
  const step = plotW / n
  const bodyW = Math.max(1, Math.min(step * 0.7, 14))
  const x = (i: number) => PAD_L + step * (i + 0.5)
  const y = (p: number) => PAD_T + (1 - (p - lo) / (hi - lo)) * plotH

  const priceTicks = 4
  const yLabels = Array.from({ length: priceTicks + 1 }, (_, k) => lo + ((hi - lo) * k) / priceTicks)

  const dateTicks = Math.min(5, n)
  const xLabels = Array.from({ length: dateTicks }, (_, k) => {
    const i = Math.round((k * (n - 1)) / Math.max(1, dateTicks - 1))
    return { i, label: new Date(candles[i].t * 1000).toLocaleDateString('en-US', { month: 'short', year: '2-digit' }) }
  })

  function onMove(e: React.MouseEvent<SVGSVGElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    const vbX = ((e.clientX - rect.left) / rect.width) * W
    const i = Math.max(0, Math.min(n - 1, Math.floor((vbX - PAD_L) / step)))
    setHover(i)
  }

  const hc = hover != null ? candles[hover] : null
  const hMa = hover != null && ma ? ma[hover] : null
  const leftPct = hover != null ? (x(hover) / W) * 100 : 0
  const flip = leftPct > 55

  return (
    <div style={{ position: 'relative' }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        style={{ width: '100%', height: 'auto', display: 'block' }}
        preserveAspectRatio="none"
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}
      >
        {yLabels.map((p, k) => (
          <g key={k}>
            <line x1={PAD_L} x2={W - PAD_R} y1={y(p)} y2={y(p)} stroke="#1a1a1a" strokeWidth={1} />
            <text x={W - PAD_R + 5} y={y(p) + 3} fill="#555555" fontSize={10} fontFamily="'DM Mono', monospace">{fmt(p, p >= 100 ? 0 : 1)}</text>
          </g>
        ))}
        {xLabels.map((t, k) => (
          <text key={k} x={x(t.i)} y={H - 6} fill="#555555" fontSize={10} textAnchor="middle" fontFamily="'DM Mono', monospace">{t.label}</text>
        ))}
        {candles.map((c, i) => {
          const up = c.c >= c.o
          const color = up ? UP : DOWN
          const cx = x(i)
          const yO = y(c.o)
          const yC = y(c.c)
          const bodyTop = Math.min(yO, yC)
          const bodyH = Math.max(1, Math.abs(yC - yO))
          return (
            <g key={i}>
              <line x1={cx} x2={cx} y1={y(c.h)} y2={y(c.l)} stroke={color} strokeWidth={1} />
              <rect x={cx - bodyW / 2} y={bodyTop} width={bodyW} height={bodyH} fill={color} />
            </g>
          )
        })}
        {ma && (() => {
          const pts = ma.map((m, i) => (m == null ? null : `${x(i)},${y(m)}`)).filter(Boolean).join(' ')
          if (!pts) return null
          return (
            <>
              <polyline points={pts} fill="none" stroke={MA_COLOR} strokeWidth={1.5} strokeLinejoin="round" />
              <g>
                <line x1={PAD_L} x2={PAD_L + 18} y1={PAD_T + 4} y2={PAD_T + 4} stroke={MA_COLOR} strokeWidth={1.5} />
                <text x={PAD_L + 23} y={PAD_T + 7} fill={MA_COLOR} fontSize={10} fontFamily="'DM Mono', monospace">21W MA</text>
              </g>
            </>
          )
        })()}
        {hc && (
          <line x1={x(hover!)} x2={x(hover!)} y1={PAD_T} y2={H - PAD_B} stroke="#555555" strokeWidth={1} strokeDasharray="3 3" pointerEvents="none" />
        )}
      </svg>

      {hc && (
        <div
          style={{
            position: 'absolute',
            top: 6,
            left: `${leftPct}%`,
            transform: flip ? 'translateX(calc(-100% - 10px))' : 'translateX(10px)',
            pointerEvents: 'none',
            background: '#0a0a0aee',
            border: '1px solid #2a2a2a',
            borderRadius: 6,
            padding: '6px 8px',
            fontSize: 11,
            fontFamily: "'DM Mono', monospace",
            whiteSpace: 'nowrap',
            color: '#c0c0c0',
          }}
        >
          <div style={{ color: '#888888', marginBottom: 3 }}>{new Date(hc.t * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'auto auto', gap: '1px 10px' }}>
            <span style={{ color: '#555555' }}>O</span><span>{fmt(hc.o)}</span>
            <span style={{ color: '#555555' }}>H</span><span>{fmt(hc.h)}</span>
            <span style={{ color: '#555555' }}>L</span><span>{fmt(hc.l)}</span>
            <span style={{ color: '#555555' }}>C</span><span style={{ color: hc.c >= hc.o ? UP : DOWN }}>{fmt(hc.c)}</span>
            {hMa != null && (<><span style={{ color: MA_COLOR }}>MA</span><span style={{ color: MA_COLOR }}>{fmt(hMa)}</span></>)}
          </div>
        </div>
      )}
    </div>
  )
}
