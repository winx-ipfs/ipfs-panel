<script setup lang="ts">
import { ChevronRight, MoreHorizontal, Pin, RefreshCw } from '@lucide/vue'
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
import { formatBytes, localShareURL } from '@/files/model'
import { shortenMiddle } from '@/lib/inline'
import { parseTarget, validPinName } from '@/pins/cid'
import { filterPins, sortPins, type Pin as PinRow, type PinFilter, type PinJob } from '@/pins/model'
import { usePinsStore } from '@/stores/pins'
import { useToastStore } from '@/stores/toast'

const { t, locale } = useI18n()
const route = useRoute()
const store = usePinsStore()
const toast = useToastStore()

const filters = ['all', 'recursive', 'direct'] as const
const query = ref('')
const filter = ref<PinFilter>('all')
const dialog = ref<'add' | 'unpin' | null>(null)
const mode = ref<'cid' | 'file'>('cid')
const draft = ref('')
const nameDraft = ref('')
const kind = ref<'recursive' | 'direct'>('recursive')
const chosen = ref<File | null>(null)
const uploadPercent = ref(0)
const uploading = ref(false)
const target = ref<PinRow | null>(null)
const menuFor = ref('')
const menuWrap = ref<HTMLElement | null>(null)
const picker = ref<HTMLInputElement | null>(null)
let menuOpener: HTMLElement | null = null
let poll = 0

const shown = computed(() => filterPins(sortPins(store.pins, locale.value === 'zh' ? 'zh-Hans' : 'en'), query.value, filter.value))
const visibleJobs = computed(() => store.jobs.filter((job) => job.state !== 'succeeded'))
const dialogOpen = computed({
  get: () => dialog.value !== null,
  set: (value: boolean) => {
    if (!value) closeDialog()
  },
})
const targetError = computed(() => {
  if (mode.value !== 'cid' || !draft.value.trim()) return ''
  return parseTarget(draft.value) ? '' : t('pinsTargetError')
})
const nameError = computed(() => (nameDraft.value && !validPinName(nameDraft.value) ? t('pinsErrorName') : ''))
const canSubmit = computed(() => {
  if (uploading.value || nameError.value) return false
  if (mode.value === 'file') return Boolean(chosen.value)
  return Boolean(parseTarget(draft.value))
})
const marker = computed(() => {
  if (dialog.value === 'add' && targetError.value) return 'add'
  if (dialog.value === 'unpin') return 'unpin'
  if (store.jobs.some((job) => job.state === 'running')) return 'job'
  if (store.status === 'blocked') return 'down'
  if (store.status === 'ready' && store.pins.length === 0) return 'empty'
  if (store.status === 'ready') return 'list'
  return ''
})
const blockedTitle = computed(() => {
  if (store.nodeState === 'starting') return t('pinsNodeStartingTitle')
  if (store.nodeState === 'stopping') return t('pinsNodeStoppingTitle')
  if (store.nodeState === 'failed') return t('pinsNodeFailedTitle')
  return t('pinsNodeDownTitle')
})
const blockedBody = computed(() => {
  if (store.nodeState === 'starting') return t('pinsNodeStartingBody')
  if (store.nodeState === 'stopping') return t('pinsNodeStoppingBody')
  if (store.nodeState === 'failed') return t('pinsNodeFailedBody')
  return t('pinsNodeDownBody')
})
const dialogTitle = computed(() => {
  if (dialog.value === 'unpin') return t('pinsUnpinTitle')
  if (dialog.value === 'add') return t('pinsAddTitle')
  return ''
})

function typeLabel(type: string) {
  return type === 'direct' ? t('pinsTypeDirect') : t('pinsTypeRecursive')
}

function sizeText(pin: PinRow) {
  return pin.size === null ? '—' : formatBytes(pin.size)
}

function actionName(pin: PinRow) {
  return pin.name || shortenMiddle(pin.cid, 8, 6)
}

function rowKey(pin: PinRow) {
  return `${pin.type}:${pin.cid}`
}

function filterText(key: PinFilter) {
  if (key === 'recursive') return t('pinsFilterRecursive')
  if (key === 'direct') return t('pinsFilterDirect')
  return t('pinsFilterAll')
}

function problemText(code: string, name: string) {
  if (code === 'invalid_cid') return t('pinsTargetError')
  if (code === 'invalid_name') return t('pinsErrorName')
  if (code === 'not_found') return t('pinsErrorNotFound')
  if (code === 'indirect') return t('pinsErrorIndirect')
  if (code === 'too_many') return t('pinsErrorTooMany')
  if (code === 'too_large') return t('pinsErrorLarge', { name, size: formatBytes(store.maxBytes) })
  if (code === 'timeout') return t('pinsErrorTimeout')
  if (code === 'network') return t('pinsErrorNetwork')
  return t('pinsErrorGeneric')
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
  const wrap = menuOpener?.closest('.pins-menu-wrap')
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

function closeDialog() {
  dialog.value = null
  target.value = null
  uploading.value = false
}

function openAdd() {
  closeMenu(false)
  mode.value = 'cid'
  draft.value = ''
  nameDraft.value = ''
  kind.value = 'recursive'
  chosen.value = null
  uploadPercent.value = 0
  dialog.value = 'add'
}

function openUnpin(pin: PinRow) {
  closeMenu(false)
  target.value = pin
  dialog.value = 'unpin'
}

function onPick(event: Event) {
  const input = event.target
  if (!(input instanceof HTMLInputElement)) return
  chosen.value = input.files?.[0] ?? null
  input.value = ''
}

async function submitAdd() {
  if (!canSubmit.value) return
  if (mode.value === 'file' && chosen.value) {
    const file = chosen.value
    uploading.value = true
    const result = await store.upload(file, nameDraft.value, (percent) => {
      uploadPercent.value = percent
    })
    uploading.value = false
    if (!result.ok) {
      announce(result.error, file.name)
      return
    }
    toast.show(t('pinsUploadDone', { name: nameDraft.value || file.name }))
    closeDialog()
    return
  }
  const parsed = parseTarget(draft.value)
  if (!parsed) return
  const result = await store.addPin(parsed, nameDraft.value, kind.value === 'recursive')
  if (!result.ok) {
    announce(result.error, nameDraft.value)
    return
  }
  toast.show(t('pinsAdded'))
  closeDialog()
}

async function submitUnpin() {
  if (!target.value) return
  const pin = target.value
  const result = await store.unpin(pin.cid, pin.type)
  if (!result.ok) {
    announce(result.error, pin.name)
    return
  }
  closeDialog()
}

async function cancelJob(job: PinJob) {
  const result = await store.cancel(job.id)
  if (!result.ok) announce(result.error, job.name)
}

function copyCid(pin: PinRow) {
  closeMenu(false)
  void copyText(pin.cid, t('pinsCopiedCid'))
}

function copyLink(pin: PinRow) {
  const url = localShareURL(store.localGateway, pin.cid)
  if (!url) return
  closeMenu(false)
  void copyText(url, t('pinsLocalCopied'))
}

async function refresh() {
  const ok = await store.load()
  await store.refreshJobs()
  if (!ok && store.status === 'unreachable') toast.show(t('pinsErrorNetwork'))
}

function syncPoll() {
  window.clearInterval(poll)
  if (store.preview || !store.jobs.some((job) => job.state === 'running')) return
  poll = window.setInterval(() => {
    void store.refreshJobs()
  }, 1500)
}

async function openRoute() {
  if (import.meta.env.DEV) {
    const { pinsPreview } = await import('@/dev/mockPins')
    const shot = pinsPreview(window.location.search)
    if (shot) {
      store.showPreview(shot.list, shot.jobs, shot.blocked)
      if (shot.dialog === 'add') {
        mode.value = 'cid'
        draft.value = shot.draft
        nameDraft.value = ''
        dialog.value = 'add'
      }
      return
    }
  }
  dialog.value = null
  await store.load()
  await store.refreshJobs()
}

watch(() => store.jobs.map((job) => `${job.id}:${job.state}`).join(), syncPoll)
watch(() => route.fullPath, () => {
  menuFor.value = ''
  void openRoute()
}, { immediate: true })

onMounted(() => {
  document.addEventListener('pointerdown', onPointer)
  syncPoll()
})
onUnmounted(() => {
  document.removeEventListener('pointerdown', onPointer)
  window.clearInterval(poll)
})
</script>

<template>
  <div class="pins-page" :data-pins="marker">
    <PageHeader :title="t('pinsTitle')" :description="t('pinsDescription')" />
    <details class="ui-disclosure">
      <summary>
        <ChevronRight class="ui-disclosure-chevron" :size="16" :stroke-width="1.75" aria-hidden="true" />
        <span>{{ t('pinsExplainSummary') }}</span>
      </summary>
      <p>{{ t('pinsExplainBody') }}</p>
      <p v-if="store.status === 'ready'">{{ store.namesFrom === 'kubo' ? t('pinsNamesKubo') : t('pinsNamesPanel') }}</p>
    </details>

    <section v-if="visibleJobs.length" class="pins-jobs" :aria-label="t('pinsJobs')">
      <h2>{{ t('pinsJobs') }}</h2>
      <ul>
        <li v-for="job in visibleJobs" :key="job.id">
          <div class="pins-job-row">
            <span>{{ job.name || shortenMiddle(job.cid, 8, 6) }}</span>
            <Button v-if="job.state === 'running'" variant="quiet" size="sm" @click="cancelJob(job)">
              {{ t('pinsJobCancel') }}
            </Button>
          </div>
          <p v-if="job.state === 'running'">{{ t('pinsJobRunning') }}</p>
          <p v-else-if="job.state === 'cancelled'">{{ t('pinsJobCancelled') }}</p>
          <p v-else>{{ problemText(job.error, job.name) }}</p>
          <p v-if="job.state === 'running' && job.progress > 0" class="pins-job-count">
            {{ t('pinsJobBlocks', { count: job.progress }) }}
          </p>
        </li>
      </ul>
    </section>

    <EmptyState
      v-if="store.status === 'unreachable'"
      :title="t('pinsUnreachableTitle')"
      :description="t('pinsUnreachableBody')"
    >
      <template #actions>
        <Button variant="primary" @click="refresh">{{ t('pinsRetry') }}</Button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="store.status === 'blocked'"
      :title="blockedTitle"
      :description="blockedBody"
    >
      <template #icon>
        <Pin :size="22" :stroke-width="1.75" aria-hidden="true" />
      </template>
      <template #actions>
        <RouterLink class="ui-btn is-primary" to="/">{{ t('pinsGoNode') }}</RouterLink>
      </template>
    </EmptyState>

    <Card v-else>
      <div class="pins-toolbar">
        <div class="pins-toolbar-main">
          <Button variant="primary" @click="openAdd">
            <Pin :size="18" :stroke-width="1.75" aria-hidden="true" />
            {{ t('pinsAdd') }}
          </Button>
          <IconButton :label="t('pinsRefresh')" @click="refresh">
            <RefreshCw :size="18" :stroke-width="1.75" aria-hidden="true" />
          </IconButton>
        </div>
        <div class="pins-tools">
          <label class="sr-only" for="pin-search">{{ t('pinsSearch') }}</label>
          <input
            id="pin-search"
            v-model="query"
            class="ui-input"
            type="search"
            :placeholder="t('pinsSearchPlaceholder')"
          >
          <div class="pins-filter" role="group" :aria-label="t('pinsFilter')">
            <button
              v-for="key in filters"
              :key="key"
              type="button"
              class="pins-filter-btn"
              :class="{ 'is-active': filter === key }"
              :aria-pressed="filter === key"
              @click="filter = key"
            >
              {{ filterText(key) }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="store.status === 'loading'" class="pins-loading" role="status">
        <span class="sr-only">{{ t('pinsLoading') }}</span>
        <Skeleton v-for="row in 4" :key="row" height="42px" />
      </div>
      <EmptyState
        v-else-if="store.pins.length === 0"
        :title="t('pinsEmptyTitle')"
        :description="t('pinsEmptyBody')"
      />
      <EmptyState
        v-else-if="shown.length === 0"
        :title="t('pinsNoMatchTitle')"
        :description="t('pinsNoMatchBody')"
      />
      <table v-else class="pins-table">
        <caption class="sr-only">{{ t('pinsTitle') }}</caption>
        <thead>
          <tr>
            <th scope="col">{{ t('pinsName') }}</th>
            <th scope="col">{{ t('pinsCid') }}</th>
            <th scope="col">{{ t('pinsType') }}</th>
            <th scope="col">{{ t('pinsSize') }}</th>
            <th scope="col"><span class="sr-only">{{ t('pinsMenuLabel') }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="pin in shown" :key="rowKey(pin)">
            <th scope="row" class="pins-name-cell">
              <span class="pins-name" :class="{ 'is-unnamed': !pin.name }">{{ pin.name || t('pinsUnnamed') }}</span>
            </th>
            <td class="pins-cid">
              <code>{{ shortenMiddle(pin.cid, 8, 6) }}</code>
              <button type="button" class="pins-text-btn" :aria-label="t('pinsCopyCid')" @click="copyCid(pin)">
                {{ t('pinsCopy') }}
              </button>
            </td>
            <td class="pins-type">
              <span>{{ typeLabel(pin.type) }}</span>
              <code class="pins-type-code">{{ pin.type }}</code>
            </td>
            <td class="pins-size">{{ sizeText(pin) }}</td>
            <td class="pins-meta">
              <span class="sr-only">{{ t('pinsType') }} {{ typeLabel(pin.type) }}, {{ t('pinsSize') }} {{ sizeText(pin) }}</span>
              <span aria-hidden="true">{{ typeLabel(pin.type) }} · {{ sizeText(pin) }}</span>
            </td>
            <td class="pins-actions">
              <div class="pins-menu-wrap">
                <IconButton :label="t('pinsActions', { name: actionName(pin) })" @click="toggleMenu(rowKey(pin), $event)">
                  <MoreHorizontal :size="18" :stroke-width="1.75" aria-hidden="true" />
                </IconButton>
                <div
                  v-if="menuFor === rowKey(pin)"
                  class="pins-menu"
                  role="menu"
                  :aria-label="t('pinsMenuLabel')"
                  @keydown="onMenuKey"
                >
                  <button type="button" class="pins-menu-cid" role="menuitem" :aria-label="t('pinsCopyCid')" @click="copyCid(pin)">
                    <span aria-hidden="true">{{ t('pinsCopyCid') }}</span>
                    <span class="pins-menu-cid-value" aria-hidden="true">{{ pin.cid }}</span>
                  </button>
                  <button v-if="localShareURL(store.localGateway, pin.cid)" type="button" role="menuitem" @click="copyLink(pin)">
                    {{ t('pinsLocalLink') }}
                  </button>
                  <p class="pins-menu-code" aria-hidden="true">{{ pin.type }}</p>
                  <button type="button" role="menuitem" class="is-danger" @click="openUnpin(pin)">{{ t('pinsUnpin') }}</button>
                </div>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </Card>

    <Dialog
      v-model:open="dialogOpen"
      :title="dialogTitle"
      :close-label="t('closeDialog')"
      :scrollable="dialog === 'add'"
    >
      <form v-if="dialog === 'add'" class="pins-form" @submit.prevent="submitAdd">
        <p>{{ t('pinsAddLead') }}</p>
        <div class="pins-mode" role="group" :aria-label="t('pinsAddTitle')">
          <button type="button" class="pins-filter-btn" :class="{ 'is-active': mode === 'cid' }" :aria-pressed="mode === 'cid'" @click="mode = 'cid'">
            {{ t('pinsModeCid') }}
          </button>
          <button type="button" class="pins-filter-btn" :class="{ 'is-active': mode === 'file' }" :aria-pressed="mode === 'file'" @click="mode = 'file'">
            {{ t('pinsModeFile') }}
          </button>
        </div>
        <Input
          v-if="mode === 'cid'"
          v-model="draft"
          :label="t('pinsTargetLabel')"
          :hint="t('pinsTargetHint')"
          :error="targetError"
          name="pin-target"
          :spellcheck="false"
          autofocus
        />
        <div v-else class="pins-file">
          <Button type="button" variant="quiet" @click="picker?.click()">{{ t('pinsChooseFile') }}</Button>
          <p v-if="chosen">{{ t('pinsFileChosen', { name: chosen.name }) }}</p>
          <progress v-if="uploading" :value="uploadPercent" max="100" :aria-label="t('pinsUploading', { name: chosen?.name ?? '' })" />
          <input
            ref="picker"
            class="sr-only"
            tabindex="-1"
            aria-hidden="true"
            type="file"
            @change="onPick"
          >
        </div>
        <Input
          v-model="nameDraft"
          :label="t('pinsNameLabel')"
          :hint="t('pinsNameHint')"
          :error="nameError"
          name="pin-name"
          :spellcheck="false"
        />
        <details v-if="mode === 'cid'" class="ui-disclosure pins-more">
          <summary>
            <ChevronRight class="ui-disclosure-chevron" :size="16" :stroke-width="1.75" aria-hidden="true" />
            <span>{{ t('pinsMore') }}</span>
          </summary>
          <div class="pins-choice">
            <label>
              <input v-model="kind" type="radio" value="recursive">
              <span>
                {{ t('pinsKeepLinks') }}
                <small>{{ t('pinsKeepLinksHint') }}</small>
              </span>
            </label>
            <label>
              <input v-model="kind" type="radio" value="direct">
              <span>
                {{ t('pinsKeepOne') }}
                <small>{{ t('pinsKeepOneHint') }}</small>
              </span>
            </label>
          </div>
        </details>
        <div class="pins-form-actions">
          <Button type="button" variant="quiet" @click="closeDialog">{{ t('pinsCancel') }}</Button>
          <Button type="submit" variant="primary" :disabled="!canSubmit">
            {{ mode === 'file' ? t('pinsUploadAction') : t('pinsAddAction') }}
          </Button>
        </div>
      </form>
      <form v-else-if="dialog === 'unpin'" class="pins-form" @submit.prevent="submitUnpin">
        <p>{{ t('pinsUnpinBody') }}</p>
        <div class="pins-form-actions">
          <Button type="button" variant="quiet" data-autofocus @click="closeDialog">{{ t('pinsCancel') }}</Button>
          <Button type="submit" variant="danger">{{ t('pinsUnpinAction') }}</Button>
        </div>
      </form>
    </Dialog>
  </div>
</template>
