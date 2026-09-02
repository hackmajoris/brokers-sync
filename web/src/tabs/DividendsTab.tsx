import { useState } from 'react'
import type { PortfolioData } from '../types/portfolio'
import { fmtCurrency } from '../utils/format'
import { BROKER_COLORS } from '../constants'
import { HorizBar } from '../components/charts/HorizBar'
import { BarChart } from '../components/charts/BarChart'
import { LineChart } from '../components/charts/LineChart'
import { ChartModal } from '../components/charts/ChartModal'
import { StatCard } from '../components/ui/StatCard'
import { SectionLabel } from '../components/ui/SectionLabel'
import { BrokerPill } from '../components/ui/BrokerPill'

interface Props {
  data: PortfolioData
}

export function DividendsTab({ data }: Props) {
  const [chartType, setChartType] = useState<'bar' | 'line'>('bar')
  const [modalOpen, setModalOpen] = useState(false)

  const maxDiv = data.topDivs[0]?.net ?? 1
  const maxBrokerDiv = Math.max(...data.brokers.map(b => Math.abs(b.dividends)), 1)
  const maxAnnualDiv = Math.max(...data.byYear.map(y => y.divs), 1)
  const largestPayer = data.topDivs[0]
  const yearDivs = data.byYear.filter(y => y.divs > 0)
  const chartData = yearDivs.map(y => ({ year: y.label, divs: y.divs }))

  const typeToggle = {
    value: chartType,
    onChange: (v: string) => setChartType(v as 'bar' | 'line'),
    options: [
      { value: 'bar', label: 'Bar' },
      { value: 'line', label: 'Line' },
    ],
  }

  const chart = chartType === 'bar'
    ? <BarChart data={chartData} keyX="year" keyY="divs" color="#fb923c" height={160} />
    : <LineChart data={chartData} keyX="year" keyY="divs" color="#fb923c" height={160} />

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

      {/* Progress chart + Annual dividends side by side */}
      <div className="div-chart-annual">
        {/* Dividend progress by year */}
        <div style={{ background: '#090f1c', borderRadius: 10, border: '1px solid #161f31', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #161f31', display: 'flex', alignItems: 'center', gap: 8 }}>
            <SectionLabel>Dividend Progress by Year</SectionLabel>
            <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 10, color: '#555', marginLeft: 4 }}>
              {yearDivs.length}y
            </span>
            <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 6 }}>
              <div className="chart-type-toggle">
                {typeToggle.options.map(o => (
                  <button
                    key={o.value}
                    className={`chart-type-btn ${chartType === o.value ? 'active' : ''}`}
                    onClick={() => setChartType(o.value as 'bar' | 'line')}
                  >
                    {o.label}
                  </button>
                ))}
              </div>
              <button
                className="modal-close"
                style={{ width: 28, height: 28, fontSize: 12 }}
                title="Expand"
                onClick={() => setModalOpen(true)}
              >
                ↗
              </button>
            </div>
          </div>
          <div style={{ padding: '16px 20px 16px' }}>
            {chart}
          </div>
        </div>

        {/* Annual dividends */}
        <div style={{ background: '#090f1c', borderRadius: 10, border: '1px solid #161f31', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #161f31' }}>
            <SectionLabel>Annual Dividends</SectionLabel>
          </div>
          <div style={{ padding: '12px 0' }}>
            {yearDivs.map(y => (
              <div key={y.label} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 16px' }}>
                <span style={{ width: 36, fontSize: 12, fontWeight: 600, color: '#c0c0c0', flexShrink: 0 }}>{y.label}</span>
                <div style={{ flex: 1 }}>
                  <HorizBar value={y.divs} total={maxAnnualDiv} color="#fb923c" height={6} />
                </div>
                <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, fontWeight: 600, color: '#fb923c', minWidth: 56, textAlign: 'right' }}>{fmtCurrency(y.divs)}</span>
              </div>
            ))}
          </div>
          <div style={{ padding: '12px 16px', borderTop: '1px solid #161f31', background: '#080808' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: 11, fontWeight: 600, color: '#c0c0c0' }}>Total Net</span>
              <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 14, fontWeight: 700, color: '#fb923c' }}>{fmtCurrency(data.allTime.dividends)}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 4 }}>
              <span style={{ fontSize: 11, color: '#8b8fa3' }}>Tax withheld</span>
              <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, color: '#f87171' }}>−{fmtCurrency(Math.abs(data.allTime.taxWithheld))}</span>
            </div>
          </div>
        </div>
      </div>

      {/* By symbol */}
      <div style={{ background: '#090f1c', borderRadius: 10, border: '1px solid #161f31', overflow: 'hidden' }}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid #161f31' }}>
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
                <HorizBar value={d.net} total={maxDiv} color="#facc15" height={6} bg="#161f31" />
              </div>
              <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 13, fontWeight: 600, color: '#facc15', minWidth: 64, textAlign: 'right' }}>{fmtCurrency(d.net)}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Broker breakdown */}
      <div style={{ background: '#090f1c', borderRadius: 10, border: '1px solid #161f31', overflow: 'hidden' }}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid #161f31' }}>
          <SectionLabel>Dividends by Broker</SectionLabel>
        </div>
        <div className="broker-div-row">
          {data.brokers.map(b => {
            const bc = BROKER_COLORS[b.name] ?? '#c0c0c0'
            return (
              <div
                key={b.name}
                style={{ padding: '16px', borderRight: '1px solid #161f31', borderBottom: '1px solid #161f31' }}
              >
                <BrokerPill name={b.name} />
                <div style={{ marginTop: 12, fontSize: 20, fontWeight: 700, color: b.dividends > 0 ? '#facc15' : '#f87171' }}>
                  {fmtCurrency(Math.abs(b.dividends))}
                </div>
                <div style={{ marginTop: 4, fontSize: 11, color: '#8b8fa3' }}>
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

      {/* Expanded modal */}
      {modalOpen && (
        <ChartModal
          label="Dividend Progress by Year"
          value={fmtCurrency(data.allTime.dividends)}
          onClose={() => setModalOpen(false)}
          typeToggle={typeToggle}
        >
          {chartType === 'bar'
            ? <BarChart data={chartData} keyX="year" keyY="divs" color="#fb923c" height={400} />
            : <LineChart data={chartData} keyX="year" keyY="divs" color="#fb923c" height={400} />
          }
        </ChartModal>
      )}
    </div>
  )
}
