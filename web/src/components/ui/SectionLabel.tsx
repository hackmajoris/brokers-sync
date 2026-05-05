import type { ReactNode } from 'react'

export function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <div
      style={{
        fontSize: 10,
        fontWeight: 600,
        letterSpacing: '0.12em',
        color: '#555555',
        textTransform: 'uppercase',
        marginBottom: 10,
      }}
    >
      {children}
    </div>
  )
}
