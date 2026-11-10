export type NodeState = 'down' | 'starting' | 'up' | 'stopping' | 'failed'

export interface NodeError {
  code: string
  detail: string
}

export interface NodeActivity {
  peers: number | null
  rateIn: number | null
  rateOut: number | null
}

export interface NodeOverview {
  state: NodeState
  peerId: string | null
  version: string | null
  repoPath: string
  error: NodeError | null
  activity?: NodeActivity | null
}
