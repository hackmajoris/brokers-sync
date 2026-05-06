import { useState, useEffect } from 'react'
import type { PortfolioData } from './types/portfolio'
import { fetchPortfolioData, uploadZip, mapRawPortfolio, cacheRawPortfolio } from './services/portfolioService'
import { loadMockPortfolio } from './services/mockService'
import { loadCachedZip } from './services/zipCache'
import { ACCENT_DEFAULT } from './constants'
import { OverviewTab } from './tabs/OverviewTab'
import { BrokersTab } from './tabs/BrokersTab'
import { PositionsTab } from './tabs/PositionsTab'
import { PnLTab } from './tabs/PnLTab'
import { DividendsTab } from './tabs/DividendsTab'
import { SettingsView } from './views/SettingsView'

const TABS = [
  { id: 'overview', label: 'Overview' },
  { id: 'brokers', label: 'Brokers' },
  { id: 'positions', label: 'Positions' },
  { id: 'pnl', label: 'P&L' },
  { id: 'dividends', label: 'Dividends' },
  { id: 'settings', label: 'Settings' },
] as const

type TabId = (typeof TABS)[number]['id']

export function App() {
  const [data, setData] = useState<PortfolioData | null>(null)
  const [noData, setNoData] = useState(false)
  const [isDemo, setIsDemo] = useState(false)
  const [activeTab, setActiveTab] = useState<TabId>('overview')
  const [refreshing, setRefreshing] = useState(false)
  const accent = ACCENT_DEFAULT

  useEffect(() => {
    async function init() {
      const cached = await loadCachedZip()
      if (cached) {
        // Show cached JSON immediately, then refresh in background
        const initial = await fetchPortfolioData().catch(() => null)
        if (initial) setData(initial); else { setNoData(true); setActiveTab('settings') }

        setRefreshing(true)
        try {
          const raw = await uploadZip(cached.blob, cached.name)
          if (raw) {
            cacheRawPortfolio(raw)
            setData(mapRawPortfolio(raw))
            setNoData(false)
          }
        } catch { /* silent — stale data is still shown */ }
        setRefreshing(false)
      } else {
        const real = await fetchPortfolioData().catch(() => null)
        if (real) {
          setData(real)
        } else {
          setData(loadMockPortfolio())
          setIsDemo(true)
        }
      }
    }
    init()
  }, [])

  const generatedDate = data
    ? new Date(data.generatedAt).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
    : '…'

  return (
    <div style={{ minHeight: '100vh', background: '#000000', color: '#ffffff' }}>
      {/* Header */}
      <header style={{ background: '#0a0a0a', borderBottom: '1px solid #1a1a1a', padding: '0 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', height: 52, position: 'sticky', top: 0, zIndex: 10, gap: 8 }}>
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
                color: activeTab === t.id ? accent : '#888888',
                padding: '4px 10px', borderRadius: 6, fontSize: 12, fontWeight: 500, cursor: 'pointer',
                transition: 'all 0.15s', whiteSpace: 'nowrap', flexShrink: 0,
              }}
            >{t.label}</button>
          ))}
        </nav>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
          {isDemo && (
            <span style={{
              fontSize: 10, fontWeight: 700, letterSpacing: '0.08em',
              background: '#f59e0b22', border: '1px solid #f59e0b55',
              color: '#f59e0b', borderRadius: 4, padding: '2px 7px',
              userSelect: 'none',
            }}>DEMO</span>
          )}
          <div style={{ fontSize: 11, color: '#555555' }} className="date-label">
            {refreshing
              ? <span style={{ color: accent, opacity: 0.7 }}>Refreshing…</span>
              : <>As of <span style={{ color: '#c0c0c0' }}>{generatedDate}</span></>
            }
          </div>
        </div>
      </header>

      <main style={{ padding: '16px', maxWidth: 1300, margin: '0 auto', width: '100%' }}>
        {activeTab === 'settings' && (
          <SettingsView onImported={d => { setData(d); setNoData(false); setIsDemo(false); setActiveTab('overview') }} noData={noData} />
        )}

        {!noData && activeTab !== 'settings' && !data && (
          <div style={{ color: '#555555', fontSize: 14, padding: '40px 0', textAlign: 'center' }}>
            Loading portfolio data…
          </div>
        )}

        {data && activeTab !== 'settings' && (
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
