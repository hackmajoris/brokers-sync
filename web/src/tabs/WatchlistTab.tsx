import { useState, useEffect, useRef } from 'react'
import { searchSymbols, type TickerSearchResult } from '../services/portfolioService'
import {
  createCode,
  listWatchlist,
  upsertWatchlist,
  removeWatchlist,
  loadCode,
  saveCode,
  clearCode,
  InvalidCodeError,
  type WatchlistItem,
} from '../services/watchlistService'
import { SectionLabel } from '../components/ui/SectionLabel'
import { InfoTooltip } from '../components/ui/InfoTooltip'
import { openStockLookup } from '../components/StockLookup'
import { IndicatorCells, INDICATOR_COLUMNS, INDICATOR_INFO, indicatorSortValue, type IndicatorKey } from '../components/IndicatorColumns'
import { fmtCurrency } from '../utils/format'

interface Props {
  accent: string
}

const MAX_NOTE = 500

type SortKey = 'symbol' | 'note' | 'target' | 'price' | IndicatorKey
type SortDir = 'asc' | 'desc'

const COLUMNS: { key: SortKey; label: string; align?: 'left' | 'right' }[] = [
  { key: 'symbol', label: 'Symbol', align: 'left' },
  { key: 'note', label: 'Note', align: 'left' },
  { key: 'target', label: 'Target' },
  { key: 'price', label: 'Price' },
  ...INDICATOR_COLUMNS,
]

function sortValue(item: WatchlistItem, key: SortKey): number | string {
  switch (key) {
    case 'symbol':
      return item.symbol
    case 'note':
      return item.note ?? ''
    case 'target':
      return item.targetPrice ?? -Infinity
    case 'price':
      return item.indicators?.currentPrice ?? -Infinity
    default:
      return indicatorSortValue(item.indicators, key)
  }
}

function sortItems(items: WatchlistItem[], key: SortKey, dir: SortDir): WatchlistItem[] {
  const mul = dir === 'asc' ? 1 : -1
  return items.slice().sort((a, b) => {
    const va = sortValue(a, key)
    const vb = sortValue(b, key)
    if (typeof va === 'string' || typeof vb === 'string') {
      return mul * String(va).localeCompare(String(vb))
    }
    return mul * (va - vb)
  })
}

export function WatchlistTab({ accent }: Props) {
  const [code, setCode] = useState<string | null>(loadCode())
  const [items, setItems] = useState<WatchlistItem[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  // freshCode is shown once, right after creation, and never again.
  const [freshCode, setFreshCode] = useState<string | null>(null)
  const [entryCode, setEntryCode] = useState('')
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<TickerSearchResult[]>([])
  const [copied, setCopied] = useState(false)
  const [sortKey, setSortKey] = useState<SortKey>('symbol')
  const [sortDir, setSortDir] = useState<SortDir>('asc')
  const searchTimer = useRef<number | undefined>(undefined)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems(await listWatchlist())
    } catch (e) {
      if (e instanceof InvalidCodeError) {
        clearCode()
        setCode(null)
        setItems([])
        setError('That code was not recognised.')
      } else {
        setError(e instanceof Error ? e.message : String(e))
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (code) void refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [code])

  useEffect(() => {
    if (query.trim().length < 2) {
      setResults([])
      return
    }
    window.clearTimeout(searchTimer.current)
    const ctrl = new AbortController()
    searchTimer.current = window.setTimeout(async () => {
      try {
        setResults(await searchSymbols(query.trim(), ctrl.signal))
      } catch {
        setResults([])
      }
    }, 250)
    return () => ctrl.abort()
  }, [query])

  async function handleCreate() {
    setError(null)
    try {
      const created = await createCode()
      saveCode(created)
      setFreshCode(created)
      setCode(created)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  function handleUseExisting() {
    const trimmed = entryCode.trim()
    if (!trimmed) return
    saveCode(trimmed)
    setCode(trimmed)
    setEntryCode('')
  }

  async function handleAdd(symbol: string) {
    setQuery('')
    setResults([])
    try {
      await upsertWatchlist({ symbol })
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  // Note and target edits patch local state instead of calling refresh(): a
  // reload re-fetches indicators for every symbol upstream, which is far too
  // expensive for a keystroke-level edit. Adding or removing a symbol does
  // refresh, since new indicators are genuinely needed.
  async function handleSave(item: WatchlistItem, patch: Partial<WatchlistItem>) {
    try {
      await upsertWatchlist({ symbol: item.symbol, note: item.note, targetPrice: item.targetPrice, ...patch })
      setItems(prev => prev.map(i => (i.symbol === item.symbol ? { ...i, ...patch } : i)))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function handleRemove(symbol: string) {
    try {
      await removeWatchlist(symbol)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir(key === 'symbol' || key === 'note' ? 'asc' : 'desc')
    }
  }

  function handleForget() {
    clearCode()
    setCode(null)
    setItems([])
    setFreshCode(null)
  }

  if (!code) {
    return (
      <div style={{ maxWidth: 520, margin: '0 auto', display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', gap: 18 }}>
        <SectionLabel>Watchlist</SectionLabel>
        <p style={{ fontSize: 12, color: '#888888', lineHeight: 1.6, margin: 0 }}>
          Track companies you do not own yet. There are no accounts — a portfolio code is
          the only thing that identifies your list, and anyone holding it can read and
          edit that list.
        </p>
        {error && <ErrorLine text={error} />}
        <button onClick={handleCreate} style={primaryBtn(accent)}>
          Create a portfolio code
        </button>
        <div style={{ display: 'flex', gap: 8, width: '100%' }}>
          <input
            value={entryCode}
            onChange={e => setEntryCode(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleUseExisting()}
            placeholder="Already have one? Paste it here"
            style={{ ...inputStyle, flex: 1 }}
          />
          <button onClick={handleUseExisting} style={secondaryBtn}>
            Use
          </button>
        </div>
      </div>
    )
  }

  return (
    <div style={{ maxWidth: items.length > 0 ? 1300 : 820, margin: '0 auto', width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
      <SectionLabel>Watchlist</SectionLabel>

      {freshCode && (
        <div style={{ width: '100%', border: `1px solid ${accent}55`, background: accent + '11', borderRadius: 8, padding: 14, display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', gap: 10 }}>
          <span style={{ fontSize: 10, fontWeight: 600, color: accent, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
            Save this code now
          </span>
          <code style={{ fontSize: 18, letterSpacing: '0.08em', color: '#e8e8e8' }}>{freshCode}</code>
          <span style={{ fontSize: 11, color: '#999999', lineHeight: 1.6 }}>
            This is the only way back to your watchlist. It cannot be recovered or reset —
            if you lose it, the list is gone. It is shown once and will not be displayed again.
          </span>
          <div style={{ display: 'flex', gap: 8 }}>
            <button
              onClick={() => {
                void navigator.clipboard.writeText(freshCode)
                setCopied(true)
              }}
              style={primaryBtn(accent)}
            >
              {copied ? 'Copied' : 'Copy code'}
            </button>
            <button onClick={() => setFreshCode(null)} style={secondaryBtn}>
              I have saved it
            </button>
          </div>
        </div>
      )}

      <div style={{ position: 'relative', width: '100%', maxWidth: 420 }}>
        <input
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search a symbol to add…"
          style={{ ...inputStyle, width: '100%' }}
        />
        {results.length > 0 && (
          <div style={{ position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 20, background: '#0a0a0a', border: '1px solid #1f1f1f', borderRadius: 7, marginTop: 4, maxHeight: 260, overflowY: 'auto' }}>
            {results.map(r => (
              <button
                key={r.symbol}
                onClick={() => handleAdd(r.symbol)}
                style={{ display: 'flex', justifyContent: 'space-between', gap: 10, width: '100%', padding: '8px 10px', background: 'transparent', border: 'none', color: '#d0d0d0', fontSize: 12, cursor: 'pointer', textAlign: 'left' }}
              >
                <span style={{ fontWeight: 600 }}>{r.symbol}</span>
                <span style={{ color: '#777777', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.name}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {error && <ErrorLine text={error} />}

      {loading && items.length === 0 ? (
        <span style={{ fontSize: 12, color: '#666666' }}>Loading…</span>
      ) : items.length === 0 ? (
        <span style={{ fontSize: 12, color: '#666666' }}>Nothing tracked yet.</span>
      ) : (
        <div style={{ width: '100%', overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12, textAlign: 'left' }}>
          <thead>
            <tr style={{ color: '#666666', textAlign: 'left', background: '#000000' }}>
              {COLUMNS.map(col => (
                <th
                  key={col.key}
                  onClick={() => handleSort(col.key)}
                  style={{
                    ...th,
                    textAlign: col.align ?? 'right',
                    whiteSpace: 'nowrap',
                    cursor: 'pointer',
                    userSelect: 'none',
                    color: sortKey === col.key ? '#c0c0c0' : '#666666',
                    ...(col.key === 'symbol' ? stickyCol('#000000', 2) : {}),
                  }}
                >
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, flexDirection: col.align === 'left' ? 'row' : 'row-reverse' }}>
                    {col.label}
                    <span style={{ fontSize: 8, opacity: sortKey === col.key ? 1 : 0.25 }}>{sortKey === col.key && sortDir === 'asc' ? '▲' : '▼'}</span>
                    {INDICATOR_INFO[col.key] && <InfoTooltip text={INDICATOR_INFO[col.key]!} />}
                  </span>
                </th>
              ))}
              <th style={th} />
            </tr>
          </thead>
          <tbody>
            {sortItems(items, sortKey, sortDir).map(item => (
              <Row key={item.symbol} item={item} accent={accent} onSave={handleSave} onRemove={handleRemove} />
            ))}
          </tbody>
        </table>
        </div>
      )}

      <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, borderTop: '1px solid #161616', paddingTop: 12 }}>
        <span style={{ fontSize: 11, color: '#555555' }}>
          {items.length} tracked. Your code is stored in this browser only.
        </span>
        <button onClick={handleForget} style={secondaryBtn}>
          Forget code on this device
        </button>
      </div>
    </div>
  )
}

function Row({
  item,
  accent,
  onSave,
  onRemove,
}: {
  item: WatchlistItem
  accent: string
  onSave: (item: WatchlistItem, patch: Partial<WatchlistItem>) => void
  onRemove: (symbol: string) => void
}) {
  const [note, setNote] = useState(item.note ?? '')
  const [target, setTarget] = useState(item.targetPrice ? String(item.targetPrice) : '')

  const p = item.indicators
  // Target hit: the price has fallen below the buy target the user set, so the
  // input turns green. No target or no price means nothing to compare.
  const hit = item.targetPrice > 0 && p?.currentPrice != null && p.currentPrice < item.targetPrice

  return (
    // Clicking the row opens the same lookup modal the Positions tab uses.
    // The inputs and the remove button stop propagation so editing a note does
    // not also open the popup.
    <tr
      onClick={() => openStockLookup(item.symbol)}
      style={{ borderTop: '1px solid #161616', cursor: 'pointer' }}
      onMouseEnter={e => {
        e.currentTarget.style.background = accent + '11'
        ;(e.currentTarget.firstElementChild as HTMLElement).style.background = '#111111'
      }}
      onMouseLeave={e => {
        e.currentTarget.style.background = 'transparent'
        ;(e.currentTarget.firstElementChild as HTMLElement).style.background = '#000000'
      }}
    >
      <td style={{ ...td, fontWeight: 600, color: accent, whiteSpace: 'nowrap', ...stickyCol('#000000', 1) }}>{item.symbol}</td>
      <td style={td} onClick={e => e.stopPropagation()}>
        <input
          value={note}
          maxLength={MAX_NOTE}
          onChange={e => setNote(e.target.value)}
          onBlur={() => note !== (item.note ?? '') && onSave(item, { note })}
          placeholder="Add a note"
          style={{ ...inputStyle, width: '100%', minWidth: 140 }}
        />
      </td>
      <td style={{ ...td, textAlign: 'right' }} onClick={e => e.stopPropagation()}>
        <input
          value={target}
          inputMode="decimal"
          onChange={e => setTarget(e.target.value)}
          onBlur={() => {
            const parsed = target === '' ? 0 : Number(target)
            if (!Number.isNaN(parsed) && parsed !== item.targetPrice) onSave(item, { targetPrice: parsed })
          }}
          placeholder="—"
          style={{
            ...inputStyle,
            width: 90,
            textAlign: 'right',
            ...(hit ? { color: '#34d399', borderColor: '#34d39955', background: '#34d39914', fontWeight: 600 } : {}),
          }}
        />
      </td>
      <td style={{ ...td, textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, whiteSpace: 'nowrap' }}>
        {p?.currentPrice != null ? fmtCurrency(p.currentPrice) : '—'}
      </td>
      {p ? (
        <IndicatorCells p={p} />
      ) : (
        // Indicators are fetched upstream and can be missing for a delisted or
        // unrecognised symbol; keep the column count stable so the row lines up.
        INDICATOR_COLUMNS.map(col => (
          <td key={col.key} style={{ ...td, textAlign: 'right', color: '#555555' }}>
            —
          </td>
        ))
      )}
      <td style={{ ...td, textAlign: 'right' }} onClick={e => e.stopPropagation()}>
        <button onClick={() => onRemove(item.symbol)} style={{ ...secondaryBtn, padding: '4px 8px' }}>
          Remove
        </button>
      </td>
    </tr>
  )
}

function ErrorLine({ text }: { text: string }) {
  return <span style={{ fontSize: 12, color: '#e06c6c' }}>{text}</span>
}

const inputStyle: React.CSSProperties = {
  background: '#0a0a0a',
  border: '1px solid #1f1f1f',
  borderRadius: 6,
  padding: '6px 9px',
  color: '#d0d0d0',
  fontSize: 12,
  outline: 'none',
}

const secondaryBtn: React.CSSProperties = {
  background: 'transparent',
  border: '1px solid #262626',
  borderRadius: 6,
  padding: '6px 12px',
  color: '#999999',
  fontSize: 11,
  cursor: 'pointer',
}

function primaryBtn(accent: string): React.CSSProperties {
  return {
    background: accent + '22',
    border: `1px solid ${accent}55`,
    borderRadius: 6,
    padding: '7px 14px',
    color: accent,
    fontSize: 12,
    fontWeight: 600,
    cursor: 'pointer',
  }
}

function stickyCol(background: string, zIndex: number): React.CSSProperties {
  return { position: 'sticky', left: 0, background, zIndex, borderRight: '1px solid #161616' }
}

const th: React.CSSProperties = { padding: '6px 8px', fontWeight: 600, fontSize: 10, textTransform: 'uppercase', letterSpacing: '0.06em' }
const td: React.CSSProperties = { padding: '6px 8px', color: '#c0c0c0' }
