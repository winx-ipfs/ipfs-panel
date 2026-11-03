import type { FileEntry, FileList, UploadJob } from '@/files/model'

export type FilesShot = 'list' | 'empty' | 'uploading' | 'delete' | 'down'

export interface FilesPreview {
  shot: FilesShot
  list: FileList
  uploads: UploadJob[]
  blocked: '' | 'down' | 'starting' | 'stopping' | 'failed'
  dialog: '' | 'delete'
}

const gateway = 'http://127.0.0.1:8081'
const notes = 'bafybeigdyrzt5sfp7k4notesfilecid0001'
const photo = 'bafybeigdyrzt5sfp7k4photofolder0001'
const readme = 'bafybeigdyrzt5sfp7k4readmemarkdown01'
const meeting = 'bafybeigdyrzt5sfp7k4meetingnotes0001'

const sample: FileEntry[] = [
  {
    name: 'photos',
    type: 'directory',
    size: 0,
    cid: photo,
    modified: '2026-10-02T08:30:00Z',
  },
  {
    name: 'notes.txt',
    type: 'file',
    size: 2480,
    cid: notes,
    modified: '2026-11-01T03:12:00Z',
  },
  {
    name: '会议记录.txt',
    type: 'file',
    size: 18432,
    cid: meeting,
    modified: '2026-09-18T11:04:00Z',
  },
  {
    name: 'readme.md',
    type: 'file',
    size: 860,
    cid: readme,
    modified: null,
  },
  {
    name: 'vacation-photos-from-the-coast.jpg',
    type: 'file',
    size: 2400000,
    cid: 'bafybeigdyrzt5sfp7k4coastphotoscid1',
    modified: '2026-08-14T16:40:00Z',
  },
]

export function filesPreview(search: string): FilesPreview | null {
  const shot = new URLSearchParams(search).get('files')
  if (shot !== 'list' && shot !== 'empty' && shot !== 'uploading' && shot !== 'delete' && shot !== 'down') {
    return null
  }
  const list: FileList = {
    path: '/',
    localGateway: gateway,
    maxBytes: 512 * 1024 * 1024,
    entries: shot === 'empty' || shot === 'down' ? [] : sample,
  }
  const uploads: UploadJob[] =
    shot === 'uploading'
      ? [
          { id: 1, name: 'notes.txt', percent: 100, status: 'done', error: '' },
          { id: 2, name: 'vacation-photos-from-the-coast.jpg', percent: 62, status: 'uploading', error: '' },
        ]
      : []
  return {
    shot,
    list,
    uploads,
    blocked: shot === 'down' ? 'down' : '',
    dialog: shot === 'delete' ? 'delete' : '',
  }
}

export type { FileEntry }
