export const HEALTH_COLORS: Record<string, string> = {
  healthy: '#34d399',
  fair: '#fbbf24',
  weak: '#fb923c',
  unhealthy: '#f87171',
}

export const VALUATION_COLORS: Record<string, string> = {
  undervalued: '#34d399',
  fair: '#fbbf24',
  overvalued: '#f87171',
  unclear: '#8b8fa3',
}

export function ratingLabel(v: string): string {
  return v.charAt(0).toUpperCase() + v.slice(1)
}

// Yahoo usually sends the recommendation as a bare label ("Buy"), and only
// sometimes as "2.2 - Buy". The label is therefore the source of truth for both
// colour and sort order, with the score used only when it happens to be there.
const ANALYST_RANK: Record<string, number> = {
  'strong buy': 1,
  buy: 2,
  outperform: 2,
  overweight: 2,
  accumulate: 2,
  hold: 3,
  neutral: 3,
  'market perform': 3,
  underperform: 4,
  underweight: 4,
  reduce: 4,
  sell: 5,
  'strong sell': 5,
}

// 1 (Strong Buy) to 5 (Sell) — low is bullish, the opposite direction to every
// other number on the table, which is why this never goes through clr().
export function analystRank(rating?: string, score?: number): number | undefined {
  const known = rating ? ANALYST_RANK[rating.trim().toLowerCase()] : undefined
  if (known != null) return known
  return score != null && score > 0 ? score : undefined
}

export function analystColor(rating?: string, score?: number): string {
  const rank = analystRank(rating, score)
  if (rank == null) return '#8b8fa3'
  if (rank <= 1.5) return '#34d399'
  if (rank <= 2.5) return '#86efac'
  if (rank <= 3.5) return '#fbbf24'
  if (rank <= 4.5) return '#fb923c'
  return '#f87171'
}
