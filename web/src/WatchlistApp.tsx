import { ACCENT_DEFAULT } from './constants'
import { WatchlistTab } from './tabs/WatchlistTab'
import { StockLookup } from './components/StockLookup'

// Standalone shell for the watchlist app. It reuses WatchlistTab unchanged and
// loads no portfolio data, so it works without any broker export.
export function WatchlistApp() {
  const accent = ACCENT_DEFAULT
  const buildDate = new Date(__BUILD_DATE__).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })

  return (
    <div style={{ minHeight: '100vh', background: '#000000', color: '#ffffff' }}>
      <header style={{ background: '#0a0a0a', borderBottom: '1px solid #161f31', padding: '0 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', height: 52, position: 'sticky', top: 0, zIndex: 10, gap: 8 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
          <div style={{ width: 28, height: 28, borderRadius: 7, background: `linear-gradient(135deg, ${accent}cc, ${accent}44)`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 12, fontWeight: 700, color: 'white', flexShrink: 0 }}>W</div>
          <span style={{ fontWeight: 600, fontSize: 15, letterSpacing: '-0.02em', whiteSpace: 'nowrap' }}>watch<span style={{ color: accent }}>list</span></span>
        </div>
        <div style={{ fontSize: 10, color: '#3a3a3a' }} className="date-label">
          build: {buildDate}
        </div>
      </header>

      <main style={{ padding: '16px', maxWidth: 1300, margin: '0 auto', width: '100%' }}>
        <WatchlistTab accent={accent} />
      </main>

      <StockLookup accent={accent} />
    </div>
  )
}
