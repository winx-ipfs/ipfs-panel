import type { NodeOverview } from '@/node/types'

const repoPath = '/home/me/.local/share/ipfs-panel/kubo'
const peerId = '12D3KooWRjS6k8iG8m3mS5kQ8o8n8Yw8vQd7pQh2oE4mN1aB2cD'
const version = 'kubo/0.32.1/'

export function devMockFromLocation(
  search: string,
): { overview: NodeOverview | null; unreachable: boolean } | null {
  const mock = new URLSearchParams(search).get('mock')
  if (!mock) return null
  if (mock === 'unreachable') {
    return {
      unreachable: true,
      overview: { state: 'up', peerId, version, repoPath, error: null },
    }
  }
  const base = { peerId: null, version: null, repoPath, error: null as NodeOverview['error'] }
  const states: Record<string, NodeOverview> = {
    down: { ...base, state: 'down' },
    starting: { ...base, state: 'starting' },
    stopping: { ...base, state: 'stopping' },
    up: { ...base, state: 'up', peerId, version, activity: { peers: 12, rateIn: 1536, rateOut: 420 } },
    failed: {
      ...base,
      state: 'failed',
      error: { code: 'binary_not_found', detail: 'ipfs not found on PATH' },
    },
  }
  const overview = states[mock]
  if (!overview) return null
  return { overview, unreachable: false }
}
