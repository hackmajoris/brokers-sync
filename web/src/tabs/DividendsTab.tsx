import type { PortfolioData } from '../types/portfolio'
import { fmtCurrency } from '../utils/format'
import { BROKER_COLORS } from '../constants'
import { HorizBar } from '../components/charts/HorizBar'
import { StatCard } from '../components/ui/StatCard'
import { SectionLabel } from '../components/ui/SectionLabel'
import { BrokerPill } from '../components/ui/BrokerPill'

interface Props {
  data: PortfolioData
}

export function DividendsTab({ data }: Props) {
  const maxDiv = data.topDivs[0]?.net ?? 1
  const maxBrokerDiv = Math.max(...data.brokers.map(b => Math.abs(b.dividends)), 1)
  const maxAnnualDiv = Math.max(...data.byYear.map(y => y.divs), 1)
  const largestPayer = data.topDivs[0]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div className="kpi-5" style={{ gridTemplateColumns: 'repeat(4,1fr)' }}>
        <StatCard label="Total Dividends (Net)" value={fmtCurrency(data.allTime.dividends)} sub="All time" />
        <StatCard label="Tax Withheld" value={fmtCurrency(Math.abs(data.allTime.taxWithheld))} sub="All time" valueColor="#f87171" />
        <StatCard label="YTD Dividends" value={fmtCurrency(data.ytd.dividends)} sub="Year-to-date" />
        <StatCard
          label="Largest Payer"
          value={largestPayer?.symbol ?? '—'}
          sub={largestPayer ? `${fmtCurrency(largestPayer.net)} net all time` : ''}
          valueColor="#facc15"
        />
      </div>

      <div className="div-chart-annual">
        {/* By symbol */}
        <div style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #1e293b' }}>
            <SectionLabel>Dividends by Symbol (Net, USD)</SectionLabel>
          </div>
          <div style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: 6 }}>
            {data.topDivs.map(d => (
              <div key={d.symbol} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div style={{ width: 28, height: 28, borderRadius: 6, flexShrink: 0, background: '#facc1522', border: '1px solid #facc1533', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 8, fontWeight: 700, color: '#facc15' }}>
                  {d.symbol.slice(0, 3)}
                </div>
                <span style={{ width: 56, fontWeight: 500, fontSize: 12, flexShrink: 0 }}>{d.symbol}</span>
                <div style={{ flex: 1, minWidth: 40 }}>
                  <HorizBar value={d.net} total={maxDiv} color="#facc15" height={6} bg="#1e293b" />
                </div>
                <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 13, fontWeight: 600, color: '#facc15', minWidth: 64, textAlign: 'right' }}>{fmtCurrency(d.net)}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Annual dividends */}
        <div style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #1e293b' }}>
            <SectionLabel>Annual Dividends</SectionLabel>
          </div>
          <div style={{ padding: '12px 0' }}>
            {data.byYear.filter(y => y.divs > 0).map(y => (
              <div key={y.label} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 16px' }}>
                <span style={{ width: 36, fontSize: 12, fontWeight: 600, color: '#94a3b8', flexShrink: 0 }}>{y.label}</span>
                <div style={{ flex: 1 }}>
                  <HorizBar value={y.divs} total={maxAnnualDiv} color="#fb923c" height={6} />
                </div>
                <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, fontWeight: 600, color: '#fb923c', minWidth: 56, textAlign: 'right' }}>{fmtCurrency(y.divs)}</span>
              </div>
            ))}
          </div>
          <div style={{ padding: '12px 16px', borderTop: '1px solid #1e293b', background: '#0f172a' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: 11, fontWeight: 600, color: '#94a3b8' }}>Total Net</span>
              <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 14, fontWeight: 700, color: '#fb923c' }}>{fmtCurrency(data.allTime.dividends)}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 4 }}>
              <span style={{ fontSize: 11, color: '#475569' }}>Tax withheld</span>
              <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, color: '#f87171' }}>−{fmtCurrency(Math.abs(data.allTime.taxWithheld))}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Broker breakdown */}
      <div style={{ background: '#111827', borderRadius: 10, border: '1px solid #1e293b', overflow: 'hidden' }}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid #1e293b' }}>
          <SectionLabel>Dividends by Broker</SectionLabel>
        </div>
        <div className="broker-div-row">
          {data.brokers.map(b => {
            const bc = BROKER_COLORS[b.name] ?? '#94a3b8'
            return (
              <div
                key={b.name}
                style={{ padding: '16px', borderRight: '1px solid #1e293b', borderBottom: '1px solid #1e293b' }}
              >
                <BrokerPill name={b.name} />
                <div style={{ marginTop: 12, fontSize: 20, fontWeight: 700, color: b.dividends > 0 ? '#facc15' : '#f87171' }}>
                  {fmtCurrency(Math.abs(b.dividends))}
                </div>
                <div style={{ marginTop: 4, fontSize: 11, color: '#475569' }}>
                  {b.dividends > 0 ? 'Net dividends' : 'Tax-negative'}
                </div>
                <div style={{ marginTop: 10 }}>
                  <HorizBar value={Math.abs(b.dividends)} total={maxBrokerDiv} color={bc} height={4} />
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
