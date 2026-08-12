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
  transfer_in: number
  transfer_out: number
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
  week_52_low?: number
  week_52_high?: number
  pe?: number
  forward_pe?: number
  ytd_return?: number
  three_year_return?: number
  five_year_return?: number
  fcf?: number
  fcf_interpretation?: string
  ev_to_ebitda?: number
  ev_to_ebitda_interpretation?: string
  debt_to_equity?: number
  debt_to_equity_interpretation?: string
  cash_flow_quality?: number
  cash_flow_quality_interpretation?: string
  market_cap?: number
  market_cap_interpretation?: string
  price_to_sales?: number
  price_to_sales_interpretation?: string
  price_to_book?: number
  price_to_book_interpretation?: string
  fcf_yield?: number
  fcf_yield_interpretation?: string
  profit_margin?: number
  profit_margin_interpretation?: string
  operating_margin?: number
  operating_margin_interpretation?: string
  quarterly_earnings_growth?: number
  quarterly_earnings_growth_interpretation?: string
  quarterly_revenue_growth?: number
  quarterly_revenue_growth_interpretation?: string
  cash?: number
  cash_interpretation?: string
  debt?: number
  debt_interpretation?: string
  net?: number
  dividend_yield?: number
  dividend_yield_interpretation?: string
  payout_ratio?: number
  payout_ratio_interpretation?: string
  payout_date?: string
  payout_date_interpretation?: string
  health_rating?: string
  health_reason?: string
  valuation_rating?: string
  valuation_reason?: string
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
  avgCost?: number
  currentPrice?: number
  weekLow52?: number
  weekHigh52?: number
  pe?: number
  forwardPE?: number
  ytdReturn?: number
  threeYrReturn?: number
  fiveYrReturn?: number
  fcf?: number
  fcfInterpretation?: string
  evToEbitda?: number
  evToEbitdaInterpretation?: string
  debtToEquity?: number
  debtToEquityInterpretation?: string
  cashFlowQuality?: number
  cashFlowQualityInterpretation?: string
  marketCap?: number
  marketCapInterpretation?: string
  priceToSales?: number
  priceToSalesInterpretation?: string
  priceToBook?: number
  priceToBookInterpretation?: string
  fcfYield?: number
  fcfYieldInterpretation?: string
  profitMargin?: number
  profitMarginInterpretation?: string
  operatingMargin?: number
  operatingMarginInterpretation?: string
  quarterlyEarningsGrowth?: number
  quarterlyEarningsGrowthInterpretation?: string
  quarterlyRevenueGrowth?: number
  quarterlyRevenueGrowthInterpretation?: string
  cash?: number
  cashInterpretation?: string
  debt?: number
  debtInterpretation?: string
  net?: number
  dividendYield?: number
  dividendYieldInterpretation?: string
  payoutRatio?: number
  payoutRatioInterpretation?: string
  payoutDate?: string
  payoutDateInterpretation?: string
  healthRating?: string
  healthReason?: string
  valuationRating?: string
  valuationReason?: string
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
  transferIn: number
  transferOut: number
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
  transferIn: number
  transferOut: number
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
  transferIn: number
  transferOut: number
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
