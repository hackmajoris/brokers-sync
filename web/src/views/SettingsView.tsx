import { useRef, useState, useEffect } from 'react'
import { ACCENT_DEFAULT } from '../constants'
import { mapRawPortfolio, cacheRawPortfolio, uploadZip } from '../services/portfolioService'
import { cacheZip, loadCachedZip, clearCachedZip } from '../services/zipCache'
import type { PortfolioData } from '../types/portfolio'

type Status = 'idle' | 'uploading' | 'done'

const accent = ACCENT_DEFAULT

interface Props {
  noData?: boolean
  onImported?: (data: PortfolioData) => void
}

const BROKERS = [
  {
    id: 'ibkr',
    name: 'IBKR',
    full: 'Interactive Brokers',
    format: 'CSV',
    steps: [
      'Log in to Client Portal',
      'Go to Performance & Reports → Activity Statements',
      'Select Transaction History',
      'Set your start and end date range',
      'Click Download',
    ],
  },
  {
    id: 'revolut',
    name: 'Revolut',
    full: 'Revolut',
    format: 'CSV',
    steps: [
      'Open the Revolut app and tap Invest in the bottom menu',
      'Tap the ⋯ icon in the top right corner',
      'Select Documents → Stocks',
      'Choose Account statement or Profit & Loss statement',
      'Select your date range and export as CSV',
    ],
  },
  {
    id: 'trading212',
    name: 'Trading212',
    full: 'Trading212',
    format: 'CSV',
    steps: [
      'Open the app and go to Menu → History',
      'Tap the Export button',
      'Select your desired timeframe',
      'Choose which data to include (orders, dividends, transactions)',
      'Export as CSV and save the file',
    ],
  },
  {
    id: 'tradeville',
    name: 'Tradeville',
    full: 'Tradeville',
    format: 'XLSX / CSV',
    steps: [
      'Log in to your Tradeville account (web platform)',
      'Go to the Statements menu',
      'Open Available Reports → Account history',
      'Select your trading account',
      'Set a start date and end date, then click Request',
      'Download the generated report file',
    ],
  },
]

export function SettingsView({ noData, onImported }: Props) {
  const fileRef = useRef<HTMLInputElement>(null)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const [fileName, setFileName] = useState<string | null>(null)
  const [status, setStatus] = useState<Status>('idle')
  const [logs, setLogs] = useState<string[]>([])
  const [success, setSuccess] = useState<boolean | null>(null)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [expandedBroker, setExpandedBroker] = useState<string | null>(null)
  const [cachedZipName, setCachedZipName] = useState<string | null>(null)
  const [cachedZipDate, setCachedZipDate] = useState<number | null>(null)

  useEffect(() => {
    loadCachedZip().then(entry => {
      if (entry) {
        setCachedZipName(entry.name)
        setCachedZipDate(entry.savedAt)
      }
    })
  }, [])

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  function handleFileChange() {
    const file = fileRef.current?.files?.[0]
    setFileName(file ? file.name : null)
    setLogs([])
    setSuccess(null)
    setFetchError(null)
    setStatus('idle')
  }

  async function uploadBlob(blob: Blob, name: string) {
    setStatus('uploading')
    setLogs([])
    setSuccess(null)
    setFetchError(null)
    try {
      const raw = await uploadZip(blob, name, line => setLogs(prev => [...prev, line]))
      setSuccess(raw !== null)
      setStatus('done')
      if (raw) {
        cacheRawPortfolio(raw)
        onImported?.(mapRawPortfolio(raw))
      }
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : 'Upload failed')
      setStatus('done')
    }
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const file = fileRef.current?.files?.[0]
    if (!file) return
    await cacheZip(file)
    setCachedZipName(file.name)
    setCachedZipDate(Date.now())
    await uploadBlob(file, file.name)
    if (fileRef.current) fileRef.current.value = ''
    setFileName(null)
  }

  async function handleReprocess() {
    const entry = await loadCachedZip()
    if (!entry) return
    await uploadBlob(entry.blob, entry.name)
  }

  async function handleClearZip() {
    await clearCachedZip()
    setCachedZipName(null)
    setCachedZipDate(null)
  }

  const busy = status === 'uploading'

  return (
    <div style={{ maxWidth: 640, paddingTop: 8, margin: '0 auto' }}>
      {noData && (
        <div style={{
          background: accent + '11', border: `1px solid ${accent}33`,
          borderRadius: 10, padding: '14px 16px', marginBottom: 24,
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          <span style={{ fontSize: 18 }}>📂</span>
          <div>
            <div style={{ fontSize: 13, fontWeight: 600, color: accent, marginBottom: 2 }}>No data yet</div>
            <div style={{ fontSize: 12, color: '#888888' }}>Upload a ZIP with your broker CSV / XLSX files to get started.</div>
          </div>
        </div>
      )}

      <div style={{ marginBottom: 28 }}>
        <h2 style={{ fontSize: 16, fontWeight: 600, color: '#ffffff', marginBottom: 4 }}>How to download your reports</h2>
        <p style={{ fontSize: 13, color: '#888888', lineHeight: 1.5, marginBottom: 14 }}>
          Export your activity files from each broker you use, then ZIP them together and upload below.
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {BROKERS.map(b => {
            const open = expandedBroker === b.id
            return (
              <div
                key={b.id}
                style={{
                  background: '#0a0a0a',
                  border: `1px solid ${open ? accent + '44' : '#1a1a1a'}`,
                  borderRadius: 8,
                  overflow: 'hidden',
                  transition: 'border-color 0.15s',
                }}
              >
                <button
                  onClick={() => setExpandedBroker(open ? null : b.id)}
                  style={{
                    width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    padding: '10px 14px', background: 'none', border: 'none', cursor: 'pointer',
                    textAlign: 'left',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <span style={{ fontSize: 13, fontWeight: 600, color: '#e0e0e0' }}>{b.full}</span>
                    <span style={{
                      fontSize: 11, fontWeight: 500,
                      background: accent + '18', color: accent,
                      borderRadius: 4, padding: '1px 7px',
                    }}>{b.format}</span>
                  </div>
                  <span style={{ fontSize: 11, color: '#444444', userSelect: 'none' }}>{open ? '▲' : '▼'}</span>
                </button>

                {open && (
                  <div style={{ padding: '0 14px 12px 14px' }}>
                    <ol style={{ margin: 0, paddingLeft: 18, display: 'flex', flexDirection: 'column', gap: 5 }}>
                      {b.steps.map((step, i) => (
                        <li key={i} style={{ fontSize: 12, color: '#888888', lineHeight: 1.55 }}>{step}</li>
                      ))}
                    </ol>
                  </div>
                )}
              </div>
            )
          })}
        </div>

        <div style={{
          marginTop: 12,
          background: accent + '0d', border: `1px solid ${accent}2a`,
          borderRadius: 8, padding: '10px 14px',
          display: 'flex', alignItems: 'flex-start', gap: 10,
        }}>
          <span style={{ fontSize: 16, lineHeight: 1, flexShrink: 0, marginTop: 1 }}>📦</span>
          <p style={{ margin: 0, fontSize: 12, color: '#888888', lineHeight: 1.6 }}>
            Once you have all your files, <strong style={{ color: '#c0c0c0' }}>put them in a single ZIP archive</strong> — file names don't matter.
            Then upload it below. The server will auto-detect each broker's format.
          </p>
        </div>
      </div>

      <div style={{ marginBottom: 24 }}>
        <h2 style={{ fontSize: 16, fontWeight: 600, color: '#ffffff', marginBottom: 4 }}>Import data</h2>
        <p style={{ fontSize: 13, color: '#888888', lineHeight: 1.5 }}>
          Upload a ZIP file containing your broker CSV / XLSX export files. The server will
          parse them, regenerate <code style={{ color: '#c0c0c0', fontFamily: 'monospace' }}>result.json</code>,
          and refresh live prices.
        </p>
      </div>

      {cachedZipName && (
        <div style={{
          background: accent + '0d', border: `1px solid ${accent}2a`,
          borderRadius: 8, padding: '10px 14px', marginBottom: 12,
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          <span style={{ fontSize: 15, flexShrink: 0 }}>📦</span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 12, fontWeight: 500, color: '#c0c0c0', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {cachedZipName}
            </div>
            {cachedZipDate && (
              <div style={{ fontSize: 11, color: '#555555', marginTop: 1 }}>
                Cached {new Date(cachedZipDate).toLocaleString()}
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={handleReprocess}
            disabled={busy}
            style={{
              background: busy ? '#1a1a1a' : accent + '22',
              border: `1px solid ${busy ? '#252525' : accent + '55'}`,
              color: busy ? '#555555' : accent,
              borderRadius: 6, padding: '4px 12px', fontSize: 12, fontWeight: 500,
              cursor: busy ? 'not-allowed' : 'pointer', flexShrink: 0,
            }}
          >
            Re-process
          </button>
          <button
            type="button"
            onClick={handleClearZip}
            disabled={busy}
            style={{
              background: 'none', border: 'none', color: '#444444',
              fontSize: 16, cursor: busy ? 'not-allowed' : 'pointer',
              padding: '2px 4px', flexShrink: 0, lineHeight: 1,
            }}
            title="Remove cached ZIP"
          >
            ×
          </button>
        </div>
      )}

      <form onSubmit={handleSubmit} style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: 16 }}>
        <label style={{
          display: 'flex', alignItems: 'center', gap: 10, flex: 1,
          background: '#0a0a0a', border: '1px solid #1a1a1a', borderRadius: 8,
          padding: '8px 12px', cursor: 'pointer',
        }}>
          <input
            ref={fileRef}
            type="file"
            name="file"
            accept=".zip"
            required
            onChange={handleFileChange}
            style={{ display: 'none' }}
          />
          <span style={{
            background: accent + '22', border: `1px solid ${accent}44`,
            color: accent, borderRadius: 5, padding: '3px 10px',
            fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap', flexShrink: 0,
          }}>
            Choose ZIP…
          </span>
          <span style={{ fontSize: 13, color: fileName ? '#c0c0c0' : '#555555', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {fileName ?? 'No file chosen'}
          </span>
        </label>
        <button
          type="submit"
          disabled={busy || !fileName}
          style={{
            background: busy ? '#1a1a1a' : accent + '22',
            border: `1px solid ${busy ? '#252525' : accent + '55'}`,
            color: busy ? '#555555' : accent,
            borderRadius: 8, padding: '8px 16px', fontSize: 13, fontWeight: 500,
            cursor: busy || !fileName ? 'not-allowed' : 'pointer',
            whiteSpace: 'nowrap', flexShrink: 0,
          }}
        >
          {busy ? 'Processing…' : 'Upload & import'}
        </button>
      </form>

      {fetchError && !busy && (
        <div style={{
          background: '#f8717122', border: '1px solid #f8717144',
          borderRadius: 8, padding: '10px 14px', color: '#f87171', fontSize: 13, marginBottom: 12,
        }}>
          {fetchError}
        </div>
      )}

      {(logs.length > 0 || busy) && (
        <div style={{
          background: '#0a0d12', border: `1px solid ${
            status === 'done'
              ? success ? accent + '44' : '#f8717144'
              : '#1a1a1a'
          }`,
          borderRadius: 8, overflow: 'hidden',
        }}>
          {status === 'done' && (
            <div style={{
              padding: '8px 14px', borderBottom: '1px solid #1a1a1a',
              fontSize: 12, fontWeight: 500,
              color: success ? accent : '#f87171',
            }}>
              {success ? 'Import completed successfully.' : 'Import finished with errors.'}
            </div>
          )}
          {busy && (
            <div style={{
              padding: '8px 14px', borderBottom: '1px solid #1a1a1a',
              fontSize: 12, color: '#555555',
            }}>
              Processing…
            </div>
          )}
          <pre style={{
            padding: '12px 14px', fontSize: 12, color: '#888888',
            fontFamily: 'ui-monospace, monospace', overflowX: 'auto',
            maxHeight: 280, overflowY: 'auto', margin: 0, lineHeight: 1.6,
          }}>
            {logs.join('\n')}
            <div ref={logsEndRef} />
          </pre>
        </div>
      )}
    </div>
  )
}
