<script setup lang="ts">
import { ScrollText } from '@lucide/vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import Dialog from '@/components/Dialog.vue'
import EmptyState from '@/components/EmptyState.vue'
import Input from '@/components/Input.vue'
import PageHeader from '@/components/PageHeader.vue'
import Select from '@/components/Select.vue'
import Skeleton from '@/components/Skeleton.vue'
import { clock, type LogLevel, type LogLine } from '@/logs/model'
import { useLogsStore } from '@/stores/logs'
import { useToastStore } from '@/stores/toast'

const ROW = 32
const { t } = useI18n()
const store = useLogsStore()
const toast = useToastStore()
const draft = ref('')
const scroller = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewport = ref(0)
const levelsOpen = ref(false)
const subsystem = ref('all')
const writeLevel = ref('info')
const levelError = ref('')
const levelDetail = ref('')
const saving = ref(false)
let queryTimer = 0

const sourceOptions = computed(() => [
  { value: 'all', label: t('logsSourceAll') },
  { value: 'manager', label: t('logsSourceManager') },
  { value: 'kubo', label: t('logsSourceKubo') },
])
const levelOptions = computed(() => [
  { value: 'debug', label: t('logsLevelDebug') },
  { value: 'info', label: t('logsLevelInfo') },
  { value: 'warn', label: t('logsLevelWarn') },
  { value: 'error', label: t('logsLevelError') },
])
const writeOptions = computed(() => [
  { value: 'debug', label: t('logsTagDebug') },
  { value: 'info', label: t('logsTagInfo') },
  { value: 'warn', label: t('logsTagWarn') },
  { value: 'error', label: t('logsTagError') },
  { value: 'default', label: t('logsLevelDefault') },
])
const filtering = computed(
  () => store.source !== 'all' || store.level !== 'info' || store.q.trim().length > 0,
)
const marker = computed(() => {
  if (store.status === 'loading') return 'loading'
  if (store.status === 'failed') return 'error'
  if (store.lines.length === 0) {
    if (filtering.value) return 'filtered'
    if (store.node !== 'up') return 'stopped'
    if (!store.capture && store.node === 'up') return 'empty'
    return 'empty'
  }
  if (!store.follow) return 'paused'
  if (filtering.value) return 'filtered'
  return 'streaming'
})
const note = computed(() => {
  if (store.status !== 'ready' || store.lines.length === 0) return ''
  if (store.node !== 'up') return t('logsKept')
  if (!store.capture) return t('logsAdopted')
  return ''
})
const emptyTitle = computed(() => {
  if (filtering.value) return t('logsNoMatchTitle')
  if (store.node === 'starting') return t('logsStartingTitle')
  if (store.node === 'stopping') return t('logsStoppingTitle')
  if (store.node === 'failed') return t('logsFailedTitle')
  if (store.node !== 'up') return t('logsStoppedTitle')
  if (!store.capture) return t('logsAdopted')
  return t('logsEmptyTitle')
})
const emptyBody = computed(() => {
  if (filtering.value) return t('logsNoMatchBody')
  if (store.node === 'starting') return t('logsStartingBody')
  if (store.node === 'stopping') return t('logsStoppingBody')
  if (store.node === 'failed') return t('logsFailedBody')
  if (store.node !== 'up') return t('logsStoppedBody')
  if (!store.capture) return ''
  return t('logsEmptyBody')
})
const problem = computed(() => problemText(store.error))
const start = computed(() => {
  if (viewport.value <= 0) return 0
  return Math.max(0, Math.floor(scrollTop.value / ROW) - 8)
})
const end = computed(() => {
  if (viewport.value <= 0) return store.lines.length
  return Math.min(store.lines.length, Math.ceil((scrollTop.value + viewport.value) / ROW) + 8)
})
const visible = computed(() => store.lines.slice(start.value, end.value))
const padTop = computed(() => start.value * ROW)
const padBottom = computed(() => Math.max(0, (store.lines.length - end.value) * ROW))

function problemText(code: string) {
  if (code === 'timeout') return t('logsErrorTimeout')
  if (code === 'kubo_rejected') return t('logsErrorRejected')
  if (code === 'bad_level') return t('logsErrorLevel')
  if (code === 'bad_subsystem') return t('logsErrorSubsystem')
  if (code === 'network') return t('logsErrorNetwork')
  if (!code || code === 'unauthorized') return ''
  return t('logsErrorGeneric')
}

function levelLabel(level: LogLevel) {
  if (level === 'debug') return t('logsTagDebug')
  if (level === 'warn') return t('logsTagWarn')
  if (level === 'error') return t('logsTagError')
  return t('logsTagInfo')
}

function sourceLabel(source: LogLine['source']) {
  return source === 'kubo' ? t('logsSourceKubo') : t('logsSourceManager')
}

function onSource(event: Event) {
  const target = event.target
  if (target instanceof HTMLSelectElement) void store.setSource(target.value)
}

function onLevel(event: Event) {
  const target = event.target
  if (target instanceof HTMLSelectElement) void store.setLevel(target.value)
}

function onScroll(event: Event) {
  const el = event.target
  if (el instanceof HTMLElement) scrollTop.value = el.scrollTop
}

async function stick() {
  if (!store.follow) return
  await nextTick()
  const el = scroller.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}

async function copyLines() {
  const text = store.lines.map((line) => `${line.time}\t${line.level}\t${line.source}\t${line.text}`).join('\n')
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const area = document.createElement('textarea')
    area.value = text
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.left = '-9999px'
    document.body.appendChild(area)
    area.select()
    document.execCommand('copy')
    area.remove()
  }
  toast.show(t('copied'))
}

function toggleFollow() {
  store.setFollow(!store.follow)
  if (store.follow) void stick()
}

function clearFilter() {
  draft.value = ''
  store.resetFilters()
}

async function applyLevel() {
  saving.value = true
  levelError.value = ''
  levelDetail.value = ''
  const result = await store.applyLevel(subsystem.value.trim(), writeLevel.value)
  saving.value = false
  if (result.error) {
    levelError.value = problemText(result.error)
    levelDetail.value = result.detail
    return
  }
  levelsOpen.value = false
  toast.show(t('logsLevelSet'))
}

watch(draft, (value) => {
  window.clearTimeout(queryTimer)
  queryTimer = window.setTimeout(() => {
    if (value !== store.q) void store.setQuery(value)
  }, 200)
})

watch(
  () => store.lines.length,
  () => {
    void stick()
  },
)

watch(scroller, (el, _, onCleanup) => {
  if (!el || typeof ResizeObserver === 'undefined') return
  const observer = new ResizeObserver(() => {
    viewport.value = el.clientHeight
    void stick()
  })
  observer.observe(el)
  viewport.value = el.clientHeight
  onCleanup(() => observer.disconnect())
})

onMounted(async () => {
  if (import.meta.env.DEV) {
    const { logsPreview } = await import('@/dev/mockLogs')
    const shot = logsPreview(window.location.search)
    if (shot) {
      store.showPreview(shot)
      draft.value = shot.q
      return
    }
  }
  void store.load()
})

onUnmounted(() => {
  window.clearTimeout(queryTimer)
  store.stop()
})
</script>

<template>
  <div class="logs-page" :data-logs="marker">
    <PageHeader :title="t('logsTitle')" :description="t('logsDescription')" />
    <Card class="logs-board">
      <div class="logs-toolbar">
        <label class="logs-field">
          <span class="sr-only">{{ t('logsSource') }}</span>
          <select
            class="logs-select"
            :value="store.source"
            :disabled="store.status === 'loading'"
            :aria-label="t('logsSource')"
            @change="onSource"
          >
            <option v-for="option in sourceOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <label class="logs-field">
          <span class="sr-only">{{ t('logsLevel') }}</span>
          <select
            class="logs-select"
            :value="store.level"
            :disabled="store.status === 'loading'"
            :aria-label="t('logsLevel')"
            @change="onLevel"
          >
            <option v-for="option in levelOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <input
          v-model="draft"
          class="logs-search"
          type="search"
          :placeholder="t('logsSearch')"
          :aria-label="t('logsSearchLabel')"
          :disabled="store.status === 'loading'"
          @keydown.escape="clearFilter"
        >
        <Button
          :pressed="store.follow"
          :disabled="store.status === 'loading'"
          @click="toggleFollow"
        >
          {{ store.follow ? t('logsPause') : t('logsFollow') }}
        </Button>
        <Button :disabled="store.status !== 'ready' || store.lines.length === 0" @click="copyLines">
          {{ t('logsCopy') }}
        </Button>
        <Button :disabled="store.status === 'loading'" @click="store.download">
          {{ t('logsDownload') }}
        </Button>
        <Button :disabled="store.lines.length === 0" :aria-label="t('logsClearLabel')" @click="store.clearView">
          {{ t('logsClear') }}
        </Button>
        <Button :disabled="store.node !== 'up' || store.status === 'loading'" @click="levelsOpen = true">
          <span class="logs-expert">{{ t('configExpert') }}</span>
          {{ t('logsLevels') }}
        </Button>
        <span class="logs-meta">
          <span class="logs-count">{{ t('logsCount', { n: store.lines.length }) }}</span>
          <span class="logs-live" :data-state="store.follow ? 'follow' : 'pause'">
            {{ store.follow ? t('logsFollowing') : t('logsPaused') }}
          </span>
        </span>
      </div>

      <p v-if="note" class="logs-note">{{ note }}</p>

      <div v-if="store.status === 'loading'" class="logs-loading" role="status">
        <span class="sr-only">{{ t('logsLoading') }}</span>
        <Skeleton v-for="row in 12" :key="row" height="28px" />
      </div>

      <div v-else-if="store.status === 'failed'" class="logs-fill">
        <EmptyState :title="problem || t('logsErrorTitle')">
          <template #icon>
            <ScrollText :size="22" :stroke-width="1.75" aria-hidden="true" />
          </template>
          <template #actions>
            <Button variant="primary" @click="store.load">{{ t('logsRetry') }}</Button>
          </template>
        </EmptyState>
        <p v-if="store.detail" class="logs-detail">{{ store.detail }}</p>
      </div>

      <div v-else-if="store.lines.length === 0" class="logs-fill">
        <EmptyState :title="emptyTitle" :description="emptyBody">
          <template #icon>
            <ScrollText :size="22" :stroke-width="1.75" aria-hidden="true" />
          </template>
          <template v-if="filtering" #actions>
            <Button variant="primary" @click="clearFilter">{{ t('logsClearFilter') }}</Button>
          </template>
          <template v-else-if="store.node !== 'up'" #actions>
            <RouterLink class="ui-btn is-primary" to="/">{{ t('logsGoNode') }}</RouterLink>
          </template>
        </EmptyState>
      </div>

      <div v-else ref="scroller" class="logs-scroll" role="log" aria-live="off" :aria-label="t('logsTitle')" @scroll="onScroll">
        <div class="logs-canvas" :style="{ paddingTop: `${padTop}px`, paddingBottom: `${padBottom}px` }">
          <div v-for="line in visible" :key="line.id" class="logs-row">
            <time class="logs-time" :datetime="line.time" :title="line.time">{{ clock(line.time) }}</time>
            <span class="logs-level" :data-level="line.level">{{ levelLabel(line.level) }}</span>
            <span class="logs-source">{{ sourceLabel(line.source) }}</span>
            <span class="logs-text">{{ line.text }}</span>
          </div>
        </div>
      </div>
    </Card>

    <Dialog v-model:open="levelsOpen" :title="t('logsLevelTitle')" :close-label="t('closeDialog')" scrollable>
      <p class="logs-level-lead">
        <span class="logs-expert">{{ t('configExpert') }}</span>
        {{ t('logsLevelBody') }}
      </p>
      <Input
        v-model="subsystem"
        name="subsystem"
        autocomplete="off"
        :spellcheck="false"
        :label="t('logsSubsystem')"
        :hint="t('logsSubsystemHint')"
      />
      <Select v-model="writeLevel" :label="t('logsDialogLevel')" :options="writeOptions" />
      <p v-if="levelError" class="logs-level-error" role="alert">
        {{ levelError }}
        <span v-if="levelDetail" class="logs-detail">{{ levelDetail }}</span>
      </p>
      <div class="logs-level-actions">
        <Button variant="quiet" @click="levelsOpen = false">{{ t('closeDialog') }}</Button>
        <Button variant="primary" :disabled="saving || !subsystem.trim()" @click="applyLevel">
          {{ t('logsApply') }}
        </Button>
      </div>
    </Dialog>
  </div>
</template>
