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

// Motion stops in-flight animations on unmount. happy-dom rejects animation.finished
// with AbortError, and nothing is waiting on that promise in a unit test.
if (typeof Animation !== 'undefined') {
  const cancel = Animation.prototype.cancel
  Animation.prototype.cancel = function cancelQuietly(this: Animation, ...args: []) {
    void this.finished?.catch(() => undefined)
    return cancel.apply(this, args)
  }
}
