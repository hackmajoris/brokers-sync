import { useEffect, useState } from 'react'

// Keep in sync with the pre-paint script in watchlist.html, which applies the
// stored theme before React mounts so a light page never flashes dark.
const THEME_KEY = 'bs.theme'

export type Theme = 'light' | 'dark'

function loadTheme(): Theme {
  try {
    if (localStorage.getItem(THEME_KEY) === 'light') return 'light'
  } catch {
    // Storage blocked: fall through to the default.
  }
  return 'dark'
}

export function useTheme(): { theme: Theme; setTheme: (t: Theme) => void } {
  const [theme, setThemeState] = useState<Theme>(loadTheme)

  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  function setTheme(t: Theme) {
    try {
      localStorage.setItem(THEME_KEY, t)
    } catch {
      // Still switches for this visit.
    }
    setThemeState(t)
  }

  return { theme, setTheme }
}

// Appends a two-digit hex alpha. A hex colour keeps the plain suffix it always
// had; a var() cannot take one, so it goes through color-mix instead.
export function alpha(color: string, hex: string): string {
  if (color.startsWith('#')) return color + hex
  return `color-mix(in srgb, ${color} ${((parseInt(hex, 16) / 255) * 100).toFixed(1)}%, transparent)`
}

// clr() and the rating maps return fixed hex because the portfolio tabs append
// alpha suffixes to them and feed them to canvas charts. Components the
// watchlist shares pass those values through here to pick up the theme.
const THEMED: Record<string, string> = {
  '#34d399': 'var(--pos)',
  '#86efac': 'var(--pos-2)',
  '#f87171': 'var(--neg)',
  '#fbbf24': 'var(--amber)',
  '#fb923c': 'var(--orange)',
  '#c0c0c0': 'var(--text-2)',
  '#8b8fa3': 'var(--text-3)',
}

export function themed(color: string): string {
  return THEMED[color] ?? color
}
