import type { SceneCatalog, SceneItem } from '@/scenes/model'

export type ScenesShot = 'list' | 'preview' | 'applied'

const share: SceneItem = {
  id: 'share',
  active: false,
  restart: true,
  changes: [{ key: 'Provide.Strategy', code: 'mfs' }],
}

export function scenesPreview(search: string): {
  shot: ScenesShot
  catalog: SceneCatalog
  openId: string
  result: '' | 'gateway'
} | null {
  const shot = new URLSearchParams(search).get('scenes')
  if (shot !== 'list' && shot !== 'preview' && shot !== 'applied') return null
  const gatewayChanges =
    shot === 'list'
      ? []
      : [
          { key: 'Provide.Strategy', code: 'pinned+mfs' },
          { key: 'Gateway.NoFetch', code: 'local' },
          { key: 'Gateway.NoDNSLink', code: 'off' },
          { key: 'Gateway.ExposeRoutingAPI', code: 'off' },
          { key: 'Addresses.Gateway', code: 'public' },
        ]
  const catalog: SceneCatalog = {
    ready: true,
    gatewayPort: 8081,
    nodeState: shot === 'applied' ? 'stopping' : 'up',
    revert: shot === 'applied',
    scenes: [
      { id: 'quiet', active: false, restart: true, changes: [{ key: 'Routing.Type', code: 'autoclient' }, { key: 'Swarm.ConnMgr', code: 'quiet' }] },
      share,
      { id: 'pinset', active: false, restart: true, changes: [{ key: 'Provide.Strategy', code: 'pinned' }, { key: 'Gateway.NoFetch', code: 'local' }] },
      {
        id: 'gateway',
        active: shot === 'applied',
        restart: shot !== 'applied',
        changes: shot === 'applied' ? [] : gatewayChanges,
      },
    ],
  }
  return {
    shot,
    catalog,
    openId: shot === 'preview' ? 'gateway' : '',
    result: shot === 'applied' ? 'gateway' : '',
  }
}
