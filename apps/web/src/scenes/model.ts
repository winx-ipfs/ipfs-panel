import type { MessageSchema } from '@/locales/en'

export type SceneID = 'quiet' | 'share' | 'pinset' | 'gateway'

export interface SceneChange {
  key: string
  code: string
}

export interface SceneItem {
  id: string
  active: boolean
  restart: boolean
  changes: SceneChange[]
}

export interface SceneCatalog {
  ready: boolean
  gatewayPort: number
  nodeState: string
  revert: boolean
  scenes: SceneItem[]
}

type Key = keyof MessageSchema

const titles: Record<SceneID, Key> = {
  quiet: 'scenesQuietTitle',
  share: 'scenesShareTitle',
  pinset: 'scenesPinsetTitle',
  gateway: 'scenesGatewayTitle',
}

const summaries: Record<SceneID, Key> = {
  quiet: 'scenesQuietSummary',
  share: 'scenesShareSummary',
  pinset: 'scenesPinsetSummary',
  gateway: 'scenesGatewaySummary',
}

export function isSceneID(value: string): value is SceneID {
  return value === 'quiet' || value === 'share' || value === 'pinset' || value === 'gateway'
}

export function sceneTitleKey(id: string): Key {
  return isSceneID(id) ? titles[id] : 'scenesTitle'
}

export function sceneSummaryKey(id: string): Key {
  return isSceneID(id) ? summaries[id] : 'scenesDescription'
}

export function resultKey(id: string, restart: string): Key {
  if (restart === 'none') return 'scenesResultNone'
  const later = restart === 'later'
  if (id === 'revert') return later ? 'scenesResultRevertedLater' : 'scenesResultRevertedStarted'
  switch (id) {
    case 'quiet':
      return later ? 'scenesResultQuietLater' : 'scenesResultQuietStarted'
    case 'share':
      return later ? 'scenesResultShareLater' : 'scenesResultShareStarted'
    case 'pinset':
      return later ? 'scenesResultPinsetLater' : 'scenesResultPinsetStarted'
    case 'gateway':
      return later ? 'scenesResultGatewayLater' : 'scenesResultGatewayStarted'
    default:
      return 'scenesResultNone'
  }
}

export function changePhraseKey(change: SceneChange): Key | null {
  switch (`${change.key}:${change.code}`) {
    case 'Routing.Type:autoclient':
      return 'scenesChangeRoutingQuiet'
    case 'Routing.Type:auto':
      return 'scenesChangeRoutingAuto'
    case 'AutoNAT.ServiceMode:disabled':
      return 'scenesChangeAutonatOff'
    case 'AutoNAT.ServiceMode:enabled':
      return 'scenesChangeAutonatOn'
    case 'Swarm.RelayService.Enabled:off':
      return 'scenesChangeRelayOff'
    case 'Swarm.RelayService.Enabled:on':
      return 'scenesChangeRelayOn'
    case 'Swarm.ConnMgr:quiet':
      return 'scenesChangeConnQuiet'
    case 'Swarm.ConnMgr:usual':
      return 'scenesChangeConnUsual'
    case 'Provide.Enabled:on':
      return 'scenesChangeProvideOn'
    case 'Provide.Enabled:off':
      return 'scenesChangeProvideOff'
    case 'Provide.Strategy:all':
      return 'scenesChangeStrategyAll'
    case 'Provide.Strategy:mfs':
      return 'scenesChangeStrategyMfs'
    case 'Provide.Strategy:pinned':
      return 'scenesChangeStrategyPinned'
    case 'Provide.Strategy:pinned+mfs':
      return 'scenesChangeStrategyBoth'
    case 'Gateway.NoFetch:local':
      return 'scenesChangeFetchLocal'
    case 'Gateway.NoFetch:fetch':
      return 'scenesChangeFetchRemote'
    case 'Gateway.NoDNSLink:off':
      return 'scenesChangeDNSOff'
    case 'Gateway.NoDNSLink:on':
      return 'scenesChangeDNSOn'
    case 'Gateway.ExposeRoutingAPI:off':
      return 'scenesChangeRoutingAPIOff'
    case 'Gateway.ExposeRoutingAPI:on':
      return 'scenesChangeRoutingAPIOn'
    case 'Addresses.Gateway:public':
      return 'scenesChangeGatewayPublic'
    case 'Addresses.Gateway:local':
      return 'scenesChangeGatewayLocal'
    default:
      return null
  }
}

export function sceneErrorKey(code: string): Key {
  switch (code) {
    case 'not_ready':
      return 'scenesErrorNotReady'
    case 'node_busy':
      return 'scenesErrorBusy'
    case 'api_not_loopback':
      return 'scenesErrorAPI'
    case 'gateway_not_readonly':
      return 'scenesErrorReadonly'
    case 'nothing_to_revert':
      return 'scenesErrorUndo'
    case 'network':
      return 'scenesErrorNetwork'
    default:
      return 'scenesErrorUnknown'
  }
}

export function parseCatalog(value: unknown): SceneCatalog | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (!Array.isArray(body.scenes)) return null
  const scenes: SceneItem[] = []
  for (const item of body.scenes) {
    if (!item || typeof item !== 'object') return null
    const row = item as Record<string, unknown>
    if (typeof row.id !== 'string') return null
    const changes: SceneChange[] = []
    if (Array.isArray(row.changes)) {
      for (const change of row.changes) {
        if (!change || typeof change !== 'object') continue
        const raw = change as Record<string, unknown>
        if (typeof raw.key === 'string' && typeof raw.code === 'string') {
          changes.push({ key: raw.key, code: raw.code })
        }
      }
    }
    scenes.push({
      id: row.id,
      active: row.active === true,
      restart: row.restart === true,
      changes,
    })
  }
  return {
    ready: body.ready === true,
    gatewayPort: typeof body.gatewayPort === 'number' ? body.gatewayPort : 8081,
    nodeState: typeof body.nodeState === 'string' ? body.nodeState : 'down',
    revert: body.revert === true,
    scenes,
  }
}
