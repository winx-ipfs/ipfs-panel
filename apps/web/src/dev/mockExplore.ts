import type { ExploreView } from '@/explore/model'

export type ExploreShot = 'start' | 'result'

export interface ExplorePreview {
  shot: ExploreShot
  view: ExploreView | null
  draft: string
  blocked: '' | 'down'
}

const root = 'bafybeigccimv3zqm5g4jt363faybagywkvqbrismoquogimy7kvz2sj7sq'
const notes = 'Qmc3zqKcwzbbvw3MQm3hXdg8BQoFjGdZiGdAfXAyAGGdLi'
const meeting = 'QmdMxMx29KVYhHnaCc1icWYxQqXwUNCae6t1wS2NqruiHd'

const folder: ExploreView = {
  target: `/ipfs/${root}/photos`,
  cid: 'QmVmf9vLEdWeBjh74kTibHVkim6iLsRXs5jhHzbSdWjoLt',
  version: 1,
  codec: 'dag-pb',
  hash: 'sha2-256',
  kind: 'folder',
  size: null,
  focus: '',
  text: '',
  data: undefined,
  dataOmitted: false,
  linksTruncated: false,
  localGateway: 'http://127.0.0.1:8081',
  links: [
    { name: 'notes.txt', cid: notes, size: 2480 },
    { name: '会议记录.txt', cid: meeting, size: 18432 },
  ],
}

export function explorePreview(search: string): ExplorePreview | null {
  const shot = new URLSearchParams(search).get('explore')
  if (shot === 'start') {
    return { shot, view: null, draft: '', blocked: '' }
  }
  if (shot === 'result') {
    return { shot, view: folder, draft: `/ipfs/${root}/photos`, blocked: '' }
  }
  if (shot === 'down') {
    return { shot: 'start', view: null, draft: '', blocked: 'down' }
  }
  return null
}
