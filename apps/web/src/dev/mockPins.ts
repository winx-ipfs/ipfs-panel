import type { Pin, PinJob, PinList } from '@/pins/model'

export type PinsShot = 'list' | 'empty' | 'add' | 'job' | 'down'

export interface PinsPreview {
  shot: PinsShot
  list: PinList
  jobs: PinJob[]
  blocked: '' | 'down' | 'starting' | 'stopping' | 'failed'
  dialog: '' | 'add'
  draft: string
}

const notes = 'bafkreibceirceirceirceirceirceirceirceirceirceirceirceircei'
const photo = 'QmPVGjYFugq4XUyBfoTHG6c3qxfBS26jEdaFM1gdAVuMZ2'
const meeting = 'bafkreibceirceirceirceirceirceirceirceirceirceirceirceircem'
const coast = 'bafybeigdyrzt5sfp7k4coastphotoscidexample000000000000000000'

const sample: Pin[] = [
  { cid: notes, name: 'notes.txt', type: 'recursive', size: 2480 },
  { cid: meeting, name: '会议记录.txt', type: 'recursive', size: 18432 },
  { cid: photo, name: 'photos', type: 'direct', size: 4096 },
  { cid: coast, name: '', type: 'recursive', size: null },
]

export function pinsPreview(search: string): PinsPreview | null {
  const shot = new URLSearchParams(search).get('pins')
  if (shot !== 'list' && shot !== 'empty' && shot !== 'add' && shot !== 'job' && shot !== 'down') return null
  const list: PinList = {
    namesFrom: shot === 'list' || shot === 'add' || shot === 'job' ? 'kubo' : 'panel',
    localGateway: 'http://127.0.0.1:8081',
    maxBytes: 512 * 1024 * 1024,
    pins: shot === 'empty' || shot === 'down' ? [] : sample,
  }
  const jobs: PinJob[] =
    shot === 'job'
      ? [
          {
            id: '0123456789abcdef0123456789abcdef',
            cid: notes,
            name: 'notes.txt',
            type: 'recursive',
            state: 'running',
            progress: 12,
            error: '',
            created: '2026-11-01T03:12:00Z',
          },
        ]
      : []
  return {
    shot,
    list,
    jobs,
    blocked: shot === 'down' ? 'down' : '',
    dialog: shot === 'add' ? 'add' : '',
    draft: shot === 'add' ? 'not-a-cid' : '',
  }
}
