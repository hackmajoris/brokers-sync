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
import { fmtCurrency, fmtPct, clr } from '../utils/format'
import type { Position } from '../types/portfolio'

interface Props {
  accent: string
}

const MAX_NOTE = 500

// A new symbol starts with a buy target 20% below the current price, which is a
// usable starting point to edit rather than an empty field.
const DEFAULT_TARGET_DISCOUNT = 0.8

function defaultTarget(price: number): number {
  return Number((price * DEFAULT_TARGET_DISCOUNT).toFixed(2))
}

type SortKey = 'symbol' | 'note' | 'target' | 'targetGap' | 'price' | IndicatorKey
type SortDir = 'asc' | 'desc'

// Performance and the 52-week range sit ahead of the target columns; the
// valuation and health indicators follow the price.
const LEAD_INDICATORS: IndicatorKey[] = ['today', 'oneWeek', 'oneMonth', 'ytd', 'threeYr', 'fiveYr', 'tenYr', 'range']
const TRAIL_INDICATORS: IndicatorKey[] = INDICATOR_COLUMNS.map(c => c.key).filter(k => !LEAD_INDICATORS.includes(k))

const indicatorColumn = (key: IndicatorKey) => INDICATOR_COLUMNS.find(c => c.key === key)!

const COLUMNS: { key: SortKey; label: string; align?: 'left' | 'right' }[] = [
  { key: 'symbol', label: 'Symbol', align: 'left' },
  ...LEAD_INDICATORS.map(indicatorColumn),
  { key: 'target', label: 'Target Value' },
  { key: 'targetGap', label: 'Target Diff' },
  { key: 'price', label: 'Price' },
  ...TRAIL_INDICATORS.map(indicatorColumn),
  { key: 'note', label: 'Note', align: 'left' },
]

// targetGap is how far the price still has to move to reach the target, as a
// percentage of the price. Negative means it has to fall, positive means it is
// already past. null when there is no target or no price to compare.
function targetGap(item: WatchlistItem): number | null {
  const price = item.indicators?.currentPrice
  if (!(item.targetPrice > 0) || price == null || price <= 0) return null
  return ((item.targetPrice - price) / price) * 100
}

function sortValue(item: WatchlistItem, key: SortKey): number | string {
  switch (key) {
    case 'symbol':
      return item.symbol
    case 'note':
      return item.note ?? ''
    case 'target':
      return item.targetPrice ?? -Infinity
    case 'targetGap':
      return targetGap(item) ?? -Infinity
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
  // null means no column sort is active, which is the only state where pinned
  // items float to the top. Once a column is sorted the sort owns the whole
  // list, pinned or not, otherwise sorting by P/E would only ever reorder the
  // pinned block and look broken.
  const [sortKey, setSortKey] = useState<SortKey | null>(null)
  const [sortDir, setSortDir] = useState<SortDir>('asc')
  const [compare, setCompare] = useState(false)
  // Symbols selected for comparison. Deliberately in memory only — this is a
  // scratch selection, unlike item.pinned which is stored per portfolio.
  const [selected, setSelected] = useState<string[]>([])
  // In compare mode the unselected rows are hidden, so picking needs its own
  // state — otherwise there would be nothing left to click once the first
  // symbol is selected.
  const [picking, setPicking] = useState(true)
  const searchTimer = useRef<number | undefined>(undefined)

  async function refresh(): Promise<WatchlistItem[]> {
    setLoading(true)
    setError(null)
    try {
      const fetched = await listWatchlist()
      setItems(fetched)
      return fetched
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
    return []
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
      const fetched = await refresh()
      // Seed a starting buy target 20% under the current price. The refresh
      // already carries indicators, so this costs no extra upstream fetch.
      const added = fetched.find(i => i.symbol === symbol)
      const price = added?.indicators?.currentPrice
      if (added && !added.targetPrice && price != null) {
        await handleSave(added, { targetPrice: defaultTarget(price) })
      }
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
      await upsertWatchlist({ symbol: item.symbol, note: item.note, targetPrice: item.targetPrice, pinned: item.pinned, ...patch })
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

  // Clicking a column cycles through its two directions and then back to no
  // sort at all, which is how the user gets the pinned-first order back.
  function handleSort(key: SortKey) {
    if (key !== sortKey) {
      setSortKey(key)
      setSortDir(key === 'symbol' || key === 'note' ? 'asc' : 'desc')
      return
    }
    const firstDir = key === 'symbol' || key === 'note' ? 'asc' : 'desc'
    if (sortDir === firstDir) {
      setSortDir(firstDir === 'asc' ? 'desc' : 'asc')
    } else {
      setSortKey(null)
    }
  }

  function handleSelect(symbol: string) {
    setSelected(prev => (prev.includes(symbol) ? prev.filter(s => s !== symbol) : [...prev, symbol]))
  }

  function toggleCompare() {
    if (compare) {
      exitCompare()
      return
    }
    setPicking(selected.length === 0)
    setCompare(true)
  }

  // Clear is the way out of compare mode: it drops the selection and leaves,
  // rather than leaving an empty comparison the user has to dismiss twice.
  function exitCompare() {
    setSelected([])
    setCompare(false)
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

  // With a column sorted, that sort is the whole order. With no column sorted,
  // pinned items lead and the rest fall back to symbol order.
  const ordered = sortKey
    ? sortItems(items, sortKey, sortDir)
    : (() => {
        const bySymbol = sortItems(items, 'symbol', 'asc')
        return [...bySymbol.filter(i => i.pinned), ...bySymbol.filter(i => !i.pinned)]
      })()
  const visibleRows = compare
    ? [
        ...ordered.filter(i => selected.includes(i.symbol)),
        ...(picking ? ordered.filter(i => !selected.includes(i.symbol)) : []),
      ]
    : ordered

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

      {items.length > 0 && (
        <div style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 8 }}>
          <button onClick={toggleCompare} style={compare ? primaryBtn(accent) : secondaryBtn}>
            {compare ? `Comparing ${selected.length}` : 'Compare'}
          </button>
          {compare && (
            <>
              <button onClick={() => setPicking(v => !v)} style={secondaryBtn}>
                {picking ? 'Done picking' : 'Add more'}
              </button>
              <button onClick={exitCompare} style={secondaryBtn}>
                Clear
              </button>
              <span style={{ fontSize: 11, color: '#555555' }}>Click a row to add or drop it from the comparison.</span>
            </>
          )}
        </div>
      )}

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
            {visibleRows.map(item => (
              <Row
                key={item.symbol}
                item={item}
                accent={accent}
                onSave={handleSave}
                onRemove={handleRemove}
                selected={compare && selected.includes(item.symbol)}
                onSelect={compare ? handleSelect : undefined}
              />
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
  selected,
  onSelect,
}: {
  item: WatchlistItem
  accent: string
  onSave: (item: WatchlistItem, patch: Partial<WatchlistItem>) => void
  onRemove: (symbol: string) => void
  selected: boolean
  onSelect?: (symbol: string) => void
}) {
  const [note, setNote] = useState(item.note ?? '')
  const [target, setTarget] = useState(item.targetPrice ? String(item.targetPrice) : '')

  // The target can change from outside this input — adding a symbol seeds one
  // from the price after the row has already mounted — and useState keeps its
  // first value, so without this the seeded target never appears in the field.
  useEffect(() => {
    setTarget(item.targetPrice ? String(item.targetPrice) : '')
  }, [item.targetPrice])

  const p = item.indicators
  // Target hit: the price has fallen below the buy target the user set, so the
  // input turns green. No target or no price means nothing to compare.
  const hit = item.targetPrice > 0 && p?.currentPrice != null && p.currentPrice < item.targetPrice
  const gap = targetGap(item)
  // Pinned and compare-selected are separate states a row can be in at once, so
  // they get separate visual channels: pinned owns the amber edge and wash,
  // selection owns the accent wash and wins the background when both apply.
  // The first cell is sticky, so its background has to stay opaque or the
  // scrolled columns show through it.
  const restBg = selected ? accent + '0d' : item.pinned ? PIN_TINT : 'transparent'
  const stickyBg = selected ? '#121212' : item.pinned ? PIN_STICKY : '#000000'

  return (
    // Clicking the row opens the same lookup modal the Positions tab uses.
    // The inputs and the remove button stop propagation so editing a note does
    // not also open the popup.
    <tr
      onClick={() => (onSelect ? onSelect(item.symbol) : openStockLookup(item.symbol))}
      style={{ borderTop: '1px solid #161616', cursor: 'pointer', background: restBg }}
      onMouseEnter={e => {
        e.currentTarget.style.background = accent + '11'
        ;(e.currentTarget.firstElementChild as HTMLElement).style.background = '#111111'
      }}
      onMouseLeave={e => {
        e.currentTarget.style.background = restBg
        ;(e.currentTarget.firstElementChild as HTMLElement).style.background = stickyBg
      }}
    >
      <td style={{ ...td, fontWeight: 600, color: accent, whiteSpace: 'nowrap', ...stickyCol(stickyBg, 1), borderLeft: item.pinned ? `2px solid ${PIN_EDGE}` : selected ? `2px solid ${accent}` : undefined }}>
        {item.symbol}
      </td>
      <IndicatorGroup p={p} keys={LEAD_INDICATORS} />
      <td style={{ ...td, textAlign: 'right' }} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'inline-flex', alignItems: 'center', gap: 5, justifyContent: 'flex-end' }}>
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
          <button
            onClick={() => p?.currentPrice != null && onSave(item, { targetPrice: defaultTarget(p.currentPrice) })}
            disabled={p?.currentPrice == null}
            title={`Reset target to 20% below the current price`}
            aria-label={`Reset ${item.symbol} target to 20% below the current price`}
            style={{
              background: 'transparent',
              border: '1px solid #262626',
              borderRadius: 6,
              width: 24,
              height: 24,
              lineHeight: 1,
              padding: 0,
              color: '#777777',
              fontSize: 12,
              cursor: p?.currentPrice == null ? 'default' : 'pointer',
              opacity: p?.currentPrice == null ? 0.4 : 1,
            }}
            onMouseEnter={e => {
              if (p?.currentPrice == null) return
              e.currentTarget.style.color = accent
              e.currentTarget.style.borderColor = accent + '55'
            }}
            onMouseLeave={e => {
              e.currentTarget.style.color = '#777777'
              e.currentTarget.style.borderColor = '#262626'
            }}
          >
            ↻
          </button>
        </div>
      </td>
      <td
        style={{
          ...td,
          textAlign: 'right',
          fontSize: 11,
          fontWeight: 600,
          fontFamily: "'DM Mono', monospace",
          whiteSpace: 'nowrap',
          color: gap == null ? '#555555' : clr(gap),
        }}
      >
        {gap == null ? '—' : fmtPct(gap)}
      </td>
      <td style={{ ...td, textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, whiteSpace: 'nowrap' }}>
        {p?.currentPrice != null ? fmtCurrency(p.currentPrice) : '—'}
      </td>
      <IndicatorGroup p={p} keys={TRAIL_INDICATORS} />
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
        <div style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
        <button
          onClick={() => onSave(item, { pinned: !item.pinned })}
          title={item.pinned ? `Unpin ${item.symbol}` : `Pin ${item.symbol} to the top`}
          aria-label={item.pinned ? `Unpin ${item.symbol}` : `Pin ${item.symbol} to the top`}
          aria-pressed={item.pinned}
          style={{
            background: 'transparent',
            border: `1px solid ${item.pinned ? PIN_EDGE + '55' : '#262626'}`,
            borderRadius: 6,
            width: 24,
            height: 24,
            lineHeight: 1,
            padding: 0,
            color: item.pinned ? PIN_EDGE : '#777777',
            fontSize: 13,
            cursor: 'pointer',
          }}
        >
          {item.pinned ? '★' : '☆'}
        </button>
        <button
          onClick={() => onRemove(item.symbol)}
          title={`Remove ${item.symbol}`}
          aria-label={`Remove ${item.symbol}`}
          style={{
            background: 'transparent',
            border: '1px solid #262626',
            borderRadius: 6,
            width: 24,
            height: 24,
            lineHeight: 1,
            padding: 0,
            color: '#777777',
            fontSize: 14,
            cursor: 'pointer',
          }}
          onMouseEnter={e => {
            e.currentTarget.style.color = '#e06c6c'
            e.currentTarget.style.borderColor = '#e06c6c55'
          }}
          onMouseLeave={e => {
            e.currentTarget.style.color = '#777777'
            e.currentTarget.style.borderColor = '#262626'
          }}
        >
          ×
        </button>
        </div>
      </td>
    </tr>
  )
}

// Indicators are fetched upstream and can be missing for a delisted or
// unrecognised symbol; keep the column count stable so the row lines up.
function IndicatorGroup({ p, keys }: { p?: Position; keys: IndicatorKey[] }) {
  if (p) return <IndicatorCells p={p} keys={keys} />
  return (
    <>
      {keys.map(key => (
        <td key={key} style={{ ...td, textAlign: 'right', color: '#555555' }}>
          —
        </td>
      ))}
    </>
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

// Pinned rows are marked in amber rather than the portfolio accent, so a pinned
// row and a compare-selected row never read as the same thing.
const PIN_EDGE = '#d9a441'
const PIN_TINT = '#d9a4410f'
const PIN_STICKY = '#151209'

function stickyCol(background: string, zIndex: number): React.CSSProperties {
  return { position: 'sticky', left: 0, background, zIndex, borderRight: '1px solid #161616' }
}

const th: React.CSSProperties = { padding: '6px 8px', fontWeight: 600, fontSize: 10, textTransform: 'uppercase', letterSpacing: '0.06em' }
const td: React.CSSProperties = { padding: '6px 8px', color: '#c0c0c0' }
