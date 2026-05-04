import { useState, useEffect } from 'react'
import type { PortfolioData } from './types/portfolio'
import { fetchPortfolioData } from './services/portfolioService'
import { ACCENT_DEFAULT } from './constants'
import { OverviewTab } from './tabs/OverviewTab'
import { BrokersTab } from './tabs/BrokersTab'
import { PositionsTab } from './tabs/PositionsTab'
import { PnLTab } from './tabs/PnLTab'
import { DividendsTab } from './tabs/DividendsTab'

const TABS = [
  { id: 'overview', label: 'Overview' },
  { id: 'brokers', label: 'Brokers' },
  { id: 'positions', label: 'Positions' },
  { id: 'pnl', label: 'P&L' },
  { id: 'dividends', label: 'Dividends' },
] as const

type TabId = (typeof TABS)[number]['id']

export function App() {
  const [data, setData] = useState<PortfolioData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<TabId>('overview')
  const accent = ACCENT_DEFAULT

  useEffect(() => {
    fetchPortfolioData()
      .then(setData)
      .catch(e => setError(String(e)))
  }, [])

  const generatedDate = data
    ? new Date(data.generatedAt).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
    : '…'

  return (
    <div style={{ minHeight: '100vh', background: '#0b0e14', color: '#e2e8f0' }}>
      {/* Header */}
      <header style={{ background: '#0f1623', borderBottom: '1px solid #1e293b', padding: '0 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', height: 52, position: 'sticky', top: 0, zIndex: 10, gap: 8 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
          <div style={{ width: 28, height: 28, borderRadius: 7, background: `linear-gradient(135deg, ${accent}cc, ${accent}44)`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 12, fontWeight: 700, color: 'white', flexShrink: 0 }}>B</div>
          <span style={{ fontWeight: 600, fontSize: 15, letterSpacing: '-0.02em', whiteSpace: 'nowrap' }}>broker<span style={{ color: accent }}>sync</span></span>
        </div>
        <nav style={{ display: 'flex', gap: 2, overflowX: 'auto', flexShrink: 1, minWidth: 0 }}>
          {TABS.map(t => (
            <button
              key={t.id}
              onClick={() => setActiveTab(t.id)}
              style={{
                background: activeTab === t.id ? accent + '22' : 'transparent',
                border: activeTab === t.id ? `1px solid ${accent}55` : '1px solid transparent',
                color: activeTab === t.id ? accent : '#64748b',
                padding: '4px 10px', borderRadius: 6, fontSize: 12, fontWeight: 500, cursor: 'pointer',
                transition: 'all 0.15s', whiteSpace: 'nowrap', flexShrink: 0,
              }}
            >{t.label}</button>
          ))}
        </nav>
        <div style={{ fontSize: 11, color: '#475569', flexShrink: 0 }} className="date-label">
          As of <span style={{ color: '#94a3b8' }}>{generatedDate}</span>
        </div>
      </header>

      <main style={{ padding: '16px', maxWidth: 1300, margin: '0 auto', width: '100%' }}>
        {error && (
          <div style={{ background: '#f8717122', border: '1px solid #f8717144', borderRadius: 8, padding: '12px 16px', color: '#f87171', marginBottom: 16 }}>
            Failed to load data: {error}
          </div>
        )}

        {!data && !error && (
          <div style={{ color: '#475569', fontSize: 14, padding: '40px 0', textAlign: 'center' }}>
            Loading portfolio data…
          </div>
        )}

        {data && (
          <>
            {activeTab === 'overview' && <OverviewTab data={data} accent={accent} />}
            {activeTab === 'brokers' && <BrokersTab data={data} accent={accent} />}
            {activeTab === 'positions' && <PositionsTab data={data} accent={accent} />}
            {activeTab === 'pnl' && <PnLTab data={data} />}
            {activeTab === 'dividends' && <DividendsTab data={data} />}
          </>
        )}
      </main>
    </div>
  )
}
