// ── Raw JSON types (as produced by brokers-sync) ─────────────────────────────

export interface RawPeriod {
  label: string
  start: string
  end: string
  realized_pnl: number
  dividends_net: number
  tax_withheld: number
  fees: number
  commissions: number
  deposits: number
  withdrawals: number
  buy_volume: number
  sell_volume: number
  gain_pct: number
}

export interface RawPosition {
  symbol: string
  currency: string
  quantity: number
  avg_cost: number
  total_cost: number
  current_price: number
  market_value: number
  unrealized_pnl: number
  unrealized_pct_omitempty: number
}

export interface RawRealizedBySymbol {
  symbol: string
  pnl: number
}

export interface RawDividendBySymbol {
  symbol: string
  gross: number
  tax_withheld: number
  net: number
}

export interface RawBroker {
  name: string
  base_currency: string
  cash_balance: number
  open_positions: RawPosition[]
  realized_pnl_by_symbol: RawRealizedBySymbol[]
  all_time: RawPeriod
  ytd: RawPeriod
  mtd: RawPeriod
  by_year: RawPeriod[]
  dividends_by_symbol: RawDividendBySymbol[]
}

export interface RawPortfolio {
  generated_at: string
  base_currency: string
  cash_balance: number
  brokers: RawBroker[]
  open_positions: RawPosition[]
  realized_pnl_by_symbol: RawRealizedBySymbol[]
  all_time: RawPeriod
  ytd: RawPeriod
  mtd: RawPeriod
  by_year: RawPeriod[]
  dividends_by_symbol: RawDividendBySymbol[]
}

// ── Presentation model ────────────────────────────────────────────────────────

export interface Position {
  symbol: string
  mv: number | null
  pnl: number | null
  pct: number | null
  cost: number
  quantity?: number
  currentPrice?: number
}

export interface RealizedBySymbol {
  symbol: string
  pnl: number
}

export interface DividendBySymbol {
  symbol: string
  gross: number
  taxWithheld: number
  net: number
}

export interface YearStat {
  label: string
  gainPct: number
  rPnl: number
  divs: number
  deposits: number
  buyVol: number
  sellVol: number
}

export interface PeriodSummary {
  realizedPnl: number
  dividends: number
  taxWithheld: number
  fees: number
  deposits: number
  withdrawals: number
  gainPct: number
  buyVol: number
  sellVol: number
}

export interface BrokerData {
  name: string
  currency: string
  cashBalance: number
  allTimeGain: number
  allTimeRPnl: number
  ytdGain: number
  ytdRPnl: number
  ytdDivs: number
  mtdGain: number
  mtdRPnl: number
  deposits: number
  withdrawals: number
  dividends: number
  buyVol: number
  sellVol: number
  positions: Position[]
  realizedBySymbol: RealizedBySymbol[]
  byYear: YearStat[]
  dividendsBySymbol: DividendBySymbol[]
}

export interface PortfolioData {
  generatedAt: string
  baseCurrency: string
  cashBalance: number
  brokers: BrokerData[]
  allTime: PeriodSummary
  ytd: PeriodSummary
  mtd: PeriodSummary
  byYear: YearStat[]
  openPositions: Position[]
  topRPnl: RealizedBySymbol[]
  topDivs: DividendBySymbol[]
}
