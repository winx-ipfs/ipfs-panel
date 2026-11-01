export type NodeState = 'down' | 'starting' | 'up' | 'stopping' | 'failed'

export interface NodeError {
  code: string
  detail: string
}

export interface NodeOverview {
  state: NodeState
  peerId: string | null
  version: string | null
  repoPath: string
  error: NodeError | null
}
