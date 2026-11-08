type Listener = () => void

const listeners = new Set<Listener>()

/** The router registers this. API stores call notifyUnauthorized on 401. */
export function onUnauthorized(listener: Listener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function notifyUnauthorized() {
  for (const listener of listeners) listener()
}
