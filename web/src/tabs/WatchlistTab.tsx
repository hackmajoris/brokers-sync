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

interface Props {
  accent: string
}

const MAX_NOTE = 500

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

  async function handleSave(item: WatchlistItem, patch: Partial<WatchlistItem>) {
    try {
      await upsertWatchlist({ ...item, ...patch })
      await refresh()
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
    <div style={{ maxWidth: 820, margin: '0 auto', width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
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
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12, textAlign: 'left' }}>
          <thead>
            <tr style={{ color: '#666666', textAlign: 'left' }}>
              <th style={th}>Symbol</th>
              <th style={th}>Note</th>
              <th style={{ ...th, textAlign: 'right' }}>Target</th>
              <th style={th} />
            </tr>
          </thead>
          <tbody>
            {items.map(item => (
              <Row key={item.symbol} item={item} accent={accent} onSave={handleSave} onRemove={handleRemove} />
            ))}
          </tbody>
        </table>
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

  return (
    <tr style={{ borderTop: '1px solid #161616' }}>
      <td style={{ ...td, fontWeight: 600, color: accent }}>{item.symbol}</td>
      <td style={td}>
        <input
          value={note}
          maxLength={MAX_NOTE}
          onChange={e => setNote(e.target.value)}
          onBlur={() => note !== (item.note ?? '') && onSave(item, { note })}
          placeholder="Add a note"
          style={{ ...inputStyle, width: '100%' }}
        />
      </td>
      <td style={{ ...td, textAlign: 'right' }}>
        <input
          value={target}
          inputMode="decimal"
          onChange={e => setTarget(e.target.value)}
          onBlur={() => {
            const parsed = target === '' ? 0 : Number(target)
            if (!Number.isNaN(parsed) && parsed !== item.targetPrice) onSave(item, { targetPrice: parsed })
          }}
          placeholder="—"
          style={{ ...inputStyle, width: 90, textAlign: 'right' }}
        />
      </td>
      <td style={{ ...td, textAlign: 'right' }}>
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

const th: React.CSSProperties = { padding: '6px 8px', fontWeight: 600, fontSize: 10, textTransform: 'uppercase', letterSpacing: '0.06em' }
const td: React.CSSProperties = { padding: '6px 8px', color: '#c0c0c0' }
