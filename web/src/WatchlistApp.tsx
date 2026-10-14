import { WatchlistTab } from './tabs/WatchlistTab'
import { StockLookup } from './components/StockLookup'
import logo from './assets/watchlist-logo.svg'
import logoLight from './assets/watchlist-logo-light.svg'
import { ThemeToggle } from './components/ui/ThemeToggle'
import { useTheme } from './lib/theme'
import { useIsMobile } from './lib/useIsMobile'


// Standalone shell for the watchlist app. It reuses WatchlistTab unchanged and
// loads no portfolio data, so it works without any broker export.
export function WatchlistApp() {
  const accent = 'var(--accent)'
  const { theme, setTheme } = useTheme()
  const mobile = useIsMobile()
  const buildDate = new Date(__BUILD_DATE__).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg)', color: 'var(--text)', display: 'flex', flexDirection: 'column' }}>
      <header style={{ background: 'var(--surface)', borderBottom: '1px solid var(--line)', padding: '0 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, height: 52, position: 'sticky', top: 0, zIndex: 10 }}>
        <img src={theme === 'light' ? logoLight : logo} alt="Watchlist" height={28} style={{ display: 'block', flexShrink: 0 }} />
        <ThemeToggle value={theme} onChange={setTheme} compact={mobile} />
      </header>

      <main style={{ padding: '16px', maxWidth: 1300, margin: '0 auto', width: '100%', flex: 1 }}>
        <WatchlistTab accent={accent} />
      </main>

      <footer style={{ borderTop: '1px solid var(--line)', padding: '16px', fontSize: 11, color: 'var(--text-3)', lineHeight: 1.6 }}>
        <div style={{ maxWidth: 1300, margin: '0 auto', display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', gap: '8px 24px' }}>
          <span>Market data from Yahoo Finance, may be delayed. Not investment advice.</span>
          <span style={{ display: 'flex', flexWrap: 'wrap', gap: '4px 14px' }}>
            <span style={{ color: 'var(--text-build)' }}>build {buildDate}</span>
          </span>
        </div>
      </footer>

      <StockLookup accent={accent} />
    </div>
  )
}
