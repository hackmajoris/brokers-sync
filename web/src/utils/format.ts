export function fmt(n: number | null | undefined, decimals = 2): string {
  if (n == null) return '—'
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(n)
}

const CURRENCY_SYMBOL: Record<string, string> = {
  USD: '$', EUR: '€', GBP: '£', RON: 'lei ', GBX: 'p',
}

export function currencySymbol(cur = 'USD'): string {
  return CURRENCY_SYMBOL[cur] ?? `${cur} `
}

export function fmtK(n: number | null | undefined, cur = 'USD'): string {
  if (n == null) return '—'
  const sym = currencySymbol(cur)
  if (Math.abs(n) >= 1000) return `${sym}${(n / 1000).toFixed(1)}k`
  return `${sym}${fmt(n)}`
}

// fmtKMBT abbreviates a raw magnitude (no currency conversion applied upstream,
// so no currency symbol is prefixed — value is in the figure's native reporting units).
export function fmtKMBT(n: number | null | undefined): string {
  if (n == null) return '—'
  const abs = Math.abs(n)
  const sign = n < 0 ? '-' : ''
  if (abs >= 1e12) return `${sign}${(abs / 1e12).toFixed(2)}T`
  if (abs >= 1e9) return `${sign}${(abs / 1e9).toFixed(2)}B`
  if (abs >= 1e6) return `${sign}${(abs / 1e6).toFixed(2)}M`
  if (abs >= 1e3) return `${sign}${(abs / 1e3).toFixed(1)}K`
  return `${sign}${fmt(abs)}`
}

export function fmtCurrency(n: number | null | undefined, cur = 'USD'): string {
  if (n == null) return '—'
  return `${currencySymbol(cur)}${fmt(n)}`
}

export function fmtPct(n: number | null | undefined): string {
  if (n == null) return '—'
  return `${n >= 0 ? '+' : ''}${fmt(n)}%`
}

export function clr(n: number | null | undefined, inv = false): string {
  if (n == null) return '#c0c0c0'
  const pos = n >= 0
  if (inv) return pos ? '#f87171' : '#34d399'
  return pos ? '#34d399' : '#f87171'
}
