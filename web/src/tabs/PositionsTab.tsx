import type { PortfolioData } from '../types/portfolio'
import { fmt, fmtCurrency, fmtPct, fmtK, clr } from '../utils/format'
import { BROKER_COLORS } from '../constants'
import { HorizBar } from '../components/charts/HorizBar'
import { StatCard } from '../components/ui/StatCard'
import { SectionLabel } from '../components/ui/SectionLabel'
import { BrokerPill } from '../components/ui/BrokerPill'

interface Props {
  data: PortfolioData
  accent: string
}

export function PositionsTab({ data, accent }: Props) {
  const totalMV = data.openPositions.reduce((s, p) => s + (p.mv ?? 0), 0)
  const totalUPnl = data.openPositions.reduce((s, p) => s + (p.pnl ?? 0), 0)

  const best = data.openPositions.reduce(
    (best, p) => ((p.pct ?? -Infinity) > (best.pct ?? -Infinity) ? p : best),
    data.openPositions[0]
  )

  const sorted = data.openPositions.slice().sort((a, b) => (b.mv ?? 0) - (a.mv ?? 0))

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10 }}>
        <StatCard label="Portfolio Value" value={fmtCurrency(totalMV)} sub={`${data.openPositions.length} positions`} />
        <StatCard label="Unrealized P&L" value={fmtCurrency(totalUPnl)} sub={fmtPct(totalMV > 0 ? (totalUPnl / totalMV) * 100 : 0)} valueColor={clr(totalUPnl)} />
        <StatCard
          label="Best Performer"
          value={best?.symbol ?? '—'}
          sub={best ? `${fmtPct(best.pct)} unrealized` : ''}
          valueColor="#34d399"
        />
      </div>

      {/* All positions table */}
      <div style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid #1e293b' }}>
          <SectionLabel>Open Positions</SectionLabel>
        </div>
        <div style={{ overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 520 }}>
            <thead>
              <tr style={{ background: '#0f172a' }}>
                {['Symbol', 'Market Value', 'Cost Basis', 'Unrealized P&L', 'Return', 'Allocation'].map(h => (
                  <th key={h} style={{ padding: '8px 14px', textAlign: h === 'Symbol' ? 'left' : 'right', fontSize: 10, fontWeight: 600, color: '#475569', letterSpacing: '0.08em', textTransform: 'uppercase', whiteSpace: 'nowrap' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sorted.map(p => (
                <tr
                  key={p.symbol}
                  style={{ borderTop: '1px solid #1e293b', transition: 'background 0.1s' }}
                  onMouseEnter={e => (e.currentTarget.style.background = '#1e293b44')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <td style={{ padding: '10px 14px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div style={{ width: 26, height: 26, borderRadius: 6, background: accent + '22', border: `1px solid ${accent}33`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 9, fontWeight: 700, color: accent, flexShrink: 0 }}>
                        {p.symbol.slice(0, 3)}
                      </div>
                      <span style={{ fontWeight: 600, fontSize: 13 }}>{p.symbol}</span>
                    </div>
                  </td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontWeight: 600, fontFamily: "'DM Mono', monospace", fontSize: 13 }}>{fmtCurrency(p.mv)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', color: '#64748b', fontFamily: "'DM Mono', monospace", fontSize: 12 }}>{fmtCurrency(p.cost)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontWeight: 600, fontFamily: "'DM Mono', monospace", fontSize: 13, color: clr(p.pnl) }}>{fmtCurrency(p.pnl)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                    <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: clr(p.pct) + '22', color: clr(p.pct) }}>{fmtPct(p.pct)}</span>
                  </td>
                  <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 3, alignItems: 'flex-end' }}>
                      <span style={{ fontSize: 11, color: '#94a3b8' }}>{fmt(totalMV > 0 ? ((p.mv ?? 0) / totalMV) * 100 : 0, 1)}%</span>
                      <div style={{ width: 60 }}>
                        <HorizBar value={p.mv ?? 0} total={totalMV} color={accent} height={3} />
                      </div>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Per-broker position cards */}
      <div className="grid-2">
        {data.brokers.filter(b => b.positions.some(p => p.mv != null)).map(b => {
          const bc = BROKER_COLORS[b.name] ?? '#94a3b8'
          return (
            <div key={b.name} style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
              <div style={{ padding: '12px 16px', borderBottom: '1px solid #1e293b', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <BrokerPill name={b.name} />
                <div style={{ fontSize: 11, color: '#475569' }}>{b.currency}</div>
              </div>
              <div style={{ padding: '8px 0' }}>
                {b.positions.filter(p => p.mv != null).map(p => (
                  <div key={p.symbol} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '7px 16px' }}>
                    <div style={{ width: 24, height: 24, borderRadius: 5, flexShrink: 0, background: bc + '22', border: `1px solid ${bc}33`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 8, fontWeight: 700, color: bc }}>
                      {p.symbol.slice(0, 3)}
                    </div>
                    <span style={{ flex: 1, fontWeight: 500, fontSize: 12 }}>{p.symbol}</span>
                    <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, color: '#94a3b8' }}>{fmtK(p.mv)}</span>
                    <span style={{ padding: '1px 6px', borderRadius: 3, fontSize: 10, fontWeight: 600, background: clr(p.pnl) + '22', color: clr(p.pnl), minWidth: 52, textAlign: 'right' }}>
                      {fmtPct(p.pct)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
