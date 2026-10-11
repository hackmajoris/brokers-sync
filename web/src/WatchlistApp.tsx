import { ACCENT_DEFAULT } from './constants'
import { WatchlistTab } from './tabs/WatchlistTab'
import { StockLookup } from './components/StockLookup'
import logo from './assets/watchlist-logo.svg'

// Standalone shell for the watchlist app. It reuses WatchlistTab unchanged and
// loads no portfolio data, so it works without any broker export.
export function WatchlistApp() {
  const accent = ACCENT_DEFAULT
  const buildDate = new Date(__BUILD_DATE__).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })

  return (
    <div style={{ minHeight: '100vh', background: '#000000', color: '#ffffff' }}>
      <header style={{ background: '#0a0a0a', borderBottom: '1px solid #161f31', padding: '0 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', height: 52, position: 'sticky', top: 0, zIndex: 10, gap: 8 }}>
        <img src={logo} alt="Watchlist" height={28} style={{ display: 'block', flexShrink: 0 }} />
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
