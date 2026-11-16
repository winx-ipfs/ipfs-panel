export interface ConfigDocument {
  text: string
  scene: string | null
  sceneKeys: string[]
  backup: boolean
}

export interface ConfigResult extends ConfigDocument {
  restart: 'none' | 'needed'
  diverged: boolean
}

export function jsonErrorLine(text: string): number | null {
  try {
    JSON.parse(text)
    return null
  } catch (error) {
    const message = error instanceof Error ? error.message : ''
    const match = /position (\d+)/.exec(message)
    if (!match?.[1]) return 1
    const position = Number(match[1])
    if (!Number.isFinite(position) || position < 0) return 1
    return text.slice(0, position).split('\n').length
  }
}

export function sceneDiverged(before: string, after: string, scene: string | null, keys: string[]): boolean {
  if (!scene) return false
  let left: unknown
  let right: unknown
  try {
    left = JSON.parse(before)
    right = JSON.parse(after)
  } catch {
    return false
  }
  return keys.some((key) => canon(at(left, key)) !== canon(at(right, key)))
}

export function parseDocument(value: unknown): ConfigDocument | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.text !== 'string') return null
  if (body.scene !== null && typeof body.scene !== 'string') return null
  if (!Array.isArray(body.sceneKeys) || body.sceneKeys.some((key) => typeof key !== 'string')) return null
  if (typeof body.backup !== 'boolean') return null
  return {
    text: stripPrivateKey(body.text),
    scene: body.scene,
    sceneKeys: body.sceneKeys as string[],
    backup: body.backup,
  }
}

export function parseResult(value: unknown): ConfigResult | null {
  const doc = parseDocument(value)
  if (!doc || !value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (body.restart !== 'none' && body.restart !== 'needed') return null
  if (typeof body.diverged !== 'boolean') return null
  return { ...doc, restart: body.restart, diverged: body.diverged }
}

export function configErrorKey(code: string): string {
  switch (code) {
    case 'invalid_json':
      return 'configErrorInvalid'
    case 'not_object':
      return 'configErrorObject'
    case 'privkey_locked':
      return 'configErrorPriv'
    case 'secret_locked':
      return 'configErrorSecret'
    case 'api_locked':
      return 'configErrorAPI'
    case 'identity_locked':
      return 'configErrorIdentity'
    case 'api_not_loopback':
    case 'gateway_not_loopback':
      return 'configErrorGateway'
    case 'gateway_not_readonly':
      return 'configErrorReadonly'
    case 'panel_port_conflict':
      return 'configErrorPort'
    case 'kubo_rejected':
      return 'configErrorKubo'
    case 'nothing_to_restore':
      return 'configErrorRestore'
    case 'too_large':
      return 'configErrorLarge'
    case 'node_not_running':
      return 'configErrorStopped'
    case 'network':
    case 'timeout':
    case 'unavailable':
      return 'configErrorNetwork'
    default:
      return 'configError'
  }
}

export function sceneTitleKey(id: string): string {
  switch (id) {
    case 'quiet':
      return 'scenesQuietTitle'
    case 'share':
      return 'scenesShareTitle'
    case 'pinset':
      return 'scenesPinsetTitle'
    case 'gateway':
      return 'scenesGatewayTitle'
    default:
      return 'configNoScene'
  }
}

function at(value: unknown, path: string): unknown {
  let current = value
  for (const part of path.split('.')) {
    if (!current || typeof current !== 'object' || Array.isArray(current)) return undefined
    current = (current as Record<string, unknown>)[part]
  }
  return current
}

function canon(value: unknown): string {
  if (value === undefined) return 'null'
  return JSON.stringify(value) ?? 'null'
}

function stripPrivateKey(text: string): string {
  try {
    const doc = JSON.parse(text) as { Identity?: Record<string, unknown> }
    if (doc?.Identity && Object.prototype.hasOwnProperty.call(doc.Identity, 'PrivKey')) {
      delete doc.Identity.PrivKey
      return `${JSON.stringify(doc, null, 2)}\n`
    }
  } catch {
    return text
  }
  return text
}
