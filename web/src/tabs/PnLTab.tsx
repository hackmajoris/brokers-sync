import type { PortfolioData } from '../types/portfolio'
import { fmtCurrency, fmtPct, fmtK, clr } from '../utils/format'
import { HorizBar } from '../components/charts/HorizBar'
import { StatCard } from '../components/ui/StatCard'
import { SectionLabel } from '../components/ui/SectionLabel'

interface Props {
  data: PortfolioData
}

export function PnLTab({ data }: Props) {
  const gainers = data.topRPnl.filter(p => p.pnl > 0)
  const losers = data.topRPnl.filter(p => p.pnl < 0).sort((a, b) => a.pnl - b.pnl)
  const maxGain = gainers[0]?.pnl ?? 1
  const maxLoss = Math.abs(losers[0]?.pnl ?? 1)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div className="kpi-5" style={{ gridTemplateColumns: 'repeat(4,1fr)' }}>
        <StatCard label="All-Time Realized" value={fmtCurrency(data.allTime.realizedPnl)} sub={fmtPct(data.allTime.gainPct)} valueColor={clr(data.allTime.realizedPnl)} />
        <StatCard label="YTD Realized" value={fmtCurrency(data.ytd.realizedPnl)} sub={fmtPct(data.ytd.gainPct)} valueColor={clr(data.ytd.realizedPnl)} />
        <StatCard label="MTD Realized" value={fmtCurrency(data.mtd.realizedPnl)} sub={fmtPct(data.mtd.gainPct)} valueColor={clr(data.mtd.realizedPnl)} />
        <StatCard label="Total Invested" value={fmtCurrency(data.allTime.deposits)} sub={`Withdrawn: ${fmtCurrency(Math.abs(data.allTime.withdrawals))}`} />
      </div>

      <div className="grid-2">
        <div style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #1e293b' }}>
            <SectionLabel>Top Realized Gainers</SectionLabel>
          </div>
          <div style={{ padding: '8px 0' }}>
            {gainers.slice(0, 10).map((p, i) => (
              <div key={p.symbol} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 14px' }}>
                <div style={{ fontSize: 11, color: '#334155', fontWeight: 600, width: 16, textAlign: 'right' }}>{i + 1}</div>
                <div style={{ width: 26, height: 26, borderRadius: 6, flexShrink: 0, background: '#34d39922', border: '1px solid #34d39933', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 9, fontWeight: 700, color: '#34d399' }}>
                  {p.symbol.slice(0, 3)}
                </div>
                <span style={{ flex: 1, fontWeight: 500, fontSize: 12, minWidth: 0 }}>{p.symbol}</span>
                <div style={{ flex: 2, paddingRight: 8, minWidth: 40 }}>
                  <HorizBar value={p.pnl} total={maxGain} color="#34d399" height={4} />
                </div>
                <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, fontWeight: 600, color: '#34d399', flexShrink: 0 }}>+{fmtCurrency(p.pnl)}</span>
              </div>
            ))}
          </div>
        </div>

        <div style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #1e293b' }}>
            <SectionLabel>Top Realized Losers</SectionLabel>
          </div>
          <div style={{ padding: '8px 0' }}>
            {losers.slice(0, 10).map((p, i) => (
              <div key={p.symbol} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 14px' }}>
                <div style={{ fontSize: 11, color: '#334155', fontWeight: 600, width: 16, textAlign: 'right' }}>{i + 1}</div>
                <div style={{ width: 26, height: 26, borderRadius: 6, flexShrink: 0, background: '#f8717122', border: '1px solid #f8717133', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 9, fontWeight: 700, color: '#f87171' }}>
                  {p.symbol.slice(0, 3)}
                </div>
                <span style={{ flex: 1, fontWeight: 500, fontSize: 12, minWidth: 0 }}>{p.symbol}</span>
                <div style={{ flex: 2, paddingRight: 8, minWidth: 40 }}>
                  <HorizBar value={Math.abs(p.pnl)} total={maxLoss} color="#f87171" height={4} />
                </div>
                <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, fontWeight: 600, color: '#f87171', flexShrink: 0 }}>{fmtCurrency(p.pnl)}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Annual breakdown */}
      <div style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid #1e293b' }}>
          <SectionLabel>Annual Breakdown</SectionLabel>
        </div>
        <div style={{ overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 480 }}>
            <thead>
              <tr style={{ background: '#0f172a' }}>
                {['Year', 'Realized P&L', 'Dividends', 'Return %', 'Deposits', 'Buy Vol', 'Sell Vol'].map(h => (
                  <th key={h} style={{ padding: '8px 14px', textAlign: h === 'Year' ? 'left' : 'right', fontSize: 10, fontWeight: 600, color: '#475569', letterSpacing: '0.08em', textTransform: 'uppercase', whiteSpace: 'nowrap' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.byYear.map(y => (
                <tr
                  key={y.label}
                  style={{ borderTop: '1px solid #1e293b' }}
                  onMouseEnter={e => (e.currentTarget.style.background = '#1e293b44')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <td style={{ padding: '10px 14px', fontWeight: 600 }}>{y.label}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, fontWeight: 600, color: clr(y.rPnl) }}>{fmtCurrency(y.rPnl)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, color: '#94a3b8' }}>{fmtCurrency(y.divs)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                    <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: clr(y.gainPct) + '22', color: clr(y.gainPct) }}>{fmtPct(y.gainPct)}</span>
                  </td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', color: '#64748b', fontFamily: "'DM Mono', monospace", fontSize: 12 }}>{fmtCurrency(y.deposits)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', color: '#64748b', fontFamily: "'DM Mono', monospace", fontSize: 12 }}>{fmtK(y.buyVol)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', color: '#64748b', fontFamily: "'DM Mono', monospace", fontSize: 12 }}>{fmtK(y.sellVol)}</td>
                </tr>
              ))}
              <tr style={{ borderTop: '2px solid #334155', background: '#0f172a' }}>
                <td style={{ padding: '10px 14px', fontWeight: 700, color: '#e2e8f0' }}>All Time</td>
                <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, fontWeight: 700, color: clr(data.allTime.realizedPnl) }}>{fmtCurrency(data.allTime.realizedPnl)}</td>
                <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, fontWeight: 700, color: '#94a3b8' }}>{fmtCurrency(data.allTime.dividends)}</td>
                <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                  <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 12, fontWeight: 700, background: clr(data.allTime.gainPct) + '22', color: clr(data.allTime.gainPct) }}>{fmtPct(data.allTime.gainPct)}</span>
                </td>
                <td style={{ padding: '10px 14px', textAlign: 'right', color: '#94a3b8', fontFamily: "'DM Mono', monospace", fontWeight: 700 }}>{fmtCurrency(data.allTime.deposits)}</td>
                <td style={{ padding: '10px 14px', textAlign: 'right', color: '#64748b', fontFamily: "'DM Mono', monospace" }}>{fmtK(data.allTime.buyVol)}</td>
                <td style={{ padding: '10px 14px', textAlign: 'right', color: '#64748b', fontFamily: "'DM Mono', monospace" }}>{fmtK(data.allTime.sellVol)}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
