import { useMemo, useState } from 'react'
import type { PortfolioData } from '../types/portfolio'
import { fmt, fmtCurrency, clr } from '../utils/format'
import { BrokerPill } from '../components/ui/BrokerPill'
import { SectionLabel } from '../components/ui/SectionLabel'

interface Props {
  data: PortfolioData
}

const TYPE_COLORS: Record<string, string> = {
  BUY: '#34d399',
  SELL: '#f87171',
  DIVIDEND: '#60a5fa',
  TAX_WITHHOLDING: '#fbbf24',
  DEPOSIT: '#a78bfa',
  WITHDRAWAL: '#a78bfa',
  FEE: '#fbbf24',
  INTEREST: '#60a5fa',
  STOCK_SPLIT: '#c0c0c0',
  FOREX: '#c0c0c0',
  TRANSFER_IN: '#a78bfa',
  TRANSFER_OUT: '#a78bfa',
  UNKNOWN: '#555555',
}

function typeColor(t: string): string {
  return TYPE_COLORS[t] ?? '#c0c0c0'
}

const selectStyle: React.CSSProperties = {
  background: '#0f0f0f', border: '1px solid #1a1a1a', color: '#c0c0c0',
  borderRadius: 6, padding: '6px 10px', fontSize: 12, cursor: 'pointer',
}

export function TradesTab({ data }: Props) {
  const [broker, setBroker] = useState('')
  const [type, setType] = useState('')
  const [query, setQuery] = useState('')

  const brokers = useMemo(
    () => Array.from(new Set(data.transactions.map(t => t.broker))).sort(),
    [data.transactions],
  )
  const types = useMemo(
    () => Array.from(new Set(data.transactions.map(t => t.type))).sort(),
    [data.transactions],
  )

  const rows = useMemo(() => {
    const q = query.trim().toUpperCase()
    return data.transactions.filter(t =>
      (!broker || t.broker === broker) &&
      (!type || t.type === type) &&
      (!q || t.symbol.toUpperCase().includes(q) || t.name.toUpperCase().includes(q)),
    )
  }, [data.transactions, broker, type, query])

  const headers = ['Date', 'Broker', 'Type', 'Symbol', 'Qty', 'Price', 'Commission', 'Net']

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
        <input
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search symbol or name…"
          style={{ ...selectStyle, minWidth: 200, flex: '1 1 200px' }}
        />
        <select value={broker} onChange={e => setBroker(e.target.value)} style={selectStyle}>
          <option value="">All brokers</option>
          {brokers.map(b => <option key={b} value={b}>{b}</option>)}
        </select>
        <select value={type} onChange={e => setType(e.target.value)} style={selectStyle}>
          <option value="">All types</option>
          {types.map(t => <option key={t} value={t}>{t}</option>)}
        </select>
        <span style={{ fontSize: 11, color: '#555555', marginLeft: 'auto' }}>
          {rows.length} of {data.transactions.length}
        </span>
      </div>

      <div style={{ background: '#0f0f0f', borderRadius: 10, border: '1px solid #1a1a1a', overflow: 'hidden' }}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid #1a1a1a' }}>
          <SectionLabel>Ledger</SectionLabel>
        </div>
        <div style={{ overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 720 }}>
            <thead>
              <tr style={{ background: '#080808' }}>
                {headers.map(h => (
                  <th key={h} style={{ padding: '8px 14px', textAlign: h === 'Date' || h === 'Broker' || h === 'Type' || h === 'Symbol' ? 'left' : 'right', fontSize: 10, fontWeight: 600, color: '#555555', letterSpacing: '0.08em', textTransform: 'uppercase', whiteSpace: 'nowrap' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map(t => (
                <tr
                  key={t.id}
                  style={{ borderTop: '1px solid #1a1a1a' }}
                  onMouseEnter={e => (e.currentTarget.style.background = '#1a1a1a44')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <td style={{ padding: '10px 14px', color: '#888888', fontFamily: "'DM Mono', monospace", fontSize: 12, whiteSpace: 'nowrap' }}>{t.date.slice(0, 10)}</td>
                  <td style={{ padding: '10px 14px' }}><BrokerPill name={t.broker} /></td>
                  <td style={{ padding: '10px 14px', whiteSpace: 'nowrap' }}>
                    <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 10, fontWeight: 600, background: typeColor(t.type) + '22', color: typeColor(t.type) }}>{t.type}</span>
                  </td>
                  <td style={{ padding: '10px 14px', fontWeight: 600, fontSize: 12 }}>
                    {t.symbol || <span style={{ color: '#555555' }}>—</span>}
                  </td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 12, color: '#c0c0c0' }}>{t.quantity ? fmt(t.quantity, 4) : '—'}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 12, color: '#c0c0c0' }}>{t.price ? fmtCurrency(t.price, t.currency) : '—'}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 12, color: '#888888' }}>{t.commission ? fmtCurrency(t.commission, t.currency) : '—'}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, fontWeight: 600, color: clr(t.net) }}>{fmtCurrency(t.net, t.currency)}</td>
                </tr>
              ))}
              {rows.length === 0 && (
                <tr>
                  <td colSpan={headers.length} style={{ padding: '32px 14px', textAlign: 'center', color: '#555555', fontSize: 13 }}>
                    No transactions match the filters.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
