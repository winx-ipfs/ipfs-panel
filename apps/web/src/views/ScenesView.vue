<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import Dialog from '@/components/Dialog.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import StatusPill from '@/components/StatusPill.vue'
import {
  changePhraseKey,
  resultKey,
  sceneErrorKey,
  sceneSummaryKey,
  sceneTitleKey,
  type SceneCatalog,
  type SceneItem,
} from '@/scenes/model'
import { useNodeStore } from '@/stores/node'
import { useScenesStore } from '@/stores/scenes'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const store = useScenesStore()
const node = useNodeStore()
const toast = useToastStore()

const catalog = ref<SceneCatalog | null>(null)
const openId = ref('')
const undoOpen = ref(false)
const result = ref<{ id: string; restart: string } | null>(null)
const dev = ref(false)

const selected = computed(() => catalog.value?.scenes.find((scene) => scene.id === openId.value) ?? null)
const phrases = computed(() => {
  const scene = selected.value
  if (!scene) return []
  return scene.changes
    .map((change) => {
      const key = changePhraseKey(change)
      return key ? { key: change.key, text: t(key) } : null
    })
    .filter((item): item is { key: string; text: string } => item !== null)
})
const busyNode = computed(() => {
  const state = catalog.value?.nodeState
  return state === 'starting' || state === 'stopping'
})

onMounted(() => {
  void node.startPolling()
  void openRoute()
})

onUnmounted(() => {
  node.stopPolling()
})

async function openRoute() {
  if (import.meta.env.DEV && new URLSearchParams(window.location.search).has('scenes')) {
    const { scenesPreview } = await import('@/dev/mockScenes')
    const shot = scenesPreview(window.location.search)
    if (shot) {
      dev.value = true
      catalog.value = shot.catalog
      openId.value = shot.openId
      result.value = shot.result ? { id: shot.result, restart: 'started' } : null
      return
    }
  }
  const ok = await store.load()
  catalog.value = store.catalog
  if (!ok) toast.show(t('scenesErrorNetwork'))
}

function preview(scene: SceneItem) {
  openId.value = scene.id
}

async function apply() {
  const scene = selected.value
  if (!scene || dev.value) return
  const outcome = await store.apply(scene.id)
  catalog.value = store.catalog
  if (!outcome.ok) {
    toast.show(t(sceneErrorKey(outcome.code)))
    return
  }
  result.value = { id: scene.id, restart: outcome.restart }
  openId.value = ''
}

async function revert() {
  if (dev.value) {
    undoOpen.value = false
    return
  }
  const outcome = await store.revert()
  catalog.value = store.catalog
  undoOpen.value = false
  if (!outcome.ok) {
    toast.show(t(sceneErrorKey(outcome.code)))
    return
  }
  result.value = { id: 'revert', restart: outcome.restart }
}

function shotName(): string {
  if (result.value) return 'applied'
  if (openId.value) return 'preview'
  return 'list'
}
</script>

<template>
  <div class="page page-scenes" :data-scenes="shotName()">
    <PageHeader :title="t('scenesTitle')" :description="t('scenesDescription')" />
    <p v-if="store.loading && !catalog" class="sr-only" role="status">{{ t('scenesLoading') }}</p>
    <div v-if="store.loading && !catalog" class="scene-list" aria-hidden="true">
      <Card><Skeleton width="40%" height="22px" /><Skeleton width="90%" height="16px" /></Card>
      <Card><Skeleton width="46%" height="22px" /><Skeleton width="88%" height="16px" /></Card>
    </div>
    <EmptyState v-else-if="catalog && !catalog.ready" :title="t('scenesNotReadyTitle')" :description="t('scenesNotReadyBody')">
      <template #actions>
        <RouterLink class="ui-btn is-primary" to="/">{{ t('scenesNotReadyAction') }}</RouterLink>
      </template>
    </EmptyState>
    <template v-else-if="catalog">
      <p v-if="result" class="scene-result" role="status">{{ t(resultKey(result.id, result.restart)) }}</p>
      <p v-else-if="!catalog.scenes.some((scene) => scene.active)" class="scene-note">
        {{ t('scenesNoneBody') }}
      </p>
      <div class="scene-list">
        <Card v-for="scene in catalog.scenes" :key="scene.id">
          <div class="scene-lead">
            <h2>{{ t(sceneTitleKey(scene.id)) }}</h2>
            <StatusPill v-if="scene.active" tone="success">{{ t('scenesInUse') }}</StatusPill>
          </div>
          <p class="scene-summary">{{ t(sceneSummaryKey(scene.id), { port: catalog.gatewayPort }) }}</p>
          <div class="scene-actions">
            <Button variant="quiet" :data-scene="scene.id" @click="preview(scene)">{{ t('scenesPreview') }}</Button>
          </div>
        </Card>
      </div>
      <div v-if="catalog.revert" class="scene-actions">
        <Button variant="quiet" @click="undoOpen = true">{{ t('scenesUndo') }}</Button>
      </div>
    </template>

    <Dialog v-model:open="undoOpen" :title="t('scenesUndoTitle')" :close-label="t('closeDialog')">
      <p>{{ t('scenesUndoBody') }}</p>
      <div class="scene-actions">
        <Button variant="quiet" data-autofocus @click="undoOpen = false">{{ t('scenesCancel') }}</Button>
        <Button variant="primary" :disabled="store.saving" @click="revert">{{ t('scenesUndoConfirm') }}</Button>
      </div>
    </Dialog>

    <Dialog
      :open="selected !== null"
      :title="selected ? t(sceneTitleKey(selected.id)) : t('scenesTitle')"
      :close-label="t('closeDialog')"
      scrollable
      @update:open="(value) => { if (!value) openId = '' }"
    >
      <template v-if="selected && catalog">
        <p class="scene-summary">{{ t(sceneSummaryKey(selected.id), { port: catalog.gatewayPort }) }}</p>
        <p v-if="selected.changes.length === 0" class="scene-note">{{ t('scenesResultNone') }}</p>
        <ul v-else class="scene-changes">
          <li v-for="item in phrases" :key="item.key">
            <span>{{ item.text }}</span>
            <span class="scene-key">{{ item.key }}</span>
          </li>
        </ul>
        <p v-if="selected.changes.length" class="scene-note">
          {{ busyNode ? t('scenesBusy') : catalog.nodeState === 'up' ? t('scenesRestartNow') : t('scenesRestartLater') }}
        </p>
        <div class="scene-actions">
          <Button variant="quiet" @click="openId = ''">{{ t('scenesCancel') }}</Button>
          <Button
            variant="primary"
            :disabled="store.saving || selected.changes.length === 0 || busyNode"
            data-scene-apply
            @click="apply"
          >
            {{ t('scenesApply') }}
          </Button>
        </div>
      </template>
    </Dialog>
  </div>
</template>
