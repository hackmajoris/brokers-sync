const DB_NAME = 'brokers-sync'
const DB_VERSION = 1
const STORE = 'zip-cache'
const KEY = 'last-upload'

// Entries hold the zip's bytes, not the File object. A File is only a handle to
// a path on disk: store one and days later the user has moved, renamed or
// re-exported that file, so reading it yields nothing and the upload goes out
// with a multipart header and an empty body ("multipart: NextPart: EOF").
// Copying the bytes into IndexedDB makes the cache independent of the disk.
interface StoredZip {
  data: ArrayBuffer
  name: string
  savedAt: number
  type: string
}

// Entries written before the change above hold a File under `blob`.
interface LegacyStoredZip {
  blob: Blob
  name: string
  savedAt: number
}

export interface CachedZip {
  blob: Blob
  name: string
  savedAt: number
}

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => req.result.createObjectStore(STORE)
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export async function cacheZip(file: File): Promise<void> {
  const data = await file.arrayBuffer()
  const entry: StoredZip = { data, name: file.name, savedAt: Date.now(), type: file.type || 'application/zip' }
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).put(entry, KEY)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export async function loadCachedZip(): Promise<CachedZip | null> {
  try {
    const db = await openDB()
    const entry = await new Promise<StoredZip | LegacyStoredZip | null>((resolve, reject) => {
      const tx = db.transaction(STORE, 'readonly')
      const req = tx.objectStore(STORE).get(KEY)
      req.onsuccess = () => resolve(req.result ?? null)
      req.onerror = () => reject(req.error)
    })
    if (!entry) return null

    if ('data' in entry) {
      if (entry.data.byteLength === 0) {
        await clearCachedZip()
        return null
      }
      return { blob: new Blob([entry.data], { type: entry.type }), name: entry.name, savedAt: entry.savedAt }
    }

    // Legacy entry: read the File's bytes now. A stale handle throws or reads
    // empty here rather than silently uploading nothing, and a successful read
    // is rewritten as bytes so the next load no longer depends on the disk.
    const data = await entry.blob.arrayBuffer()
    if (data.byteLength === 0) {
      await clearCachedZip()
      return null
    }
    const upgraded: StoredZip = { data, name: entry.name, savedAt: entry.savedAt, type: entry.blob.type || 'application/zip' }
    const db2 = await openDB()
    await new Promise<void>(resolve => {
      const tx = db2.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).put(upgraded, KEY)
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
    return { blob: new Blob([data], { type: upgraded.type }), name: entry.name, savedAt: entry.savedAt }
  } catch {
    // An unreadable entry is worse than none: it fails every reload until it is
    // dropped, and the user is never told why.
    await clearCachedZip()
    return null
  }
}

export async function clearCachedZip(): Promise<void> {
  try {
    const db = await openDB()
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).delete(KEY)
      tx.oncomplete = () => resolve()
      tx.onerror = () => reject(tx.error)
    })
  } catch { /* best-effort */ }
}
