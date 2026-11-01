import type { MessageSchema } from '@/locales/en'
import type { NodeOverview } from '@/node/types'

export type SentenceKey = keyof MessageSchema
export type NodeAction = 'start' | 'stop' | 'restart' | 'retry'

const failedSentences: Record<string, SentenceKey> = {
  binary_not_found: 'nodeErrorBinary',
  repo_not_writable: 'nodeErrorRepoWrite',
  init_failed: 'nodeErrorInit',
  api_not_loopback: 'nodeErrorApiPublic',
  gateway_not_loopback: 'nodeErrorGatewayPublic',
  panel_port_conflict: 'nodeErrorPanelPort',
  port_in_use: 'nodeErrorPortInUse',
  repo_busy: 'nodeErrorRepoBusy',
  daemon_exited: 'nodeErrorDaemonExited',
  health_timeout: 'nodeErrorHealthTimeout',
  unknown: 'nodeErrorUnknown',
}

export function sentenceKey(
  overview: NodeOverview | null,
  unreachable: boolean,
): SentenceKey | null {
  if (unreachable) return 'nodeUnreachable'
  if (!overview) return null
  switch (overview.state) {
    case 'down':
      return overview.error?.code === 'stop_timeout' ? 'nodeStopForced' : 'nodeStopped'
    case 'starting':
      return 'nodeStarting'
    case 'up':
      return 'nodeRunning'
    case 'stopping':
      return 'nodeStopping'
    case 'failed':
      return failedSentences[overview.error?.code ?? ''] ?? 'nodeErrorUnknown'
    default:
      return 'nodeErrorUnknown'
  }
}

export function visibleActions(
  overview: NodeOverview | null,
  unreachable: boolean,
): NodeAction[] {
  if (unreachable) return ['retry']
  if (!overview) return []
  switch (overview.state) {
    case 'down':
      return ['start']
    case 'up':
      return ['stop', 'restart']
    case 'failed':
      return overview.error?.code === 'repo_busy' ? [] : ['start']
    default:
      return []
  }
}
