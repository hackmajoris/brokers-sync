import { useEffect, useRef, useState } from 'react'

interface Props {
  text: string
}

export function InfoTooltip({ text }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLSpanElement>(null)

  useEffect(() => {
    if (!open) return
    function onClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onClickOutside)
    return () => document.removeEventListener('mousedown', onClickOutside)
  }, [open])

  return (
    <span ref={ref} style={{ position: 'relative', display: 'inline-flex', textTransform: 'none' }}>
      <button
        onClick={e => {
          e.stopPropagation()
          setOpen(o => !o)
        }}
        style={{
          width: 13,
          height: 13,
          borderRadius: '50%',
          border: '1px solid #8b8fa3',
          background: 'transparent',
          color: '#777777',
          fontSize: 9,
          fontWeight: 700,
          lineHeight: '11px',
          padding: 0,
          cursor: 'pointer',
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
        aria-label="What is this?"
      >
        i
      </button>
      {open && (
        <div
          onClick={e => e.stopPropagation()}
          style={{
            position: 'absolute',
            top: '150%',
            right: 0,
            width: 240,
            background: '#161616',
            border: '1px solid #2a2a2a',
            borderRadius: 8,
            padding: '10px 12px',
            fontSize: 11,
            fontWeight: 400,
            letterSpacing: 'normal',
            color: '#c0c0c0',
            lineHeight: 1.5,
            whiteSpace: 'normal',
            zIndex: 20,
            boxShadow: '0 8px 24px rgba(0,0,0,0.5)',
          }}
        >
          {text}
        </div>
      )}
    </span>
  )
}
