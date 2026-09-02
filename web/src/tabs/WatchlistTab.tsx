import { useState, useEffect, useRef } from 'react'
import { useIsMobile } from '../lib/useIsMobile'
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
import { fmtCurrency, fmtPct, fmtKMBT, clr } from '../utils/format'
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

type Column = { key: SortKey; label: string; align?: 'left' | 'right' }

// The table always carries every column. A phone gets the stacked list instead,
// and reaches this table through the "All columns" toggle when it wants to pan.
const COLUMNS: Column[] = [
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
  // Opens on the day's movers, biggest gain first. null means no column sort is
  // active, which is the only state where pinned items float to the top: once a
  // column is sorted the sort owns the whole list, pinned or not, otherwise
  // sorting by P/E would only ever reorder the pinned block and look broken.
  const [sortKey, setSortKey] = useState<SortKey | null>('today')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [compare, setCompare] = useState(false)
  // Symbols selected for comparison. Deliberately in memory only — this is a
  // scratch selection, unlike item.pinned which is stored per portfolio.
  const [selected, setSelected] = useState<string[]>([])
  // In compare mode the unselected rows are hidden, so picking needs its own
  // state — otherwise there would be nothing left to click once the first
  // symbol is selected.
  const [picking, setPicking] = useState(true)
  const searchTimer = useRef<number | undefined>(undefined)
  const mobile = useIsMobile()
  // On a phone the stacked list is the default view. "All columns" swaps in the
  // full table, which the user then pans sideways.
  const [allColumns, setAllColumns] = useState(false)
  const [searchFocused, setSearchFocused] = useState(false)
  // Symbol whose long-press action sheet is open, if any.
  const [sheetFor, setSheetFor] = useState<string | null>(null)
  // Compare and the column-mode toggle live behind an overflow menu: both are
  // occasional, and a permanent button row costs a whole strip of table height.
  const [menuOpen, setMenuOpen] = useState(false)
  // Phone-only quick filter. Kept out of the desktop table, which filters by
  // sorting instead.
  const [mobileFilter, setMobileFilter] = useState<MobileFilter>('All')
  const stacked = mobile && !allColumns

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

  // The stacked list has no header row to click, so its sort control sets the
  // key directly and an empty pick releases the sort back to pinned-first.
  function pickSort(key: SortKey | null) {
    setSortKey(key)
    if (key) setSortDir(key === 'symbol' || key === 'note' ? 'asc' : 'desc')
  }

  // Flipping while no column is picked would have nothing to reverse, so it
  // adopts the symbol order first. The button never reads as dead.
  function flipSort() {
    if (sortKey == null) {
      setSortKey('symbol')
      setSortDir('desc')
      return
    }
    setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
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
            style={{ ...inputStyle, flex: 1, fontSize: mobile ? 16 : 12 }}
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

  // The chips narrow the phone list only, and read against whichever column is
  // sorted so "Gainers" means gainers over the timeframe on screen.
  const tf: SortKey = sortKey ?? 'today'
  const matrixRows = stacked ? visibleRows.filter(i => matchesFilter(i, mobileFilter, tf)) : visibleRows

  // Top three movers each way, off the whole list rather than the filtered view:
  // the rails are an overview, so a chip narrowing the table below should not
  // quietly redefine what "today's biggest gainer" means.
  const withDayMove = items.filter(i => i.indicators?.todayReturn != null)
  const byDayMove = withDayMove.slice().sort((a, b) => b.indicators!.todayReturn! - a.indicators!.todayReturn!)
  const gainers = byDayMove.filter(i => i.indicators!.todayReturn! > 0).slice(0, 3)
  const losers = byDayMove.filter(i => i.indicators!.todayReturn! < 0).slice(-3).reverse()

  return (
    <div style={{ maxWidth: items.length > 0 ? 1300 : 820, margin: '0 auto', width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: mobile ? 12 : 8 }}>
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

      <div style={{ width: '100%', maxWidth: 520, display: 'flex', alignItems: 'center', gap: 8 }}>
      <div style={{ position: 'relative', flex: 1 }}>
        <span
          aria-hidden
          style={{ position: 'absolute', left: 14, top: '50%', transform: 'translateY(-50%)', color: searchFocused ? accent : '#8b8fa3', fontSize: 15, pointerEvents: 'none', transition: 'color .15s' }}
        >
          ⌕
        </span>
        <input
          value={query}
          onChange={e => setQuery(e.target.value)}
          onFocus={() => setSearchFocused(true)}
          onBlur={() => setSearchFocused(false)}
          placeholder="Search a symbol to add…"
          aria-label="Search a symbol to add"
          style={{
            ...inputStyle,
            width: '100%',
            // 16px on touch keeps iOS from zooming the page on focus.
            fontSize: mobile ? 16 : 14,
            padding: mobile ? '10px 36px' : '9px 34px',
            borderRadius: 10,
            borderColor: searchFocused ? accent + '66' : '#1f1f1f',
            background: searchFocused ? '#0d0d0d' : '#0a0a0a',
            boxShadow: searchFocused ? `0 0 0 3px ${accent}1a` : 'none',
            transition: 'border-color .15s, box-shadow .15s, background .15s',
          }}
        />
        {query !== '' && (
          <button
            onMouseDown={e => e.preventDefault()}
            onClick={() => {
              setQuery('')
              setResults([])
            }}
            title="Clear search"
            aria-label="Clear search"
            style={{
              position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)',
              width: mobile ? 34 : 26, height: mobile ? 34 : 26, borderRadius: 6,
              background: 'transparent', border: 'none', color: '#666666',
              fontSize: 14, lineHeight: 1, cursor: 'pointer',
            }}
          >
            ×
          </button>
        )}
        {results.length > 0 && (
          <div style={{ position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 20, background: '#0d0d0d', border: '1px solid #232323', borderRadius: 10, marginTop: 6, maxHeight: 300, overflowY: 'auto', boxShadow: '0 12px 28px rgba(0,0,0,.55)', padding: 4 }}>
            {results.map(r => (
              <button
                key={r.symbol}
                onClick={() => handleAdd(r.symbol)}
                style={{
                  display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12,
                  width: '100%', padding: mobile ? '12px 10px' : '9px 10px', borderRadius: 7,
                  background: 'transparent', border: 'none', color: '#d0d0d0',
                  fontSize: 13, cursor: 'pointer', textAlign: 'left',
                }}
                onMouseEnter={e => (e.currentTarget.style.background = '#181818')}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
              >
                <span style={{ fontWeight: 600, color: accent, whiteSpace: 'nowrap' }}>{r.symbol}</span>
                <span style={{ color: '#777777', fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.name}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {items.length > 0 && (
        <div style={{ position: 'relative', flexShrink: 0 }}>
          <button
            onClick={() => setMenuOpen(v => !v)}
            title="View options"
            aria-label="View options"
            aria-expanded={menuOpen}
            style={{
              ...secondaryBtn,
              width: mobile ? 38 : 34,
              height: mobile ? 38 : 34,
              padding: 0,
              fontSize: 15,
              lineHeight: 1,
              borderColor: menuOpen ? accent + '66' : '#262626',
              color: menuOpen ? accent : '#8b8fa3',
            }}
          >
            ⋯
          </button>
          {menuOpen && (
            <>
              <div onClick={() => setMenuOpen(false)} style={{ position: 'fixed', inset: 0, zIndex: 19 }} />
              <div
                style={{
                  position: 'absolute',
                  top: '100%',
                  right: 0,
                  marginTop: 6,
                  zIndex: 20,
                  minWidth: 170,
                  background: '#0d0d0d',
                  border: '1px solid #232323',
                  borderRadius: 10,
                  boxShadow: '0 12px 28px rgba(0,0,0,.55)',
                  padding: 4,
                  display: 'flex',
                  flexDirection: 'column',
                }}
              >
                <MenuItem
                  label="Compare"
                  active={compare}
                  accent={accent}
                  mobile={mobile}
                  onClick={() => {
                    toggleCompare()
                    setMenuOpen(false)
                  }}
                />
                {mobile && (
                  <MenuItem
                    label="All columns"
                    active={allColumns}
                    accent={accent}
                    mobile={mobile}
                    onClick={() => {
                      setAllColumns(v => !v)
                      setMenuOpen(false)
                    }}
                  />
                )}
              </div>
            </>
          )}
        </div>
      )}
      </div>

      {error && <ErrorLine text={error} />}

      {compare && items.length > 0 && (
        <div style={{ width: '100%', display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
          <button onClick={toggleCompare} style={primaryBtn(accent)}>
            {`Comparing ${selected.length}`}
          </button>
          {compare && (
            <>
              <button onClick={() => setPicking(v => !v)} style={secondaryBtn}>
                {picking ? 'Done picking' : 'Add more'}
              </button>
              <button onClick={exitCompare} style={secondaryBtn}>
                Clear
              </button>
              <span style={{ fontSize: 11, color: '#8b8fa3' }}>
                {stacked ? 'Tap a row to add or drop it.' : 'Click a row to add or drop it from the comparison.'}
              </span>
            </>
          )}
        </div>
      )}

      {loading && items.length === 0 ? (
        <span style={{ fontSize: 12, color: '#666666' }}>Loading…</span>
      ) : items.length === 0 ? (
        <span style={{ fontSize: 12, color: '#666666' }}>Nothing tracked yet.</span>
      ) : stacked ? (
        <>
          <StatTiles total={items.length} rows={matrixRows} tf={tf} />
          <Rail title="Today's Gainers" accentBar="#34d399" items={gainers} onOpen={openStockLookup} />
          <Rail title="Today's Losers" accentBar="#f87171" items={losers} onOpen={openStockLookup} />
          <FilterChips value={mobileFilter} onPick={setMobileFilter} accent={accent} />
          <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
            <SectionLabel>All Stats</SectionLabel>
            <span style={{ fontSize: 9.5, color: '#475569' }}>{matrixRows.length} rows · swipe table →</span>
          </div>
          <div
            className="rail"
            style={{
              width: '100%',
              // Sideways only. The rows run as long as the list does and the
              // page carries the vertical scroll, so the matrix never becomes a
              // second scroll region competing with the one under the thumb.
              overflowX: 'auto',
              overflowY: 'hidden',
              background: '#090f1c',
              border: '1px solid #161f31',
              borderRadius: 12,
              WebkitOverflowScrolling: 'touch',
            }}
          >
            <div style={{ minWidth: PIN_W + MOBILE_COLS.length * COL_W }}>
              <MatrixHead sortKey={sortKey} sortDir={sortDir} onPick={pickSort} onFlip={flipSort} />
              {matrixRows.map((item, i) => (
                <MatrixRow
                  key={item.symbol}
                  item={item}
                  accent={accent}
                  sortKey={sortKey}
                  selected={compare && selected.includes(item.symbol)}
                  onSelect={compare ? handleSelect : undefined}
                  onLongPress={() => setSheetFor(item.symbol)}
                  last={i === matrixRows.length - 1}
                />
              ))}
              {matrixRows.length === 0 && (
                <div style={{ padding: '28px 14px', textAlign: 'center', color: '#475569', fontSize: 12 }}>No symbols match.</div>
              )}
            </div>
          </div>
          {sheetFor && (
            <ActionSheet
              item={items.find(i => i.symbol === sheetFor)!}
              accent={accent}
              onPin={item => {
                handleSave(item, { pinned: !item.pinned })
                setSheetFor(null)
              }}
              onRemove={item => {
                void handleRemove(item.symbol)
                setSheetFor(null)
              }}
              onClose={() => setSheetFor(null)}
            />
          )}
        </>
      ) : (
        <div style={{ width: '100%', overflow: 'auto', maxHeight: `calc(100dvh - ${mobile ? 300 : compare ? 218 : 178}px)`, minHeight: 240, background: '#090f1c', border: '1px solid #161f31', borderRadius: 12 }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12, textAlign: 'left' }}>
          <thead>
            <tr style={{ color: '#666666', textAlign: 'left', background: '#090f1c' }}>
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
                    // Sticky lives on the cells, not the row: with
                    // border-collapse a sticky <tr> is ignored. The symbol
                    // header is sticky on both axes and has to outrank both the
                    // other headers and the sticky body cells.
                    position: 'sticky',
                    top: 0,
                    zIndex: 3,
                    background: '#090f1c',
                    ...(col.key === 'symbol' ? { ...stickyCol('#090f1c', 4), top: 0 } : {}),
                  }}
                >
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, flexDirection: col.align === 'left' ? 'row' : 'row-reverse' }}>
                    {col.label}
                    <span style={{ fontSize: 8, opacity: sortKey === col.key ? 1 : 0.25 }}>{sortKey === col.key && sortDir === 'asc' ? '▲' : '▼'}</span>
                    {INDICATOR_INFO[col.key] && <InfoTooltip text={INDICATOR_INFO[col.key]!} />}
                  </span>
                </th>
              ))}
              <th style={{ ...th, position: 'sticky', top: 0, zIndex: 3, background: '#090f1c' }} />
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
                touch={mobile}
              />
            ))}
          </tbody>
        </table>
        </div>
      )}

      <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, borderTop: '1px solid #161f31', paddingTop: 12 }}>
        <span style={{ fontSize: 11, color: '#8b8fa3' }}>
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
  touch,
}: {
  item: WatchlistItem
  accent: string
  onSave: (item: WatchlistItem, patch: Partial<WatchlistItem>) => void
  onRemove: (symbol: string) => void
  selected: boolean
  onSelect?: (symbol: string) => void
  touch: boolean
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
  const stickyBg = selected ? '#16203a' : item.pinned ? PIN_STICKY : '#090f1c'
  // The edge is an inset shadow rather than a border: border-collapse hands
  // collapsed borders to the table to paint, so a border here scrolls out of
  // view with the table instead of staying with the sticky cell.
  const edge = item.pinned ? PIN_EDGE : selected ? accent : null
  const tap = touch ? 38 : 24

  return (
    // Clicking the row opens the same lookup modal the Positions tab uses.
    // The inputs and the remove button stop propagation so editing a note does
    // not also open the popup.
    <tr
      onClick={() => (onSelect ? onSelect(item.symbol) : openStockLookup(item.symbol))}
      style={{ borderTop: '1px solid #161f31', cursor: 'pointer', background: restBg }}
      onMouseEnter={e => {
        e.currentTarget.style.background = accent + '11'
        ;(e.currentTarget.firstElementChild as HTMLElement).style.background = '#141c2c'
      }}
      onMouseLeave={e => {
        e.currentTarget.style.background = restBg
        ;(e.currentTarget.firstElementChild as HTMLElement).style.background = stickyBg
      }}
    >
      <td style={{ ...td, fontWeight: 600, color: accent, whiteSpace: 'nowrap', ...stickyCol(stickyBg, 1), boxShadow: edge ? `inset 2px 0 0 0 ${edge}` : undefined }}>
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
              fontSize: touch ? 16 : 12,
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
          color: gap == null ? '#8b8fa3' : clr(gap),
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
          style={{ ...inputStyle, width: '100%', minWidth: 140, fontSize: touch ? 16 : 12 }}
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
            width: tap,
            height: tap,
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
            width: tap,
            height: tap,
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
        <td key={key} style={{ ...td, textAlign: 'right', color: '#8b8fa3' }}>
          —
        </td>
      ))}
    </>
  )
}

// ── Stacked mobile list ──────────────────────────────────────────────────────

// The phone view is a stats matrix rather than a card list: the pinned symbol
// column stays put while every timeframe and ratio scrolls past it sideways, so
// a row can be read across without collapsing the data down to three lines.
const MC = {
  surf: '#111827',
  line: '#161f31',
  mut: '#64748b',
  dim: '#475569',
  txt: '#e2e8f0',
}

const PIN_W = 104
const COL_W = 58

type MobileGroup = 'perf' | 'val'
type MobileKind = 'pct' | 'num' | 'cap'

// marketCap has no IndicatorKey behind it, so Cap displays but never sorts.
type MobileColKey = SortKey | 'marketCap'

const MOBILE_COLS: { key: MobileColKey; label: string; kind: MobileKind; group: MobileGroup }[] = [
  { key: 'today', label: '1D', kind: 'pct', group: 'perf' },
  { key: 'oneWeek', label: '1W', kind: 'pct', group: 'perf' },
  { key: 'oneMonth', label: '1M', kind: 'pct', group: 'perf' },
  { key: 'ytd', label: 'YTD', kind: 'pct', group: 'perf' },
  { key: 'fiveYr', label: '5Y', kind: 'pct', group: 'perf' },
  { key: 'tenYr', label: '10Y', kind: 'pct', group: 'perf' },
  { key: 'pe', label: 'P/E', kind: 'num', group: 'val' },
  { key: 'forwardPe', label: 'Fwd', kind: 'num', group: 'val' },
  { key: 'targetGap', label: 'Tgt', kind: 'pct', group: 'val' },
  { key: 'marketCap', label: 'Cap', kind: 'cap', group: 'val' },
]

const MOBILE_FILTERS = ['All', '★ Pinned', 'Gainers', 'Losers', 'Undervalued'] as const
type MobileFilter = (typeof MOBILE_FILTERS)[number]

// "Undervalued" is the target reading, not a screen: the price still has room
// to rise to the target the user set.
function matchesFilter(item: WatchlistItem, filter: MobileFilter, tf: SortKey): boolean {
  if (filter === 'All') return true
  if (filter === '★ Pinned') return item.pinned
  if (filter === 'Undervalued') return (targetGap(item) ?? 0) > 0
  const v = cellValue(item, tf)
  if (v == null) return false
  return filter === 'Gainers' ? v > 0 : v < 0
}

function cellValue(item: WatchlistItem, key: MobileColKey): number | undefined {
  const p = item.indicators
  switch (key) {
    case 'today':
      return p?.todayReturn
    case 'oneWeek':
      return p?.oneWeekReturn
    case 'oneMonth':
      return p?.oneMonthReturn
    case 'ytd':
      return p?.ytdReturn
    case 'fiveYr':
      return p?.fiveYrReturn
    case 'tenYr':
      return p?.tenYrReturn
    case 'pe':
      return p?.pe != null && p.pe > 0 ? p.pe : undefined
    case 'forwardPe':
      return p?.forwardPE != null && p.forwardPE > 0 ? p.forwardPE : undefined
    case 'targetGap':
      return targetGap(item) ?? undefined
    case 'marketCap':
      return p?.marketCap
    default:
      return undefined
  }
}

// A four-digit percentage eats two columns, so anything past 1000% collapses to
// a multiple: +1,284% reads as 13x in the same width.
function compactPct(v: number): string {
  if (Math.abs(v) >= 1000) return `${v < 0 ? '-' : ''}${(Math.abs(v) / 100).toFixed(0)}x`
  return `${v >= 0 ? '+' : ''}${v.toFixed(1)}%`
}

function MatrixCell({ item, kind, colKey }: { item: WatchlistItem; kind: MobileKind; colKey: MobileColKey }) {
  const v = cellValue(item, colKey)
  if (v == null) return <span style={{ fontSize: 11, color: MC.dim, fontFamily: "'DM Mono', monospace" }}>—</span>
  if (kind === 'pct')
    return (
      <span style={{ fontSize: 11, fontWeight: 600, fontFamily: "'DM Mono', monospace", color: clr(v), letterSpacing: '-0.04em' }}>
        {compactPct(v)}
      </span>
    )
  if (kind === 'cap')
    return <span style={{ fontSize: 10.5, fontFamily: "'DM Mono', monospace", color: '#94a3b8' }}>{fmtKMBT(v)}</span>
  return <span style={{ fontSize: 11, fontFamily: "'DM Mono', monospace", color: '#cbd5e1', letterSpacing: '-0.03em' }}>{v.toFixed(1)}</span>
}

// Sorting lives entirely on the headers: tap a column to sort by it, tap the
// active one again to reverse. Same contract as the desktop table.
function MatrixHead({
  sortKey,
  sortDir,
  onPick,
  onFlip,
}: {
  sortKey: SortKey | null
  sortDir: SortDir
  onPick: (key: SortKey) => void
  onFlip: () => void
}) {
  return (
    <div style={{ display: 'flex', alignItems: 'stretch', height: 28, background: '#0f172a', borderBottom: `1px solid ${MC.line}`, position: 'sticky', top: 0, zIndex: 3 }}>
      <div style={{ position: 'sticky', left: 0, zIndex: 2, width: PIN_W, flexShrink: 0, background: '#0f172a', borderRight: `1px solid ${MC.line}`, display: 'flex', alignItems: 'center', padding: '0 8px 0 10px' }}>
        <span style={{ fontSize: 9, fontWeight: 700, letterSpacing: '0.08em', textTransform: 'uppercase', color: MC.dim }}>Symbol</span>
      </div>
      {MOBILE_COLS.map((c, i) => (
        <div
          key={c.key}
          onClick={() => c.key !== 'marketCap' && (c.key === sortKey ? onFlip() : onPick(c.key))}
          style={{
            width: COL_W,
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            padding: '0 8px',
            cursor: c.key === 'marketCap' ? 'default' : 'pointer',
            background: c.key === sortKey ? ACCENT_TINT_HEAD : 'transparent',
            borderLeft: MOBILE_COLS[i - 1] && MOBILE_COLS[i - 1].group !== c.group ? `1px solid ${MC.line}` : 'none',
          }}
        >
          <span style={{ fontSize: 9.5, fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase', color: c.key === sortKey ? '#818cf8' : MC.dim }}>
            {c.label}
            {c.key === sortKey && <span style={{ fontSize: 7, marginLeft: 2 }}>{sortDir === 'asc' ? '▲' : '▼'}</span>}
          </span>
        </div>
      ))}
    </div>
  )
}

const ACCENT_TINT_HEAD = '#818cf81c'
const ACCENT_TINT_CELL = '#818cf80f'

// One matrix row. The pinned cell has to stay opaque for the same reason the
// desktop table's does: the scrolled columns pass underneath it.
function MatrixRow({
  item,
  accent,
  sortKey,
  selected,
  onSelect,
  onLongPress,
  last,
}: {
  item: WatchlistItem
  accent: string
  sortKey: SortKey | null
  selected: boolean
  onSelect?: (symbol: string) => void
  onLongPress: () => void
  last: boolean
}) {
  const p = item.indicators
  const edge = item.pinned ? PIN_EDGE : selected ? accent : null
  const rowBg = selected ? accent + '0d' : item.pinned ? PIN_TINT : 'transparent'
  const pinBg = selected ? '#16203a' : item.pinned ? PIN_STICKY : '#090f1c'

  // A long press opens the action sheet. The press also fires a click when the
  // finger lifts, which would open the lookup modal on top of the sheet, so the
  // timer sets a flag the click handler checks and clears.
  const timer = useRef<number | undefined>(undefined)
  const fired = useRef(false)

  function start() {
    fired.current = false
    timer.current = window.setTimeout(() => {
      fired.current = true
      onLongPress()
    }, 500)
  }

  function cancel() {
    window.clearTimeout(timer.current)
  }

  useEffect(() => () => window.clearTimeout(timer.current), [])

  return (
    <div
      onPointerDown={start}
      onPointerUp={cancel}
      onPointerLeave={cancel}
      onPointerCancel={cancel}
      onContextMenu={e => e.preventDefault()}
      onClick={() => {
        if (fired.current) {
          fired.current = false
          return
        }
        if (onSelect) onSelect(item.symbol)
        else openStockLookup(item.symbol)
      }}
      style={{
        display: 'flex',
        alignItems: 'stretch',
        height: 44,
        background: rowBg,
        borderBottom: last ? 'none' : `1px solid ${MC.line}`,
        cursor: 'pointer',
        userSelect: 'none',
        WebkitUserSelect: 'none',
        WebkitTouchCallout: 'none',
      }}
    >
      <div
        style={{
          position: 'sticky',
          left: 0,
          zIndex: 2,
          width: PIN_W,
          flexShrink: 0,
          background: pinBg,
          borderRight: `1px solid ${MC.line}`,
          borderLeft: edge ? `2px solid ${edge}` : '2px solid transparent',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          gap: 1,
          padding: '0 8px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 4, minWidth: 0 }}>
          <span style={{ fontSize: 12.5, fontWeight: 700, letterSpacing: '-0.02em', color: accent, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {item.symbol}
          </span>
          {item.pinned && <span style={{ color: PIN_EDGE, fontSize: 8, flexShrink: 0 }}>★</span>}
        </div>
        <span style={{ fontSize: 11, fontFamily: "'DM Mono', monospace", color: '#94a3b8', letterSpacing: '-0.03em' }}>
          {p?.currentPrice != null ? fmtCurrency(p.currentPrice) : '—'}
        </span>
      </div>
      {MOBILE_COLS.map((c, i) => (
        <div
          key={c.key}
          style={{
            width: COL_W,
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            padding: '0 8px',
            background: c.key === sortKey ? ACCENT_TINT_CELL : 'transparent',
            borderLeft: MOBILE_COLS[i - 1] && MOBILE_COLS[i - 1].group !== c.group ? `1px solid ${MC.line}` : 'none',
          }}
        >
          <MatrixCell item={item} kind={c.kind} colKey={c.key} />
        </div>
      ))}
    </div>
  )
}

// Tracked / up / down / average across whatever the active filter left visible,
// read off the column the list is currently sorted by.
// Horizontal rail of cards. The design carries a sparkline on each one; there
// is no price series behind a watchlist item, so the card shows the numbers it
// can actually stand behind instead of a drawn-from-noise trend.
function Rail({
  title,
  accentBar,
  items,
  onOpen,
}: {
  title: string
  accentBar: string
  items: WatchlistItem[]
  onOpen: (symbol: string) => void
}) {
  if (items.length === 0) return null
  return (
    <div style={{ width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
        <span style={{ width: 3, height: 11, borderRadius: 2, background: accentBar }} />
        <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: '0.09em', textTransform: 'uppercase', color: '#94a3b8' }}>{title}</span>
      </div>
      <div className="rail" style={{ display: 'flex', gap: 8, overflowX: 'auto', scrollSnapType: 'x mandatory' }}>
        {items.map(item => (
          <div key={item.symbol} style={{ scrollSnapAlign: 'start' }}>
            <MoverCard item={item} tone={accentBar} onOpen={onOpen} />
          </div>
        ))}
      </div>
    </div>
  )
}

function MoverCard({ item, tone, onOpen }: { item: WatchlistItem; tone: string; onOpen: (symbol: string) => void }) {
  const p = item.indicators
  return (
    <button
      onClick={() => onOpen(item.symbol)}
      style={{
        width: 132,
        background: MC.surf,
        border: `1px solid ${tone}2e`,
        borderRadius: 11,
        padding: '9px 10px',
        display: 'flex',
        flexDirection: 'column',
        gap: 6,
        cursor: 'pointer',
        textAlign: 'left',
        fontFamily: "'DM Sans', sans-serif",
        flexShrink: 0,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 4, width: '100%' }}>
        <span style={{ fontSize: 12.5, fontWeight: 700, color: MC.txt, letterSpacing: '-0.01em', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {item.symbol}
        </span>
        {item.pinned && <span style={{ color: PIN_EDGE, fontSize: 9, flexShrink: 0 }}>★</span>}
      </div>
      <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', width: '100%' }}>
        <span style={{ fontSize: 12, fontFamily: "'DM Mono', monospace", color: '#cbd5e1' }}>
          {p?.currentPrice != null ? fmtCurrency(p.currentPrice) : '—'}
        </span>
        <span style={{ fontSize: 11.5, fontWeight: 700, fontFamily: "'DM Mono', monospace", color: tone }}>
          {p?.todayReturn != null ? fmtPct(p.todayReturn) : '—'}
        </span>
      </div>
    </button>
  )
}

function StatTiles({ total, rows, tf }: { total: number; rows: WatchlistItem[]; tf: SortKey }) {
  const vals = rows.map(r => cellValue(r, tf)).filter((v): v is number => v != null)
  const up = vals.filter(v => v > 0).length
  const down = vals.filter(v => v < 0).length
  const avg = vals.length ? vals.reduce((s, v) => s + v, 0) / vals.length : null
  const tiles: [string, string, string][] = [
    ['Tracked', String(total), MC.txt],
    ['Up', String(up), '#34d399'],
    ['Down', String(down), '#f87171'],
    ['Avg', avg == null ? '—' : compactPct(avg), avg == null ? MC.mut : clr(avg)],
  ]
  return (
    <div style={{ display: 'flex', gap: 6, width: '100%' }}>
      {tiles.map(([k, v, c]) => (
        <div key={k} style={{ flex: 1, background: MC.surf, border: `1px solid ${MC.line}`, borderRadius: 9, padding: '6px 8px' }}>
          <div style={{ fontSize: 8.5, color: MC.dim, fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase' }}>{k}</div>
          <div style={{ fontSize: 14, fontWeight: 700, color: c, fontFamily: "'DM Mono', monospace", letterSpacing: '-0.03em' }}>{v}</div>
        </div>
      ))}
    </div>
  )
}

function FilterChips({ value, onPick, accent }: { value: MobileFilter; onPick: (f: MobileFilter) => void; accent: string }) {
  return (
    <div className="rail" style={{ display: 'flex', gap: 6, overflowX: 'auto', width: '100%' }}>
      {MOBILE_FILTERS.map(c => (
        <button
          key={c}
          onClick={() => onPick(c)}
          style={{
            padding: '5px 11px',
            borderRadius: 999,
            fontSize: 11,
            fontWeight: 600,
            cursor: 'pointer',
            whiteSpace: 'nowrap',
            fontFamily: "'DM Sans', sans-serif",
            background: value === c ? accent + '26' : MC.surf,
            color: value === c ? accent : MC.mut,
            border: `1px solid ${value === c ? accent + '55' : MC.line}`,
          }}
        >
          {c}
        </button>
      ))}
    </div>
  )
}

function MenuItem({
  label,
  active,
  accent,
  mobile,
  onClick,
}: {
  label: string
  active: boolean
  accent: string
  mobile: boolean
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 12,
        width: '100%',
        padding: mobile ? '11px 10px' : '8px 10px',
        borderRadius: 7,
        border: 'none',
        background: 'transparent',
        color: active ? accent : '#d0d0d0',
        fontSize: 12,
        fontWeight: active ? 600 : 500,
        fontFamily: "'DM Sans', sans-serif",
        cursor: 'pointer',
        textAlign: 'left',
      }}
      onMouseEnter={e => (e.currentTarget.style.background = '#181818')}
      onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
    >
      {label}
      {active && <span style={{ fontSize: 10 }}>✓</span>}
    </button>
  )
}

// Long-press sheet. Pin and remove live here rather than on the row: both are
// state-changing, and a 38px control beside a tap-to-open row is a mis-tap
// waiting to happen.
function ActionSheet({
  item,
  accent,
  onPin,
  onRemove,
  onClose,
}: {
  item: WatchlistItem
  accent: string
  onPin: (item: WatchlistItem) => void
  onRemove: (item: WatchlistItem) => void
  onClose: () => void
}) {
  const row: React.CSSProperties = {
    width: '100%',
    padding: '15px 16px',
    background: 'transparent',
    border: 'none',
    borderTop: '1px solid #161f31',
    color: '#d0d0d0',
    fontSize: 15,
    textAlign: 'left',
    cursor: 'pointer',
  }

  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', zIndex: 100 }} />
      <div
        style={{
          position: 'fixed',
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 101,
          background: '#0d0d0d',
          borderTop: '1px solid #232323',
          borderRadius: '14px 14px 0 0',
          paddingBottom: 'env(safe-area-inset-bottom)',
        }}
      >
        <div style={{ padding: '14px 16px 10px', fontSize: 13, fontWeight: 600, color: accent }}>{item.symbol}</div>
        <button style={row} onClick={() => onPin(item)}>
          <span style={{ color: PIN_EDGE, marginRight: 8 }}>{item.pinned ? '★' : '☆'}</span>
          {item.pinned ? 'Unpin' : 'Pin to top'}
        </button>
        <button style={{ ...row, color: '#e06c6c' }} onClick={() => onRemove(item)}>
          Remove {item.symbol}
        </button>
        <button style={{ ...row, color: '#777777' }} onClick={onClose}>
          Cancel
        </button>
      </div>
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
  return { position: 'sticky', left: 0, background, zIndex, borderRight: '1px solid #161f31' }
}

const th: React.CSSProperties = { padding: '6px 8px', fontWeight: 600, fontSize: 10, textTransform: 'uppercase', letterSpacing: '0.06em' }
const td: React.CSSProperties = { padding: '6px 8px', color: '#c0c0c0' }
