const CODE_KEY = 'bs.portfolioCode'

export interface WatchlistItem {
  symbol: string
  note: string
  targetPrice: number
  addedAt: number
}

// InvalidCodeError means the stored code is gone or was never valid. The server
// deliberately cannot tell those apart, so neither can we.
export class InvalidCodeError extends Error {
  constructor() {
    super('Portfolio code not recognised')
  }
}

export function loadCode(): string | null {
  return localStorage.getItem(CODE_KEY)
}

export function saveCode(code: string): void {
  localStorage.setItem(CODE_KEY, code)
}

export function clearCode(): void {
  localStorage.removeItem(CODE_KEY)
}

// watchlistFetch sends the code as a header. It must never go in the URL, where
// it would leak into CloudFront and API Gateway access logs, browser history
// and Referer headers on outbound links.
async function watchlistFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const code = loadCode()
  if (!code) throw new InvalidCodeError()

  const res = await fetch(path, {
    ...init,
    headers: { ...init.headers, 'X-Portfolio-Code': code },
  })
  if (res.status === 404) throw new InvalidCodeError()
  return res
}

export async function createCode(): Promise<string> {
  const res = await fetch('/api/watchlist/new', { method: 'POST' })
  if (!res.ok) throw new Error(`Could not create a code (${res.status})`)
  const ct = res.headers.get('content-type') ?? ''
  if (!ct.includes('application/json')) throw new Error('Watchlist unavailable')
  const body = (await res.json()) as { code: string }
  return body.code
}

export async function listWatchlist(): Promise<WatchlistItem[]> {
  const res = await watchlistFetch('/api/watchlist')
  if (!res.ok) throw new Error(`Could not load watchlist (${res.status})`)
  const body = (await res.json()) as { items: WatchlistItem[] | null }
  return body.items ?? []
}

export async function upsertWatchlist(item: Partial<WatchlistItem> & { symbol: string }): Promise<void> {
  const res = await watchlistFetch('/api/watchlist', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(item),
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `Could not save (${res.status})`)
}

export async function removeWatchlist(symbol: string): Promise<void> {
  const res = await watchlistFetch(`/api/watchlist?symbol=${encodeURIComponent(symbol)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(`Could not remove ${symbol} (${res.status})`)
}
