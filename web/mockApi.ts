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


// Yahoo's average analyst recommendation. It arrives as "2.2 - Buy" sometimes
// and a bare "Buy" the rest of the time, and is empty for symbols no analyst
// covers — roughly one in ten. All three shapes are reproduced here.
function analystFor(symbol: string): { analyst_score?: number; analyst_rating?: string } {
  let h = 0
  for (const c of symbol) h = (h * 31 + c.charCodeAt(0)) % 9973
  const shape = h % 10
  if (shape === 0) return {}
  const rating = ['Strong Buy', 'Buy', 'Hold', 'Underperform', 'Sell'][Math.floor(h / 10) % 5]
  // Yahoo sends "2.2 - Buy" sometimes and a bare "Buy" the rest of the time, so
  // the mock produces both — the label has to carry the colour on its own.
  if (shape === 1 || shape === 2) return { analyst_rating: rating, analyst_score: Number((1 + ((h * 13) % 3800) / 1000).toFixed(1)) }
  return { analyst_rating: rating }
}

// Yahoo's sector names, as the real feed returns them. ETFs are deliberately
// absent: they have no sector upstream, and dev needs the "—" path visible.
const SECTOR_BY_SYMBOL: Record<string, string> = {
  AAPL: 'Technology', MSFT: 'Technology', AVGO: 'Technology', AMD: 'Technology', TSM: 'Technology',
  NVDA: 'Technology', ASML: 'Technology', PLTR: 'Technology', 'SAP.DE': 'Technology', SHOP: 'Technology',
  GOOGL: 'Communication Services', META: 'Communication Services', NFLX: 'Communication Services',
  AMZN: 'Consumer Cyclical', TSLA: 'Consumer Cyclical', 'LVMH.PA': 'Consumer Cyclical', UBER: 'Consumer Cyclical',
  COST: 'Consumer Defensive', KO: 'Consumer Defensive', 'NESN.SW': 'Consumer Defensive',
  V: 'Financial Services', MA: 'Financial Services', 'BRK-B': 'Financial Services',
  JNJ: 'Healthcare', UNH: 'Healthcare', 'NOVO-B.CO': 'Healthcare', 'BAYN.DE': 'Healthcare',
}

// Sector aggregates: weighted P/E, weighted EV/EBITDA, peers behind them.
// Communication Services sits under the backend's four-peer floor on purpose, so
// the suppressed-comparison path shows up locally instead of only in production.
const SECTOR_AGGREGATE: Record<string, { pe: number; ev: number; peers: number }> = {
  Technology: { pe: 43.9, ev: 24.1, peers: 10 },
  'Communication Services': { pe: 24.6, ev: 13.2, peers: 3 },
  'Consumer Cyclical': { pe: 27.2, ev: 16.4, peers: 9 },
  'Consumer Defensive': { pe: 22.8, ev: 14.0, peers: 8 },
  'Financial Services': { pe: 11.2, ev: 0, peers: 10 },
  Healthcare: { pe: 8.7, ev: 9.4, peers: 10 },
  Energy: { pe: 13.6, ev: 6.9, peers: 7 },
  Industrials: { pe: 21.7, ev: 13.8, peers: 9 },
}

const SECTOR_NAMES = Object.keys(SECTOR_AGGREGATE)

// withSector derives the sector columns from the P/E and EV/EBITDA an entry
// already has, so "vs Sector" is always arithmetically consistent with the
// number beside it rather than an independently invented percentage.
function withSector(ind: Record<string, unknown>): Record<string, unknown> {
  const symbol = String(ind.symbol ?? '')
  let sector = SECTOR_BY_SYMBOL[symbol]
  if (!sector) {
    // Unknown symbols still get a sector, picked deterministically so a given
    // symbol always lands in the same one across reloads.
    let h = 0
    for (const c of symbol) h = (h * 31 + c.charCodeAt(0)) % 9973
    sector = SECTOR_NAMES[h % SECTOR_NAMES.length]
  }
  const agg = SECTOR_AGGREGATE[sector]
  const out: Record<string, unknown> = { ...ind, sector, ...analystFor(symbol) }
  if (!agg || agg.peers < 4) return out

  out.sector_peer_count = agg.peers
  const pe = ind.pe as number | undefined
  if (agg.pe > 0) {
    out.sector_pe = agg.pe
    if (pe != null && pe > 0) out.pe_vs_sector = Number((((pe - agg.pe) / agg.pe) * 100).toFixed(1))
  }
  const ev = ind.ev_to_ebitda as number | undefined
  if (agg.ev > 0) {
    out.sector_ev_to_ebitda = agg.ev
    if (ev != null && ev > 0) out.ev_to_ebitda_vs_sector = Number((((ev - agg.ev) / agg.ev) * 100).toFixed(1))
  }
  return out
}

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
      health_rating: 'healthy', valuation_rating: 'overvalued',
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
      health_rating: 'healthy', valuation_rating: 'fair',
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
      health_rating: 'fair', valuation_rating: 'fair',
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
      health_rating: 'healthy', valuation_rating: 'overvalued',
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
].map(e => (e.indicators ? { ...e, indicators: withSector(e.indicators) } : e))

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
  const base: Record<string, unknown> = {
    symbol, currency: 'USD', quantity: 0, avg_cost: 0, total_cost: 0,
    current_price: price, market_value: 0, unrealized_pnl: 0, unrealized_pct_omitempty: 0,
    week_52_low: price * 0.7, week_52_high: price * 1.4,
    pe: r(8, 60, 2), forward_pe: r(7, 45, 3),
    today_return: r(-6, 6, 11), one_week_return: r(-12, 12, 12), one_month_return: r(-18, 18, 13),
    ytd_return: r(-30, 60, 4), three_year_return: r(-40, 200, 5), five_year_return: r(-50, 400, 6),
    ten_year_return: r(-20, 900, 14), market_cap: r(1e9, 3e12, 15),
    fcf: r(-2e9, 4e10, 7), ev_to_ebitda: r(6, 40, 8),
    debt_to_equity: r(0, 200, 9), cash_flow_quality: r(0.4, 1.8, 10),
    health_rating: ['healthy', 'fair', 'weak', 'unhealthy'][h % 4],
    health_reason: 'Synthesised for dev — not a real assessment.',
    valuation_rating: ['undervalued', 'fair', 'overvalued'][h % 3],
    valuation_reason: 'Synthesised for dev — not a real assessment.',
    price_to_sales: r(1, 18, 16), price_to_book: r(1, 14, 17),
    profit_margin: r(-5, 35, 18), operating_margin: r(-5, 40, 19),
    quarterly_earnings_growth: r(-20, 60, 20), quarterly_revenue_growth: r(-10, 40, 21),
    cash: r(1e8, 5e10, 22), debt: r(0, 4e10, 23),
    dividend_yield: h % 3 === 0 ? 0 : r(0.2, 4.5, 24),
    payout_ratio: h % 3 === 0 ? 0 : r(10, 80, 25),
    payout_date: h % 3 === 0 ? undefined : new Date(Date.now() + 86400000 * (h % 90)).toISOString().slice(0, 10),
  }
  base.net = (base.cash as number) - (base.debt as number)
  return withSector(base)
}

// synthCandles builds a deterministic-enough OHLC series for the lookup chart,
// so switching ranges in dev always has something to draw instead of a blank
// canvas. Point count roughly matches what the real range/interval would return.
function synthCandles(symbol: string, range: string, interval: string): { candles: { t: number; o: number; h: number; l: number; c: number; v: number }[]; ma: (number | null)[] } {
  let h = 0
  for (const c of symbol) h = (h * 31 + c.charCodeAt(0)) % 9973
  const points: Record<string, number> = { '1mo': 22, '6mo': 130, '1y': 252, '5y': 260, '10y': 520, max: 180 }
  const n = points[range] ?? 200
  const stepMs = interval === '1d' ? 86400000 : interval === '1wk' ? 86400000 * 7 : 86400000 * 30
  const end = Date.now()
  let price = 20 + (h % 380)
  const candles = []
  for (let i = n - 1; i >= 0; i--) {
    const drift = Math.sin((h + i) / 9) * 0.01
    const noise = (Math.sin((h * 7 + i * 13) % 97) + Math.sin((h * 3 + i * 29) % 61)) * 0.015
    price = Math.max(1, price * (1 + drift + noise))
    const o = price * (1 - 0.004)
    const c = price
    const hi = Math.max(o, c) * 1.006
    const lo = Math.min(o, c) * 0.994
    candles.push({ t: end - i * stepMs, o, h: hi, l: lo, c, v: 1_000_000 + (h % 5_000_000) })
  }
  const ma = candles.map((_, i) => {
    if (i < 20) return null
    let sum = 0
    for (let j = i - 20; j <= i; j++) sum += candles[j].c
    return sum / 21
  })
  return { candles, ma }
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

      server.middlewares.use('/api/ticker', (req, res) => {
        const symbol = decodeURIComponent((req.url ?? '').split('?')[0].replace(/^\//, ''))
        if (!symbol) {
          res.statusCode = 404
          res.end()
          return
        }
        const seeded = SEED.find(e => e.symbol === symbol)
        json(res, 200, seeded?.indicators ?? synthIndicators(symbol))
      })

      server.middlewares.use('/api/history', (req, res) => {
        const [path, qs] = (req.url ?? '').split('?')
        const symbol = decodeURIComponent(path.replace(/^\//, ''))
        if (!symbol) {
          res.statusCode = 404
          res.end()
          return
        }
        const params = new URLSearchParams(qs ?? '')
        json(res, 200, synthCandles(symbol, params.get('range') ?? '1y', params.get('interval') ?? '1d'))
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
