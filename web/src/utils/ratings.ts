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
  unclear: '#555555',
}

export function ratingLabel(v: string): string {
  return v.charAt(0).toUpperCase() + v.slice(1)
}
