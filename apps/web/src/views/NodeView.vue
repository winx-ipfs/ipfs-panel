<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { sentenceKey, visibleActions } from '@/node/present'
import { useNodeStore } from '@/stores/node'

const { t } = useI18n()
const node = useNodeStore()

onMounted(() => {
  void node.startPolling()
})

onUnmounted(() => {
  node.stopPolling()
})

const key = computed(() => sentenceKey(node.overview, node.unreachable))
const sentence = computed(() => (key.value ? t(key.value) : ''))
const actions = computed(() => visibleActions(node.overview, node.unreachable))
const busy = computed(
  () =>
    node.acting ||
    node.overview?.state === 'starting' ||
    node.overview?.state === 'stopping' ||
    (node.overview === null && !node.unreachable),
)
const detail = computed(() => node.overview?.error?.detail ?? '')
</script>

<template>
  <section class="node-card" :aria-busy="busy">
    <p class="sentence" role="status">{{ sentence }}</p>
    <dl v-if="node.overview?.state === 'up'" class="facts">
      <div>
        <dt>{{ t('nodeVersion') }}</dt>
        <dd>{{ node.overview?.version }}</dd>
      </div>
      <div>
        <dt>{{ t('nodePeerId') }}</dt>
        <dd class="peer">{{ node.overview?.peerId }}</dd>
      </div>
    </dl>
    <p v-if="node.overview?.repoPath" class="folder">
      <span class="label">{{ t('nodeFolder') }}</span>
      <span class="path">{{ node.overview?.repoPath }}</span>
    </p>
    <div v-if="detail" class="details">
      <div class="label">{{ t('nodeDetails') }}</div>
      <pre>{{ detail }}</pre>
    </div>
    <div v-if="actions.length" class="actions">
      <button
        v-if="actions.includes('start')"
        type="button"
        class="primary"
        :disabled="node.acting"
        @click="node.start()"
      >
        {{ t('nodeStart') }}
      </button>
      <button
        v-if="actions.includes('stop')"
        type="button"
        class="primary"
        :disabled="node.acting"
        @click="node.stop()"
      >
        {{ t('nodeStop') }}
      </button>
      <button
        v-if="actions.includes('restart')"
        type="button"
        class="quiet"
        :disabled="node.acting"
        @click="node.restart()"
      >
        {{ t('nodeRestart') }}
      </button>
      <button
        v-if="actions.includes('retry')"
        type="button"
        class="primary"
        :disabled="node.acting"
        @click="node.retry()"
      >
        {{ t('nodeRetry') }}
      </button>
    </div>
  </section>
</template>
