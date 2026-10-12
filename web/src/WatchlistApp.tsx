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
    <div style={{ minHeight: '100vh', background: '#000000', color: '#ffffff', display: 'flex', flexDirection: 'column' }}>
      <header style={{ background: '#0a0a0a', borderBottom: '1px solid #161f31', padding: '0 16px', display: 'flex', alignItems: 'center', height: 52, position: 'sticky', top: 0, zIndex: 10 }}>
        <img src={logo} alt="Watchlist" height={28} style={{ display: 'block', flexShrink: 0 }} />
      </header>

      <main style={{ padding: '16px', maxWidth: 1300, margin: '0 auto', width: '100%', flex: 1 }}>
        <WatchlistTab accent={accent} />
      </main>

      <footer style={{ borderTop: '1px solid #161f31', padding: '16px', fontSize: 11, color: '#8b8fa3', lineHeight: 1.6 }}>
        <div style={{ maxWidth: 1300, margin: '0 auto', display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', gap: '8px 24px' }}>
          <span>Market data from Yahoo Finance, may be delayed. Not investment advice.</span>
          <span style={{ display: 'flex', flexWrap: 'wrap', gap: '4px 14px' }}>
            <span style={{ color: '#3a3a3a' }}>build {buildDate}</span>
          </span>
        </div>
      </footer>

      <StockLookup accent={accent} />
    </div>
  )
}
