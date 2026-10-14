import type { ReactNode } from 'react'
import type { Theme } from '../../lib/theme'

const icon = (children: ReactNode) => (
  <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {children}
  </svg>
)

const OPTIONS: { value: Theme; label: string; icon: ReactNode }[] = [
  {
    value: 'light',
    label: 'Light',
    icon: icon(
      <>
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
      </>
    ),
  },
  { value: 'dark', label: 'Dark', icon: icon(<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />) },
]

export function ThemeToggle({ value, onChange, compact }: { value: Theme; onChange: (t: Theme) => void; compact: boolean }) {
  return (
    <div role="group" aria-label="Theme" style={{ display: 'flex', gap: 2, padding: 2, background: 'var(--line)', borderRadius: 8 }}>
      {OPTIONS.map(o => {
        const active = o.value === value
        return (
          <button
            key={o.value}
            onClick={() => onChange(o.value)}
            aria-pressed={active}
            aria-label={compact ? o.label : undefined}
            title={o.label}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 5,
              height: 28,
              padding: compact ? '0 8px' : '0 10px',
              border: 'none',
              borderRadius: 6,
              background: active ? 'var(--chip-active)' : 'transparent',
              color: active ? 'var(--text)' : 'var(--text-3)',
              boxShadow: active ? '0 1px 3px rgba(0,0,0,.12)' : 'none',
              fontSize: 12,
              fontWeight: 500,
              fontFamily: 'inherit',
              cursor: 'pointer',
            }}
          >
            {o.icon}
            {!compact && o.label}
          </button>
        )
      })}
    </div>
  )
}
