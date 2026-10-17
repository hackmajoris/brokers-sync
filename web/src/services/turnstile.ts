// Cloudflare Turnstile, the bot check in front of code creation. The script is
// loaded on first use only, so visitors who already have a code never fetch it.

interface TurnstileApi {
  render(el: HTMLElement, opts: Record<string, unknown>): string
  execute(widgetId: string): void
  remove(widgetId: string): void
}

declare global {
  interface Window {
    turnstile?: TurnstileApi
  }
}

const SCRIPT_URL = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"

// Cloudflare's documented always-pass test key, used in dev only. A production
// build without a real key fails at click time rather than shipping a check
// that can never be satisfied against the real secret.
const TEST_SITE_KEY = "1x00000000000000000000AA"
const siteKey: string | undefined =
  import.meta.env.VITE_TURNSTILE_SITE_KEY || (import.meta.env.DEV ? TEST_SITE_KEY : undefined)

let scriptLoad: Promise<TurnstileApi> | null = null

function loadScript(): Promise<TurnstileApi> {
  scriptLoad ??= new Promise((resolve, reject) => {
    const s = document.createElement("script")
    s.src = SCRIPT_URL
    s.async = true
    s.onload = () =>
      window.turnstile
        ? resolve(window.turnstile)
        : reject(new Error("Verification failed to load"))
    s.onerror = () => {
      scriptLoad = null
      reject(new Error("Verification failed to load. Check your connection and try again."))
    }
    document.head.appendChild(s)
  })
  return scriptLoad
}

// getTurnstileToken runs the check inside container and resolves with a
// single-use token. The widget stays hidden unless Cloudflare needs the user
// to interact, in which case it appears in container.
export async function getTurnstileToken(container: HTMLElement): Promise<string> {
  if (!siteKey) throw new Error("Verification is not configured.")
  const ts = await loadScript()
  let id = ""
  try {
    return await new Promise<string>((resolve, reject) => {
      id = ts.render(container, {
        sitekey: siteKey,
        appearance: "interaction-only",
        execution: "execute",
        callback: (token: string) => resolve(token),
        "error-callback": () =>
          reject(new Error("Couldn't verify you're human. Please try again.")),
        "expired-callback": () => reject(new Error("Verification expired. Please try again.")),
      })
      ts.execute(id)
    })
  } finally {
    // The token outlives the widget; removing it keeps repeated clicks from
    // stacking hidden widgets in the container.
    if (id) ts.remove(id)
  }
}
