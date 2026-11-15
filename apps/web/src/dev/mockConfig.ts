export type ConfigShot = 'loading' | 'ready' | 'dirty' | 'invalid' | 'diff' | 'saved' | 'stopped' | 'error'

const shots = new Set<ConfigShot>(['loading', 'ready', 'dirty', 'invalid', 'diff', 'saved', 'stopped', 'error'])

export function configShot(search: string): ConfigShot | null {
  const value = new URLSearchParams(search).get('config')
  if (value && shots.has(value as ConfigShot)) return value as ConfigShot
  return null
}
