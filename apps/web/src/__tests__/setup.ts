const memory = new Map<string, string>()

const storage = {
  getItem: (key: string) => memory.get(key) ?? null,
  setItem: (key: string, value: string) => {
    memory.set(key, value)
  },
  removeItem: (key: string) => {
    memory.delete(key)
  },
  clear: () => {
    memory.clear()
  },
}

function define(name: string, value: unknown) {
  try {
    Object.defineProperty(globalThis, name, { configurable: true, value })
  } catch {
    // happy-dom already provides this.
  }
}

define('localStorage', storage)
define('navigator', { language: 'en-US' })
