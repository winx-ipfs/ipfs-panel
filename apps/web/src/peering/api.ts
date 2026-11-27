import { apiJSON } from '@/lib/api'
import { parsePermanent, type PermanentList } from '@/peering/model'

export interface PeeringResult {
  list: PermanentList | null
  error: string
  state: string
  status: number
}

async function call(path: string, body?: unknown): Promise<PeeringResult> {
  const result = await apiJSON(path, body === undefined ? undefined : { method: 'POST', body })
  if (result.status === 401) return { list: null, error: 'unauthorized', state: '', status: 401 }
  const list = result.status === 200 ? parsePermanent(result.body) : null
  return {
    list,
    error: result.error,
    state: result.state,
    status: result.status,
  }
}

export function fetchPermanent(): Promise<PeeringResult> {
  return call('/api/peers/permanent')
}

export function addPermanent(body: { address: string } | { peerId: string }): Promise<PeeringResult> {
  return call('/api/peers/permanent', body)
}

export function removePermanent(peerId: string): Promise<PeeringResult> {
  return call('/api/peers/permanent/remove', { peerId })
}

export function applyPermanent(): Promise<PeeringResult> {
  return call('/api/peers/permanent/apply', {})
}
