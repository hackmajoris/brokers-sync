import { mapPosition } from './portfolioService'
import type { RawPosition } from '../types/portfolio'
import type { WatchlistItem } from './watchlistService'

// Sample list shown to a visitor without a portfolio code, so the first screen
// demonstrates the watchlist instead of describing it. The figures are fixed
// and illustrative, not live quotes. They are hand-picked to show every state
// at once: pinned rows, a price already under its target, rows far from target,
// and a spread of health, valuation and analyst ratings.

const DAY = 86_400_000

type Indicators = Omit<RawPosition, 'currency' | 'quantity' | 'avg_cost' | 'total_cost' | 'market_value' | 'unrealized_pnl' | 'unrealized_pct_omitempty'> & { currency?: string }

function raw(ind: Indicators): RawPosition {
  return {
    currency: 'USD', quantity: 0, avg_cost: 0, total_cost: 0, market_value: 0, unrealized_pnl: 0, unrealized_pct_omitempty: 0,
    health_reason: 'Sample data for the demo.',
    valuation_reason: 'Sample data for the demo.',
    ...ind,
  }
}

const SAMPLE: { symbol: string; note: string; targetPrice: number; pinned: boolean; ageDays: number; indicators: RawPosition }[] = [
  {
    symbol: 'NVDA', note: 'Wait for a pullback after earnings', targetPrice: 150, pinned: true, ageDays: 12,
    indicators: raw({
      symbol: 'NVDA', current_price: 181.36, week_52_low: 86.62, week_52_high: 212.19, pe: 48.2, forward_pe: 31.4,
      today_return: 2.91, one_week_return: 6.2, one_month_return: 12.4, ytd_return: 34.8, three_year_return: 612.4, five_year_return: 1284.1, ten_year_return: 9840.5,
      market_cap: 4.5e12, fcf: 7.21e10, ev_to_ebitda: 42.6, debt_to_equity: 12.9, cash_flow_quality: 1.08,
      analyst_rating: 'Strong Buy', analyst_score: 1.4, sector: 'Technology', sector_pe: 43.9, pe_vs_sector: 9.8, sector_peer_count: 10,
      health_rating: 'healthy', valuation_rating: 'overvalued',
    }),
  },
  {
    symbol: 'ASML', note: 'EUV monopoly, buy near the cycle low', targetPrice: 620, pinned: true, ageDays: 30,
    indicators: raw({
      symbol: 'ASML', currency: 'EUR', current_price: 704.2, week_52_low: 578.5, week_52_high: 1035, pe: 33.1, forward_pe: 26.7,
      today_return: -1.14, one_week_return: -3.6, one_month_return: 2.1, ytd_return: -8.4, three_year_return: 41.2, five_year_return: 168.9, ten_year_return: 812.4,
      market_cap: 2.92e11, fcf: 9.4e9, ev_to_ebitda: 24.3, debt_to_equity: 28.4, cash_flow_quality: 0.94,
      analyst_rating: 'Buy', analyst_score: 1.9, sector: 'Technology', sector_pe: 43.9, pe_vs_sector: -24.6, sector_peer_count: 10,
      health_rating: 'healthy', valuation_rating: 'fair',
    }),
  },
  {
    symbol: 'NOVO-B.CO', note: 'Sentiment washed out, under target', targetPrice: 420, pinned: false, ageDays: 45,
    indicators: raw({
      symbol: 'NOVO-B.CO', currency: 'DKK', current_price: 395.4, week_52_low: 371.2, week_52_high: 1020.5, pe: 14.2, forward_pe: 12.8,
      today_return: 0.86, one_week_return: -2.4, one_month_return: -9.1, ytd_return: -38.2, three_year_return: -24.7, five_year_return: 61.3, ten_year_return: 118.9,
      market_cap: 1.76e12, fcf: 2.1e10, ev_to_ebitda: 10.4, debt_to_equity: 72.1, cash_flow_quality: 1.12, dividend_yield: 2.9, payout_ratio: 49,
      analyst_rating: 'Buy', analyst_score: 2.1, sector: 'Healthcare', sector_pe: 8.7, pe_vs_sector: 63.2, sector_peer_count: 10,
      health_rating: 'healthy', valuation_rating: 'undervalued',
    }),
  },
  {
    symbol: 'MSFT', note: 'Waiting on cloud margins', targetPrice: 380, pinned: false, ageDays: 20,
    indicators: raw({
      symbol: 'MSFT', current_price: 412.6, week_52_low: 344.8, week_52_high: 468.4, pe: 34.6, forward_pe: 29.8,
      today_return: 0.42, one_week_return: 1.8, one_month_return: 3.4, ytd_return: 9.7, three_year_return: 78.2, five_year_return: 142.6, ten_year_return: 812.1,
      market_cap: 3.07e12, fcf: 7.4e10, ev_to_ebitda: 22.1, debt_to_equity: 33.6, cash_flow_quality: 1.21, dividend_yield: 0.8, payout_ratio: 25,
      analyst_rating: 'Strong Buy', analyst_score: 1.5, sector: 'Technology', sector_pe: 43.9, pe_vs_sector: -21.2, sector_peer_count: 10,
      health_rating: 'healthy', valuation_rating: 'fair',
    }),
  },
  {
    symbol: 'KO', note: 'Dividend anchor if it dips', targetPrice: 66, pinned: false, ageDays: 3,
    indicators: raw({
      symbol: 'KO', current_price: 68.9, week_52_low: 60.6, week_52_high: 74.4, pe: 24.8, forward_pe: 22.1,
      today_return: -0.31, one_week_return: 1.1, one_month_return: -0.8, ytd_return: 6.1, three_year_return: 18.7, five_year_return: 44.3, ten_year_return: 96.3,
      market_cap: 2.64e11, fcf: 9.8e9, ev_to_ebitda: 19.2, debt_to_equity: 168.5, cash_flow_quality: 0.87, dividend_yield: 2.9, payout_ratio: 68,
      analyst_rating: 'Hold', analyst_score: 2.6, sector: 'Consumer Defensive', sector_pe: 22.8, pe_vs_sector: 8.8, sector_peer_count: 8,
      health_rating: 'fair', valuation_rating: 'fair',
    }),
  },
  {
    symbol: 'PLTR', note: 'Too rich for now, tracking only', targetPrice: 45, pinned: false, ageDays: 60,
    indicators: raw({
      symbol: 'PLTR', current_price: 172.4, week_52_low: 66.1, week_52_high: 190, pe: 486.3, forward_pe: 214.7,
      today_return: -4.62, one_week_return: -11.4, one_month_return: 18.7, ytd_return: 128.6, three_year_return: 1420.5, five_year_return: 1690.2,
      market_cap: 3.78e11, fcf: 1.6e9, ev_to_ebitda: 312.4, debt_to_equity: 4.7, cash_flow_quality: 1.42,
      analyst_rating: 'Underperform', analyst_score: 3.6, sector: 'Technology', sector_pe: 43.9, pe_vs_sector: 1007.7, sector_peer_count: 10,
      health_rating: 'healthy', valuation_rating: 'overvalued',
    }),
  },
]

// demoItems returns a fresh copy each call, so demo edits made in one visit
// never leak into the next.
export function demoItems(): WatchlistItem[] {
  const now = Date.now()
  return SAMPLE.map(s => ({
    symbol: s.symbol,
    note: s.note,
    targetPrice: s.targetPrice,
    pinned: s.pinned,
    addedAt: now - s.ageDays * DAY,
    indicators: mapPosition(s.indicators),
  }))
}
