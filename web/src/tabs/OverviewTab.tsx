import { useState } from 'react'
import type { PortfolioData } from '../types/portfolio'
import { fmt, fmtCurrency, fmtPct, fmtK, clr } from '../utils/format'
import { BROKER_COLORS, BROKER_LABELS } from '../constants'
import { BarChart } from '../components/charts/BarChart'
import { DonutChart } from '../components/charts/DonutChart'
import { HorizBar } from '../components/charts/HorizBar'
import { StatCard } from '../components/ui/StatCard'
import { SectionLabel } from '../components/ui/SectionLabel'
import { BrokerPill } from '../components/ui/BrokerPill'

interface Props {
  data: PortfolioData
  accent: string
}

export function OverviewTab({ data, accent }: Props) {
  const [chartMode, setChartMode] = useState<'gainPct' | 'rPnl'>('gainPct')

  const totalMV = data.openPositions.reduce((s, p) => s + (p.mv ?? 0), 0)
  const totalUPnl = data.openPositions.reduce((s, p) => s + (p.pnl ?? 0), 0)
  const totalAmount = totalMV + data.cashBalance;

  const brokerAlloc = data.brokers
    .map(b => ({
      name: b.name,
      value: b.positions.reduce((s, p) => s + (p.mv ?? 0), 0),
      color: BROKER_COLORS[b.name] ?? '#c0c0c0',
    }))
    .filter(b => b.value > 0)
    .sort((a, b) => b.value - a.value)

  const allocTotal = brokerAlloc.reduce((s, b) => s + b.value, 0)

  const yearlyData = data.byYear.map(y => ({
    year: y.label.slice(2),
    val: chartMode === 'gainPct' ? y.gainPct : y.rPnl,
  }))

  const periodRows = [
    { label: 'All Time', d: data.allTime },
    { label: 'YTD 2026', d: data.ytd },
    { label: 'MTD May', d: data.mtd },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* KPI strip */}
      <div className="kpi-6">
        <StatCard
          label="Total Amount"
          value={fmtCurrency(totalAmount)}
          sub="MV + Cash + Unrealized P&L"
          valueColor={clr(totalAmount)}
        />
        <StatCard label="Total Market Value" value={fmtCurrency(totalMV)} sub="Open positions" />
        <StatCard
          label="Cash (all brokers)"
          value={fmtCurrency(data.cashBalance)}
          sub="Uninvested"
          valueColor={data.cashBalance >= 0 ? '#34d399' : '#f87171'}
        />
        <StatCard
          label="Unrealized P&L"
          value={fmtCurrency(totalUPnl)}
          sub={fmtPct((totalUPnl / data.allTime.deposits) * 100)}
          valueColor={clr(totalUPnl)}
        />
        <StatCard
          label="All-Time Realized"
          value={fmtCurrency(data.allTime.realizedPnl)}
          sub={fmtPct(data.allTime.gainPct)}
          valueColor={clr(data.allTime.realizedPnl)}
        />
        <StatCard
          label="Total Dividends"
          value={fmtCurrency(data.allTime.dividends)}
          sub={`Tax: −$${fmt(Math.abs(data.allTime.taxWithheld))}`}
        />
      </div>

      {/* Annual chart + donut */}
      <div className="chart-donut">
        <div style={{ background: '#0f0f0f', borderRadius: 10, padding: '16px 18px', border: '1px solid #1a1a1a', minWidth: 0, overflow: 'hidden' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <SectionLabel>Annual Performance</SectionLabel>
            <div style={{ display: 'flex', gap: 4 }}>
              {([['gainPct', 'Return %'], ['rPnl', 'Realized $']] as const).map(([m, l]) => (
                <button
                  key={m}
                  onClick={() => setChartMode(m)}
                  style={{
                    fontSize: 10, padding: '2px 8px', borderRadius: 4, cursor: 'pointer',
                    background: chartMode === m ? accent + '33' : '#1a1a1a',
                    border: chartMode === m ? `1px solid ${accent}66` : '1px solid #252525',
                    color: chartMode === m ? accent : '#888888',
                  }}
                >{l}</button>
              ))}
            </div>
          </div>
          <BarChart
            data={yearlyData} keyX="year" keyY="val"
            colorFn={v => v >= 0 ? '#34d399' : '#f87171'}
            height={130}
            formatValue={chartMode === 'gainPct'
              ? v => `${v >= 0 ? '+' : ''}${v.toFixed(1)}%`
              : v => v >= 1000 ? `$${(v / 1000).toFixed(0)}k` : `$${Math.round(v)}`
            }
          />
          <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 10 }}>
            {data.byYear.map(y => (
              <div key={y.label} style={{ textAlign: 'center', flex: 1 }}>
                <div style={{ fontSize: 10, color: y.gainPct >= 0 ? '#34d399' : '#f87171', fontWeight: 600 }}>
                  {y.gainPct >= 0 ? '+' : ''}{fmt(y.gainPct)}%
                </div>
              </div>
            ))}
          </div>
        </div>

        <div style={{ background: '#0f0f0f', borderRadius: 10, padding: '16px 18px', border: '1px solid #1a1a1a', minWidth: 0, overflow: 'hidden' }}>
          <SectionLabel>Broker Allocation</SectionLabel>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
            <DonutChart slices={brokerAlloc} size={100} />
            <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 6 }}>
              {brokerAlloc.map(b => (
                <div key={b.name} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ width: 8, height: 8, borderRadius: '50%', background: b.color, flexShrink: 0 }} />
                  <div style={{ fontSize: 11, color: '#c0c0c0', flex: 1 }}>{BROKER_LABELS[b.name] ?? b.name}</div>
                  <div style={{ fontSize: 11, fontWeight: 600, color: '#ffffff' }}>{fmtK(b.value)}</div>
                  <div style={{ fontSize: 10, color: '#888888', width: 32, textAlign: 'right' }}>
                    {fmt(allocTotal > 0 ? (b.value / allocTotal) * 100 : 0, 1)}%
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Period summary + broker perf */}
      <div className="grid-2">
        <div style={{ background: '#0f0f0f', borderRadius: 10, padding: '16px 18px', border: '1px solid #1a1a1a' }}>
          <SectionLabel>Period Summary</SectionLabel>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 8 }}>
            {periodRows.map(({ label, d }) => (
              <div key={label} style={{ background: '#080808', borderRadius: 8, padding: '10px 12px', border: '1px solid #1a1a1a' }}>
                <div style={{ fontSize: 10, color: '#555555', fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', marginBottom: 6 }}>{label}</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: clr(d.gainPct), marginBottom: 4 }}>{fmtPct(d.gainPct)}</div>
                <div style={{ fontSize: 10, color: '#888888' }}>Realized: <span style={{ color: '#c0c0c0' }}>{fmtCurrency(d.realizedPnl)}</span></div>
                <div style={{ fontSize: 10, color: '#888888' }}>Dividends: <span style={{ color: '#c0c0c0' }}>{fmtCurrency(d.dividends)}</span></div>
              </div>
            ))}
          </div>
        </div>

        <div style={{ background: '#0f0f0f', borderRadius: 10, padding: '16px 18px', border: '1px solid #1a1a1a' }}>
          <SectionLabel>Broker Performance (All-Time)</SectionLabel>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {data.brokers.map(b => (
              <div key={b.name} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div style={{ width: 72, flexShrink: 0 }}>
                  <BrokerPill name={b.name} />
                </div>
                <div style={{ flex: 1 }}>
                  <HorizBar value={Math.max(b.allTimeGain, 0)} total={50} color={BROKER_COLORS[b.name] ?? '#c0c0c0'} height={5} />
                </div>
                <div style={{ width: 50, textAlign: 'right', fontSize: 12, fontWeight: 600, color: clr(b.allTimeGain) }}>
                  {fmtPct(b.allTimeGain)}
                </div>
                <div style={{ width: 64, textAlign: 'right', fontSize: 11, color: '#888888' }}>
                  {fmtK(b.allTimeRPnl)}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
