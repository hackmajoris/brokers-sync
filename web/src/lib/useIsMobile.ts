import { useEffect, useState } from 'react'

// Matches the 600px breakpoint index.css already uses for the nav drawer, so
// the table and the chrome around it switch to their narrow layouts together.
const QUERY = '(max-width: 600px)'

export function useIsMobile(): boolean {
  const [mobile, setMobile] = useState(() => window.matchMedia(QUERY).matches)

  useEffect(() => {
    const mq = window.matchMedia(QUERY)
    const onChange = (e: MediaQueryListEvent) => setMobile(e.matches)
    mq.addEventListener('change', onChange)
    // A rotation between mount and this effect would otherwise be missed.
    setMobile(mq.matches)
    return () => mq.removeEventListener('change', onChange)
  }, [])

  return mobile
}
