import type { Plugin } from 'vite'
import type { IncomingMessage, ServerResponse } from 'http'

// Dev-only in-memory stand-in for the watchlist API. Enabled with MOCK_API=1
// (npm run dev:mock) so a normal `npm run dev` still proxies /api to the real
// backend on :8080.

interface MockEntry {
  symbol: string
  note: string
  targetPrice: number
  pinned: boolean
  addedAt: number
  indicators?: Record<string, unknown>
}

// Enough rows to scroll, so the sticky header, the sticky symbol column, pinning
// and compare mode can all be exercised in dev. Indicators are synthesised from
// the symbol, which keeps the values stable across reloads but varied enough to
// sort on. Two are pinned so the pinned-first order is visible on first load.
const BULK: MockEntry[] = [
  ['AAPL', 'Core holding candidate', 190, true],
  ['MSFT', 'Waiting on cloud margin', 350, true],
  ['GOOGL', 'Antitrust discount', 140, false],
  ['AMZN', 'Retail margin inflecting', 160, false],
  ['TSLA', 'No thesis, watching only', 180, false],
  ['META', 'Capex risk', 420, false],
  ['AVGO', 'Rich but compounding', 900, false],
  ['AMD', 'Second source to NVDA', 110, false],
  ['TSM', 'Geopolitical discount', 150, false],
  ['NESN.SW', 'Defensive, slow', 82, false],
  ['BRK-B', 'Cash pile optionality', 400, false],
  ['JNJ', 'Dividend anchor', 140, false],
  ['V', 'Toll booth', 250, false],
  ['MA', 'Same thesis as V', 420, false],
  ['UNH', 'Regulatory overhang', 450, false],
  ['COST', 'Never cheap', 700, false],
  ['LVMH.PA', 'China demand', 550, false],
  ['SAP.DE', 'Cloud transition', 180, false],
  ['NOVO-B.CO', 'GLP-1 concentration risk', 90, false],
  ['SHOP', 'High beta', 60, false],
  ['UBER', 'FCF turning', 55, false],
  ['NFLX', 'Ads ramp', 600, false],
].map(([symbol, note, targetPrice, pinned], i) => ({
  symbol: symbol as string,
  note: note as string,
  targetPrice: targetPrice as number,
  pinned: pinned as boolean,
  addedAt: Date.now() - 86400000 * (7 + i * 3),
  indicators: synthIndicators(symbol as string),
}))

const SEED: MockEntry[] = [
  {
    symbol: 'NVDA',
    note: 'Waiting for a pullback under 150',
    targetPrice: 150,
    pinned: false,
    addedAt: Date.now() - 86400000 * 12,
    indicators: {
      symbol: 'NVDA', currency: 'USD', quantity: 0, avg_cost: 0, total_cost: 0,
      current_price: 181.36, market_value: 0, unrealized_pnl: 0, unrealized_pct_omitempty: 0,
      week_52_low: 86.62, week_52_high: 212.19, pe: 48.2, forward_pe: 31.4,
      today_return: 4.91, one_week_return: 8.2, one_month_return: 12.4, ten_year_return: 9840.5,
      market_cap: 4_500_000_000_000,
      ytd_return: 34.8, three_year_return: 612.4, five_year_return: 1284.1,
      fcf: 72_100_000_000, ev_to_ebitda: 42.6, debt_to_equity: 12.9, cash_flow_quality: 1.08,
      health_rating: 'strong', valuation_rating: 'expensive',
    },
  },
  {
    symbol: 'ASML',
    note: 'Monopoly on EUV, cyclical entry point',
    targetPrice: 620,
    pinned: false,
    addedAt: Date.now() - 86400000 * 30,
    indicators: {
      symbol: 'ASML', currency: 'EUR', quantity: 0, avg_cost: 0, total_cost: 0,
      current_price: 704.2, market_value: 0, unrealized_pnl: 0, unrealized_pct_omitempty: 0,
      week_52_low: 578.5, week_52_high: 1035.0, pe: 33.1, forward_pe: 26.7,
      today_return: -2.14, one_week_return: -3.6, one_month_return: 2.1, ten_year_return: 812.4,
      market_cap: 292_000_000_000,
      ytd_return: -8.4, three_year_return: 41.2, five_year_return: 168.9,
      fcf: 9_400_000_000, ev_to_ebitda: 24.3, debt_to_equity: 28.4, cash_flow_quality: 0.94,
      health_rating: 'strong', valuation_rating: 'fair',
    },
  },
  {
    symbol: 'KO',
    note: '',
    targetPrice: 0,
    pinned: false,
    addedAt: Date.now() - 86400000 * 3,
    indicators: {
      symbol: 'KO', currency: 'USD', quantity: 0, avg_cost: 0, total_cost: 0,
      current_price: 68.9, market_value: 0, unrealized_pnl: 0, unrealized_pct_omitempty: 0,
      week_52_low: 60.6, week_52_high: 74.4, pe: 24.8, forward_pe: 22.1,
      today_return: 0.42, one_week_return: 1.1, one_month_return: -0.8, ten_year_return: 96.3,
      market_cap: 264_000_000_000,
      ytd_return: 6.1, three_year_return: 18.7, five_year_return: 44.3,
      fcf: 9_800_000_000, ev_to_ebitda: 19.2, debt_to_equity: 168.5, cash_flow_quality: 0.87,
      health_rating: 'moderate', valuation_rating: 'fair',
    },
  },
  {
    symbol: 'PLTR',
    note: 'Too rich for now, tracking only',
    targetPrice: 45,
    pinned: false,
    addedAt: Date.now() - 86400000 * 60,
    indicators: {
      symbol: 'PLTR', currency: 'USD', quantity: 0, avg_cost: 0, total_cost: 0,
      current_price: 172.4, market_value: 0, unrealized_pnl: 0, unrealized_pct_omitempty: 0,
      week_52_low: 66.1, week_52_high: 190.0, pe: 486.3, forward_pe: 214.7,
      today_return: -5.62, one_week_return: -11.4, one_month_return: 18.7,
      market_cap: 378_000_000_000,
      ytd_return: 128.6, three_year_return: 1420.5, five_year_return: 1690.2,
      fcf: 1_600_000_000, ev_to_ebitda: 312.4, debt_to_equity: 4.7, cash_flow_quality: 1.42,
      health_rating: 'strong', valuation_rating: 'very expensive',
    },
  },
  {
    symbol: 'BAYN.DE',
    note: 'Litigation overhang — no indicators upstream',
    targetPrice: 22,
    pinned: false,
    addedAt: Date.now() - 86400000 * 5,
  },
  ...BULK,
]

const SEARCH_UNIVERSE = [
  { symbol: 'NVDA', name: 'NVIDIA Corporation' },
  { symbol: 'ASML', name: 'ASML Holding N.V.' },
  { symbol: 'KO', name: 'The Coca-Cola Company' },
  { symbol: 'PLTR', name: 'Palantir Technologies Inc.' },
  { symbol: 'AAPL', name: 'Apple Inc.' },
  { symbol: 'MSFT', name: 'Microsoft Corporation' },
  { symbol: 'GOOGL', name: 'Alphabet Inc.' },
  { symbol: 'AMZN', name: 'Amazon.com, Inc.' },
  { symbol: 'TSLA', name: 'Tesla, Inc.' },
  { symbol: 'BAYN.DE', name: 'Bayer AG' },
  { symbol: 'NESN.SW', name: 'Nestlé S.A.' },
  { symbol: 'BRK-B', name: 'Berkshire Hathaway Inc.' },
]

// Symbols added from the search box are not in SEED; synthesise indicators from
// the symbol so the table has something to sort on instead of a row of dashes.
function synthIndicators(symbol: string): Record<string, unknown> {
  let h = 0
  for (const c of symbol) h = (h * 31 + c.charCodeAt(0)) % 9973
  const r = (min: number, max: number, salt: number) => min + (((h * (salt + 7)) % 1000) / 1000) * (max - min)
  const price = r(20, 400, 1)
  return {
    symbol, currency: 'USD', quantity: 0, avg_cost: 0, total_cost: 0,
    current_price: price, market_value: 0, unrealized_pnl: 0, unrealized_pct_omitempty: 0,
    week_52_low: price * 0.7, week_52_high: price * 1.4,
    pe: r(8, 60, 2), forward_pe: r(7, 45, 3),
    today_return: r(-6, 6, 11), one_week_return: r(-12, 12, 12), one_month_return: r(-18, 18, 13),
    ytd_return: r(-30, 60, 4), three_year_return: r(-40, 200, 5), five_year_return: r(-50, 400, 6),
    ten_year_return: r(-20, 900, 14), market_cap: r(1e9, 3e12, 15),
    fcf: r(-2e9, 4e10, 7), ev_to_ebitda: r(6, 40, 8),
    debt_to_equity: r(0, 200, 9), cash_flow_quality: r(0.4, 1.8, 10),
    health_rating: ['strong', 'moderate', 'weak'][h % 3],
    valuation_rating: ['cheap', 'fair', 'expensive'][h % 3],
  }
}

// Every code returns its own list, seeded on first use so the table is never
// empty in dev. Nothing persists across a dev-server restart.
const stores = new Map<string, MockEntry[]>()

function storeFor(code: string): MockEntry[] {
  let s = stores.get(code)
  if (!s) {
    s = SEED.map(e => ({ ...e }))
    stores.set(code, s)
  }
  return s
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(body))
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise(resolve => {
    let data = ''
    req.on('data', c => (data += c))
    req.on('end', () => resolve(data))
  })
}

export function mockApi(): Plugin | false {
  if (!process.env.MOCK_API) return false
  return {
    name: 'mock-watchlist-api',
    // Without a stored code the Watchlist tab shows its create-a-code gate and
    // no mock data. Seed one in the browser so the tab lands straight on the
    // seeded list; the gate is still reachable via "Forget code on this device".
    transformIndexHtml(html) {
      return html.replace(
        '</head>',
        `<script>try { localStorage.getItem('bs.portfolioCode') || localStorage.setItem('bs.portfolioCode', 'DEVMOCK') } catch (e) {}</script></head>`
      )
    },
    configureServer(server) {
      server.middlewares.use('/api/watchlist/new', (_req, res) => {
        const code = Math.random().toString(36).slice(2, 8).toUpperCase()
        storeFor(code)
        json(res, 200, { code })
      })

      server.middlewares.use('/api/search', (req, res) => {
        const q = (new URL(req.url ?? '', 'http://x').searchParams.get('q') ?? '').toLowerCase()
        const hits = SEARCH_UNIVERSE.filter(
          t => t.symbol.toLowerCase().includes(q) || t.name.toLowerCase().includes(q)
        ).slice(0, 8)
        json(res, 200, hits)
      })

      server.middlewares.use('/api/watchlist', async (req, res) => {
        const code = req.headers['x-portfolio-code']
        if (typeof code !== 'string' || !code) {
          res.statusCode = 404
          res.end()
          return
        }
        const items = storeFor(code)

        if (req.method === 'GET') {
          json(res, 200, { items })
          return
        }

        if (req.method === 'PUT') {
          const patch = JSON.parse((await readBody(req)) || '{}') as Partial<MockEntry> & { symbol: string }
          const existing = items.find(i => i.symbol === patch.symbol)
          if (existing) {
            Object.assign(existing, patch)
          } else {
            const seeded = SEED.find(e => e.symbol === patch.symbol)
            items.push({
              symbol: patch.symbol,
              note: patch.note ?? '',
              targetPrice: patch.targetPrice ?? 0,
              pinned: patch.pinned ?? false,
              addedAt: Date.now(),
              indicators: seeded?.indicators ?? synthIndicators(patch.symbol),
            })
          }
          res.statusCode = 204
          res.end()
          return
        }

        if (req.method === 'DELETE') {
          const symbol = new URL(req.url ?? '', 'http://x').searchParams.get('symbol')
          const idx = items.findIndex(i => i.symbol === symbol)
          if (idx >= 0) items.splice(idx, 1)
          res.statusCode = 204
          res.end()
          return
        }

        res.statusCode = 405
        res.end()
      })
    },
  }
}
