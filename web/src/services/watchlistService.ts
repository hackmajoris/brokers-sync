const CODE_KEY = 'bs.portfolioCode'
const TRACKED_KEY = 'bs.trackedSymbols'
const CHANGED_EVENT = 'watchlist-changed'

import { mapPosition } from './portfolioService'
import type { Position, RawPosition } from '../types/portfolio'

export interface WatchlistItem {
  symbol: string
  note: string
  targetPrice: number
  pinned: boolean
  addedAt: number
  // Live indicators, fetched server-side in the same request. Absent when the
  // symbol returned no upstream data.
  indicators?: Position
}

interface RawWatchlistEntry {
  symbol: string
  note: string
  targetPrice: number
  pinned: boolean
  addedAt: number
  indicators?: RawPosition
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
  localStorage.removeItem(TRACKED_KEY)
}

// A new symbol starts with a buy target 20% below the current price, which is a
// usable starting point to edit rather than an empty field.
const DEFAULT_TARGET_DISCOUNT = 0.8

export function defaultTarget(price: number): number {
  return Number((price * DEFAULT_TARGET_DISCOUNT).toFixed(2))
}

// Which symbols are tracked, cached locally so a caller outside the watchlist
// tab can tell without calling listWatchlist — that request re-fetches upstream
// indicators for every symbol, far too expensive to answer "is this one on the
// list?". Symbol names only; everything else still comes from the server.
export function loadTracked(): string[] {
  try {
    const raw = localStorage.getItem(TRACKED_KEY)
    return raw ? (JSON.parse(raw) as string[]) : []
  } catch {
    return []
  }
}

export function saveTracked(symbols: string[]): void {
  localStorage.setItem(TRACKED_KEY, JSON.stringify(symbols))
}

// Fired after any write, so a list rendered elsewhere can reload itself rather
// than showing a symbol the user just added from the lookup modal as missing.
export function onWatchlistChanged(fn: () => void): () => void {
  window.addEventListener(CHANGED_EVENT, fn)
  return () => window.removeEventListener(CHANGED_EVENT, fn)
}

function announceChange(): void {
  window.dispatchEvent(new CustomEvent(CHANGED_EVENT))
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
  // HTML back from an /api/ call means the request never reached the API:
  // CloudFront rewrites 404s to the SPA index page, so a code the server
  // rejected arrives here as a 200 full of markup. Without this the caller
  // fails on JSON.parse instead, the bad code is never cleared, and every
  // reload repeats it. Writes reply 204 with no body, so only HTML is checked.
  const ct = res.headers.get('content-type') ?? ''
  if (res.status === 404 || ct.includes('text/html')) throw new InvalidCodeError()
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
  const body = (await res.json()) as { items: RawWatchlistEntry[] | null }
  return (body.items ?? []).map(e => ({
    symbol: e.symbol,
    note: e.note,
    targetPrice: e.targetPrice,
    pinned: e.pinned ?? false,
    addedAt: e.addedAt,
    indicators: e.indicators ? mapPosition(e.indicators) : undefined,
  }))
}

export async function upsertWatchlist(item: Partial<WatchlistItem> & { symbol: string }): Promise<void> {
  const res = await watchlistFetch('/api/watchlist', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(item),
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `Could not save (${res.status})`)
  announceChange()
}

export async function removeWatchlist(symbol: string): Promise<void> {
  const res = await watchlistFetch(`/api/watchlist?symbol=${encodeURIComponent(symbol)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(`Could not remove ${symbol} (${res.status})`)
  announceChange()
}
