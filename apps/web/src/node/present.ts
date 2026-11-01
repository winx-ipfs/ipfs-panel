import type { MessageSchema } from '@/locales/en'
import type { NodeOverview } from '@/node/types'

export type SentenceKey = keyof MessageSchema
export type NodeAction = 'start' | 'stop' | 'restart' | 'retry'
export type StatusTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

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

export function statusLabelKey(
  overview: NodeOverview | null,
  unreachable: boolean,
): SentenceKey {
  if (unreachable) return 'statusUnreachable'
  if (!overview) return 'nodeLoading'
  switch (overview.state) {
    case 'down':
      return 'statusDown'
    case 'starting':
      return 'statusStarting'
    case 'up':
      return 'statusUp'
    case 'stopping':
      return 'statusStopping'
    case 'failed':
      return 'statusFailed'
    default:
      return 'statusFailed'
  }
}

export function statusTone(overview: NodeOverview | null, unreachable: boolean): StatusTone {
  if (unreachable) return 'warning'
  if (!overview) return 'neutral'
  switch (overview.state) {
    case 'down':
      return 'neutral'
    case 'starting':
      return 'info'
    case 'up':
      return 'success'
    case 'stopping':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'neutral'
  }
}

export function hintKey(overview: NodeOverview | null, unreachable: boolean): SentenceKey | null {
  if (unreachable) return 'nodeHintUnreachable'
  if (!overview) return null
  switch (overview.state) {
    case 'down':
      return 'nodeHintDown'
    case 'starting':
      return 'nodeHintStarting'
    case 'stopping':
      return 'nodeHintStopping'
    default:
      return null
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
      if (overview.error?.code === 'repo_busy') return []
      if (overview.error?.code === 'binary_not_found') return ['retry']
      return ['start']
    default:
      return []
  }
}
