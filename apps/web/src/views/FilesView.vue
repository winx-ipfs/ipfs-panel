<script setup lang="ts">
import { ChevronRight, FileText, Folder, FolderPlus, Home, MoreHorizontal, RefreshCw, Upload } from '@lucide/vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import Dialog from '@/components/Dialog.vue'
import EmptyState from '@/components/EmptyState.vue'
import IconButton from '@/components/IconButton.vue'
import Input from '@/components/Input.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import {
  cleanPath,
  copyName,
  crumbs,
  entryPath,
  formatBytes,
  joinPath,
  localShareURL,
  parentPath,
  sortEntries,
  validName,
  type FileEntry,
  type SortDir,
  type SortKey,
} from '@/files/model'
import { shortenMiddle } from '@/lib/inline'
import { useFilesStore } from '@/stores/files'
import { usePinsStore } from '@/stores/pins'
import { useToastStore } from '@/stores/toast'

const { t, locale } = useI18n()
const route = useRoute()
const store = useFilesStore()
const pins = usePinsStore()
const toast = useToastStore()

const sortKeys = ['name', 'size', 'modified'] as const
const sortKey = ref<SortKey>('name')
const sortDir = ref<SortDir>('asc')
const dialog = ref<'mkdir' | 'rename' | 'move' | 'copy' | 'delete' | 'replace' | null>(null)
const draft = ref('')
const folderDraft = ref('/')
const target = ref<FileEntry | null>(null)
const replaceFile = ref<File | null>(null)
const replaceRest = ref<File[]>([])
const menuFor = ref('')
const menuWrap = ref<HTMLElement | null>(null)
const dragDepth = ref(0)
const picker = ref<HTMLInputElement | null>(null)
let menuOpener: HTMLElement | null = null

const sorted = computed(() => sortEntries(store.entries, sortKey.value, sortDir.value))
const trail = computed(() => crumbs(store.path))
const dragging = computed(() => dragDepth.value > 0)
const dialogOpen = computed({
  get: () => dialog.value !== null,
  set: (value: boolean) => {
    if (!value) closeDialog()
  },
})

const marker = computed(() => {
  if (dialog.value === 'delete' && target.value) return 'delete'
  if (store.uploads.some((job) => job.status === 'uploading')) return 'uploading'
  if (store.status === 'blocked') return 'down'
  if (store.status === 'ready' && store.entries.length === 0) return 'empty'
  if (store.status === 'ready') return 'list'
  return ''
})

const blockedTitle = computed(() => {
  if (store.nodeState === 'starting') return t('filesNodeStartingTitle')
  if (store.nodeState === 'stopping') return t('filesNodeStoppingTitle')
  if (store.nodeState === 'failed') return t('filesNodeFailedTitle')
  return t('filesNodeDownTitle')
})

const blockedBody = computed(() => {
  if (store.nodeState === 'starting') return t('filesNodeStartingBody')
  if (store.nodeState === 'stopping') return t('filesNodeStoppingBody')
  if (store.nodeState === 'failed') return t('filesNodeFailedBody')
  return t('filesNodeDownBody')
})

const dialogTitle = computed(() => {
  const name = target.value?.name ?? replaceFile.value?.name ?? ''
  if (dialog.value === 'mkdir') return t('filesNewFolderTitle')
  if (dialog.value === 'rename') return t('filesRenameTitle')
  if (dialog.value === 'move') return t('filesMoveTitle', { name })
  if (dialog.value === 'copy') return t('filesDuplicateTitle', { name })
  if (dialog.value === 'delete') return t('filesDeleteTitle', { name })
  if (dialog.value === 'replace') return t('filesReplaceTitle')
  return ''
})

const nameValid = computed(() => validName(draft.value) && draft.value !== target.value?.name)
const folderValid = computed(() => {
  if (!target.value) return false
  const next = joinPath(folderDraft.value, target.value.name)
  const current = entryPath(store.path, target.value)
  return Boolean(next && current && next !== current)
})
const mkdirValid = computed(() => validName(draft.value))

function sortText(key: SortKey) {
  if (key === 'name') return t('filesSortName')
  if (key === 'size') return t('filesSortSize')
  return t('filesSortModified')
}

function sortLabel(key: SortKey) {
  if (sortKey.value !== key) return sortText(key)
  const dir = sortDir.value === 'asc' ? t('filesSortAsc') : t('filesSortDesc')
  return `${sortText(key)}, ${dir}`
}

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = key === 'name' ? 'asc' : 'desc'
}

function formatWhen(value: string | null) {
  if (!value) return t('filesModifiedUnknown')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return t('filesModifiedUnknown')
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-Hans' : 'en', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function sizeText(entry: FileEntry) {
  return entry.type === 'directory' ? '—' : formatBytes(entry.size)
}

function hrefFor(path: string) {
  const query: Record<string, string> = {}
  if (path !== '/') query.path = path
  const shot = route.query.files
  if (typeof shot === 'string') query.files = shot
  return { name: 'files' as const, query }
}

function downloadHref(entry: FileEntry) {
  const path = entryPath(store.path, entry)
  return path ? `/api/files/download?path=${encodeURIComponent(path)}` : ''
}

function problemText(code: string, name: string) {
  if (code === 'invalid_path') return t('filesErrorName')
  if (code === 'not_found') return t('filesErrorNotFound')
  if (code === 'already_exists') return t('filesErrorExists', { name })
  if (code === 'too_large') return t('filesErrorLarge', { name, size: formatBytes(store.maxBytes) })
  if (code === 'timeout') return t('filesErrorTimeout')
  if (code === 'network') return t('filesErrorNetwork')
  if (code === 'too_many') return t('pinsErrorTooMany')
  return t('filesErrorGeneric')
}

function announce(code: string, name: string) {
  if (!code || code === 'node_not_running' || code === 'unauthorized') return
  toast.show(problemText(code, name))
}

async function copyText(value: string, message: string) {
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    const area = document.createElement('textarea')
    area.value = value
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.left = '-9999px'
    document.body.appendChild(area)
    area.select()
    document.execCommand('copy')
    area.remove()
  }
  toast.show(message)
}

function closeMenu(restore: boolean) {
  menuFor.value = ''
  if (restore) menuOpener?.focus()
  menuOpener = null
}

function toggleMenu(key: string, event: MouseEvent) {
  if (menuFor.value === key) {
    closeMenu(false)
    return
  }
  const opener = event.currentTarget
  menuOpener = opener instanceof HTMLElement ? opener : null
  const wrap = menuOpener?.closest('.files-menu-wrap')
  menuWrap.value = wrap instanceof HTMLElement ? wrap : null
  menuFor.value = key
  void nextTick(() => {
    menuWrap.value?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
  })
}

function onMenuKey(event: KeyboardEvent) {
  const items = [...(menuWrap.value?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])]
  const index = items.findIndex((item) => item === document.activeElement)
  if (event.key === 'Escape') {
    event.preventDefault()
    closeMenu(true)
    return
  }
  if (event.key === 'Tab') {
    menuFor.value = ''
    menuOpener = null
    return
  }
  if (!items.length) return
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp' && event.key !== 'Home' && event.key !== 'End') return
  event.preventDefault()
  let next = 0
  if (event.key === 'End') next = items.length - 1
  else if (event.key === 'ArrowDown') next = (Math.max(index, 0) + 1) % items.length
  else if (event.key === 'ArrowUp') next = (index <= 0 ? items.length : index) - 1
  items[next]?.focus()
}

function onPointer(event: Event) {
  if (!menuFor.value || !menuWrap.value) return
  if (event.target instanceof Node && menuWrap.value.contains(event.target)) return
  menuFor.value = ''
  menuOpener = null
}

function openDialog(kind: 'mkdir' | 'rename' | 'move' | 'copy' | 'delete', entry: FileEntry | null) {
  closeMenu(false)
  target.value = entry
  if (kind === 'rename' && entry) draft.value = entry.name
  else if (kind === 'copy' && entry) draft.value = copyName(entry.name)
  else if (kind === 'mkdir') draft.value = ''
  else draft.value = ''
  folderDraft.value = parentPath(store.path === '/' ? '/keep' : store.path)
  if (kind === 'move') folderDraft.value = '/'
  dialog.value = kind
}

function closeDialog() {
  dialog.value = null
  target.value = null
  replaceFile.value = null
  replaceRest.value = []
  draft.value = ''
}

async function submitMkdir() {
  if (!mkdirValid.value) return
  const name = draft.value
  closeDialog()
  const result = await store.mkdir(name)
  if (!result.ok) announce(result.error, name)
}

async function submitRename() {
  if (!target.value || !nameValid.value) return
  const entry = target.value
  const name = draft.value
  closeDialog()
  const result = await store.rename(entry, name)
  if (!result.ok) announce(result.error, name)
}

async function submitMove() {
  if (!target.value || !folderValid.value) return
  const entry = target.value
  const folder = cleanPath(folderDraft.value)
  if (!folder) return
  closeDialog()
  const result = await store.move(entry, folder)
  if (!result.ok) announce(result.error, entry.name)
}

async function submitCopy() {
  if (!target.value || !validName(draft.value)) return
  const entry = target.value
  const name = draft.value
  closeDialog()
  const result = await store.copy(entry, name)
  if (!result.ok) announce(result.error, name)
}

async function submitDelete() {
  if (!target.value) return
  const entry = target.value
  closeDialog()
  const result = await store.remove(entry)
  if (!result.ok) announce(result.error, entry.name)
}

async function runQueue(list: File[], overwriteFirst: boolean) {
  let overwrite = overwriteFirst
  for (let index = 0; index < list.length; index += 1) {
    const file = list[index]
    if (!file) continue
    const result = await store.uploadOne(file, overwrite)
    overwrite = false
    if (result.status === 409) {
      replaceFile.value = file
      replaceRest.value = list.slice(index + 1)
      dialog.value = 'replace'
      return
    }
    if (!result.ok) announce(result.error, file.name)
  }
  await store.load(store.path)
}

async function sendFiles(list: File[]) {
  if (!list.length || store.status === 'blocked') return
  store.beginUploads()
  await runQueue(list, false)
}

async function confirmReplace() {
  const file = replaceFile.value
  const rest = replaceRest.value
  dialog.value = null
  replaceFile.value = null
  if (!file) return
  store.uploads = store.uploads.filter((job) => !(job.name === file.name && job.status === 'error'))
  await runQueue([file, ...rest], true)
}

function onPick(event: Event) {
  const input = event.target
  if (!(input instanceof HTMLInputElement) || !input.files) return
  void sendFiles([...input.files])
  input.value = ''
}

function onDragEnter(event: DragEvent) {
  event.preventDefault()
  dragDepth.value += 1
}

function onDragOver(event: DragEvent) {
  event.preventDefault()
}

function onDragLeave() {
  dragDepth.value = Math.max(0, dragDepth.value - 1)
}

function onDrop(event: DragEvent) {
  event.preventDefault()
  dragDepth.value = 0
  void sendFiles([...(event.dataTransfer?.files ?? [])])
}

async function pinEntry(entry: FileEntry) {
  const path = entryPath(store.path, entry)
  if (!path) return
  closeMenu(false)
  const result = await pins.pinPath(path, entry.name)
  if (result.ok) toast.show(t('filesPinStarted', { name: entry.name }))
  else announce(result.error, entry.name)
}

function copyCid(entry: FileEntry) {
  if (!entry.cid) return
  void copyText(entry.cid, t('filesCopiedCid'))
}

function copyLink(entry: FileEntry) {
  const url = localShareURL(store.localGateway, entry.cid)
  if (!url) return
  closeMenu(false)
  void copyText(url, t('filesLocalCopied'))
}

async function refresh() {
  const ok = await store.load(store.path)
  if (!ok && store.status === 'unreachable') toast.show(t('filesErrorNetwork'))
}

async function openRoute() {
  if (import.meta.env.DEV) {
    const { filesPreview } = await import('@/dev/mockFiles')
    const shot = filesPreview(window.location.search)
    if (shot) {
      store.showPreview(shot.list, shot.uploads, shot.blocked)
      if (shot.dialog === 'delete') {
        const file = shot.list.entries.find((entry) => entry.type === 'file') ?? null
        target.value = file
        dialog.value = file ? 'delete' : null
      }
      return
    }
  }
  const raw = typeof route.query.path === 'string' ? route.query.path : '/'
  const ok = await store.load(raw)
  if (!ok && store.status === 'ready') toast.show(t('filesErrorName'))
}

watch(() => route.fullPath, () => {
  menuFor.value = ''
  void openRoute()
}, { immediate: true })

onMounted(() => document.addEventListener('pointerdown', onPointer))
onUnmounted(() => document.removeEventListener('pointerdown', onPointer))
</script>

<template>
  <div class="files-page" :data-files="marker">
    <PageHeader :title="t('filesTitle')" :description="t('filesDescription')" />
    <details class="ui-disclosure">
      <summary>
        <ChevronRight class="ui-disclosure-chevron" :size="16" :stroke-width="1.75" aria-hidden="true" />
        <span>{{ t('filesAdvancedSummary') }}</span>
      </summary>
      <p>{{ t('filesAdvancedBody') }}</p>
    </details>

    <EmptyState
      v-if="store.status === 'unreachable'"
      :title="t('filesUnreachableTitle')"
      :description="t('filesUnreachableBody')"
    >
      <template #actions>
        <Button variant="primary" @click="refresh">{{ t('filesRetry') }}</Button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="store.status === 'blocked'"
      :title="blockedTitle"
      :description="blockedBody"
    >
      <template #icon>
        <Folder :size="22" :stroke-width="1.75" aria-hidden="true" />
      </template>
      <template #actions>
        <RouterLink class="ui-btn is-primary" to="/">{{ t('filesGoNode') }}</RouterLink>
      </template>
    </EmptyState>

    <Card v-else>
      <div class="files-toolbar">
        <div class="files-toolbar-main">
          <Button variant="primary" @click="picker?.click()">
            <Upload :size="18" :stroke-width="1.75" aria-hidden="true" />
            {{ t('filesUpload') }}
          </Button>
          <Button variant="quiet" @click="openDialog('mkdir', null)">
            <FolderPlus :size="18" :stroke-width="1.75" aria-hidden="true" />
            {{ t('filesNewFolder') }}
          </Button>
          <IconButton :label="t('filesRefresh')" @click="refresh">
            <RefreshCw :size="18" :stroke-width="1.75" aria-hidden="true" />
          </IconButton>
          <input
            ref="picker"
            class="sr-only"
            tabindex="-1"
            aria-hidden="true"
            type="file"
            multiple
            @change="onPick"
          >
        </div>
        <div class="files-sort" role="group" :aria-label="t('filesSort')">
          <button
            v-for="key in sortKeys"
            :key="key"
            type="button"
            class="files-sort-btn"
            :class="{ 'is-active': sortKey === key }"
            :aria-pressed="sortKey === key"
            :aria-label="sortLabel(key)"
            @click="toggleSort(key)"
          >
            {{ sortText(key) }}
            <span v-if="sortKey === key" aria-hidden="true">{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
          </button>
        </div>
      </div>
      <p class="files-limit">{{ t('filesLimit', { size: formatBytes(store.maxBytes) }) }}</p>

      <ul v-if="store.uploads.length" class="files-uploads" :aria-label="t('filesUploads')">
        <li v-for="job in store.uploads" :key="job.id">
          <div class="files-upload-row">
            <span>{{ job.name }}</span>
            <span class="files-upload-state">
              {{ job.status === 'uploading' ? `${job.percent}%` : job.status === 'done' ? t('filesDone') : job.status === 'queued' ? t('filesQueued') : t('filesFailed') }}
            </span>
          </div>
          <progress
            v-if="job.status === 'uploading'"
            :value="job.percent"
            max="100"
            :aria-label="t('filesUploading', { name: job.name })"
          />
        </li>
      </ul>

      <nav class="files-crumbs" :aria-label="t('filesBreadcrumb')">
        <ol>
          <li v-for="(crumb, index) in trail" :key="crumb.path">
            <ChevronRight v-if="index > 0" :size="14" :stroke-width="1.75" aria-hidden="true" />
            <span v-if="index === trail.length - 1" aria-current="page">
              <Home v-if="index === 0" class="files-crumb-home" :size="16" :stroke-width="1.75" aria-hidden="true" />
              <span v-if="index === 0" class="sr-only">{{ t('filesRoot') }}</span>
              <template v-else>{{ crumb.name }}</template>
            </span>
            <RouterLink v-else :to="hrefFor(crumb.path)">
              <Home v-if="index === 0" class="files-crumb-home" :size="16" :stroke-width="1.75" aria-hidden="true" />
              <span v-if="index === 0" class="sr-only">{{ t('filesRoot') }}</span>
              <template v-else>{{ crumb.name }}</template>
            </RouterLink>
          </li>
        </ol>
      </nav>

      <div
        class="files-drop"
        :class="{ 'is-over': dragging }"
        @dragenter="onDragEnter"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="onDrop"
      >
        <p v-if="dragging" class="files-drop-hint">{{ t('filesDrop') }}</p>
        <div v-if="store.status === 'loading'" class="files-loading" role="status">
          <span class="sr-only">{{ t('filesLoading') }}</span>
          <Skeleton v-for="row in 4" :key="row" height="42px" />
        </div>
        <EmptyState
          v-else-if="store.entries.length === 0"
          :title="t('filesEmptyTitle')"
          :description="t('filesEmptyBody')"
        />
        <table v-else class="files-table">
          <caption class="sr-only">{{ t('filesTitle') }}</caption>
          <thead>
            <tr>
              <th scope="col">{{ t('filesName') }}</th>
              <th scope="col">{{ t('filesSize') }}</th>
              <th scope="col">{{ t('filesCid') }}</th>
              <th scope="col">{{ t('filesModified') }}</th>
              <th scope="col"><span class="sr-only">{{ t('filesMenuLabel') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="entry in sorted" :key="entry.name">
              <th scope="row" class="files-name-cell">
                <span class="files-name">
                  <Folder v-if="entry.type === 'directory'" :size="18" :stroke-width="1.75" aria-hidden="true" />
                  <FileText v-else :size="18" :stroke-width="1.75" aria-hidden="true" />
                  <RouterLink
                    v-if="entry.type === 'directory'"
                    :to="hrefFor(entryPath(store.path, entry) || '/')"
                    :aria-label="t('filesOpenFolder', { name: entry.name })"
                  >
                    {{ entry.name }}
                  </RouterLink>
                  <span v-else>{{ entry.name }}</span>
                  <span class="sr-only">{{ entry.type === 'directory' ? t('filesFolder') : t('filesFile') }}</span>
                </span>
              </th>
              <td class="files-size">
                <span class="files-kicker" aria-hidden="true">{{ t('filesSize') }}</span>
                {{ sizeText(entry) }}
              </td>
              <td class="files-cid">
                <span class="files-kicker" aria-hidden="true">{{ t('filesCid') }}</span>
                <code v-if="entry.cid">{{ shortenMiddle(entry.cid, 8, 6) }}</code>
                <span v-else>{{ t('filesModifiedUnknown') }}</span>
                <button v-if="entry.cid" type="button" class="files-text-btn" :aria-label="t('filesCopyCid')" @click="copyCid(entry)">
                  {{ t('filesCopy') }}
                </button>
              </td>
              <td class="files-time">
                <span class="files-kicker" aria-hidden="true">{{ t('filesModified') }}</span>
                {{ formatWhen(entry.modified) }}
              </td>
              <td class="files-meta">
                <span class="sr-only">{{ t('filesSize') }} {{ sizeText(entry) }}, {{ t('filesModified') }} {{ formatWhen(entry.modified) }}</span>
                <span aria-hidden="true">{{ sizeText(entry) }} · {{ formatWhen(entry.modified) }}</span>
              </td>
              <td class="files-actions">
                <div class="files-menu-wrap">
                  <IconButton :label="t('filesActions', { name: entry.name })" @click="toggleMenu(entry.name, $event)">
                    <MoreHorizontal :size="18" :stroke-width="1.75" aria-hidden="true" />
                  </IconButton>
                  <div
                    v-if="menuFor === entry.name"
                    class="files-menu"
                    role="menu"
                    :aria-label="t('filesMenuLabel')"
                    @keydown="onMenuKey"
                  >
                    <a
                      v-if="entry.type === 'file'"
                      role="menuitem"
                      :href="downloadHref(entry)"
                      :download="entry.name"
                    >
                      {{ t('filesDownload') }}
                    </a>
                    <button v-if="entry.cid && localShareURL(store.localGateway, entry.cid)" type="button" role="menuitem" @click="copyLink(entry)">
                      {{ t('filesLocalLink') }}
                    </button>
                    <button v-if="entry.cid" type="button" class="files-menu-cid" role="menuitem" :aria-label="t('filesCopyCid')" @click="copyCid(entry)">
                      <span aria-hidden="true">{{ t('filesCopyCid') }}</span>
                      <span class="files-menu-cid-value" aria-hidden="true">{{ entry.cid }}</span>
                    </button>
                    <button v-if="entry.cid" type="button" role="menuitem" @click="pinEntry(entry)">{{ t('filesPin') }}</button>
                    <button type="button" role="menuitem" @click="openDialog('rename', entry)">{{ t('filesRename') }}</button>
                    <button type="button" role="menuitem" @click="openDialog('move', entry)">{{ t('filesMove') }}</button>
                    <button type="button" role="menuitem" @click="openDialog('copy', entry)">{{ t('filesDuplicate') }}</button>
                    <button type="button" role="menuitem" class="is-danger" @click="openDialog('delete', entry)">{{ t('filesDelete') }}</button>
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </Card>

    <Dialog v-model:open="dialogOpen" :title="dialogTitle" :close-label="t('closeDialog')">
      <form v-if="dialog === 'mkdir'" class="files-form" @submit.prevent="submitMkdir">
        <Input v-model="draft" :label="t('filesNewFolderLabel')" name="folder-name" :spellcheck="false" />
        <div class="files-form-actions">
          <Button type="button" variant="quiet" data-autofocus @click="closeDialog">{{ t('filesCancel') }}</Button>
          <Button type="submit" variant="primary" :disabled="!mkdirValid">{{ t('filesCreate') }}</Button>
        </div>
      </form>
      <form v-else-if="dialog === 'rename'" class="files-form" @submit.prevent="submitRename">
        <Input v-model="draft" :label="t('filesRenameLabel')" name="new-name" :spellcheck="false" />
        <div class="files-form-actions">
          <Button type="button" variant="quiet" data-autofocus @click="closeDialog">{{ t('filesCancel') }}</Button>
          <Button type="submit" variant="primary" :disabled="!nameValid">{{ t('filesRename') }}</Button>
        </div>
      </form>
      <form v-else-if="dialog === 'move'" class="files-form" @submit.prevent="submitMove">
        <Input v-model="folderDraft" :label="t('filesMoveLabel')" :hint="t('filesMoveHint')" name="folder" :spellcheck="false" />
        <div class="files-form-actions">
          <Button type="button" variant="quiet" data-autofocus @click="closeDialog">{{ t('filesCancel') }}</Button>
          <Button type="submit" variant="primary" :disabled="!folderValid">{{ t('filesMoveAction') }}</Button>
        </div>
      </form>
      <form v-else-if="dialog === 'copy'" class="files-form" @submit.prevent="submitCopy">
        <Input v-model="draft" :label="t('filesDuplicateLabel')" name="copy-name" :spellcheck="false" />
        <div class="files-form-actions">
          <Button type="button" variant="quiet" data-autofocus @click="closeDialog">{{ t('filesCancel') }}</Button>
          <Button type="submit" variant="primary" :disabled="!nameValid">{{ t('filesDuplicateAction') }}</Button>
        </div>
      </form>
      <form v-else-if="dialog === 'delete'" class="files-form" @submit.prevent="submitDelete">
        <p>{{ target?.type === 'directory' ? t('filesDeleteFolder') : t('filesDeleteFile') }}</p>
        <div class="files-form-actions">
          <Button type="button" variant="quiet" data-autofocus @click="closeDialog">{{ t('filesCancel') }}</Button>
          <Button type="submit" variant="danger">{{ t('filesDelete') }}</Button>
        </div>
      </form>
      <form v-else-if="dialog === 'replace'" class="files-form" @submit.prevent="confirmReplace">
        <p>{{ t('filesReplaceBody', { name: replaceFile?.name ?? '' }) }}</p>
        <div class="files-form-actions">
          <Button type="button" variant="quiet" data-autofocus @click="closeDialog">{{ t('filesCancel') }}</Button>
          <Button type="submit" variant="danger">{{ t('filesReplace') }}</Button>
        </div>
      </form>
    </Dialog>
  </div>
</template>
