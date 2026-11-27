import { apiJSON } from '@/lib/api'
import { parseJob, parseKubo, type KuboJob, type KuboStatus } from '@/kubo/model'

export async function fetchKubo(): Promise<KuboStatus | null> {
  const result = await apiJSON('/api/kubo')
  if (result.status !== 200) return null
  return parseKubo(result.body)
}

export async function postKubo(action: 'install' | 'upgrade'): Promise<KuboJob | null> {
  const result = await apiJSON(`/api/kubo/${action}`, { method: 'POST', body: {} })
  if (result.status !== 202) return null
  return parseJob(result.body)
}
