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

export function fmtCurrency(n: number | null | undefined, cur = 'USD'): string {
  if (n == null) return '—'
  return `${currencySymbol(cur)}${fmt(n)}`
}

export function fmtPct(n: number | null | undefined): string {
  if (n == null) return '—'
  return `${n >= 0 ? '+' : ''}${fmt(n)}%`
}

export function clr(n: number | null | undefined, inv = false): string {
  if (n == null) return '#94a3b8'
  const pos = n >= 0
  if (inv) return pos ? '#f87171' : '#34d399'
  return pos ? '#34d399' : '#f87171'
}
