<script setup lang="ts">
import { Check, Copy } from '@lucide/vue'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import IconButton from '@/components/IconButton.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import StatusPill from '@/components/StatusPill.vue'
import Table from '@/components/Table.vue'
import Tooltip from '@/components/Tooltip.vue'
import { hintKey, sentenceKey, statusLabelKey, statusTone, visibleActions } from '@/node/present'
import { useNodeStore } from '@/stores/node'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const node = useNodeStore()
const toast = useToastStore()
const copied = ref('')

onMounted(() => {
  void node.startPolling()
})

onUnmounted(() => {
  node.stopPolling()
})

const key = computed(() => sentenceKey(node.overview, node.unreachable))
const sentence = computed(() => (key.value ? t(key.value) : t('nodeLoading')))
const hint = computed(() => {
  const next = hintKey(node.overview, node.unreachable)
  return next ? t(next) : ''
})
const tone = computed(() => statusTone(node.overview, node.unreachable))
const actions = computed(() => visibleActions(node.overview, node.unreachable))
const loading = computed(() => node.overview === null && !node.unreachable)
const busy = computed(
  () =>
    node.acting ||
    node.overview?.state === 'starting' ||
    node.overview?.state === 'stopping' ||
    loading.value,
)
const detail = computed(() => node.overview?.error?.detail ?? '')
const folderParts = computed(() => (node.overview?.repoPath ?? '').split('/'))
const showVersion = computed(() => node.overview?.state === 'up' && !!node.overview.version)
const showPeer = computed(() => node.overview?.state === 'up' && !!node.overview.peerId)
const pulsing = computed(
  () => !node.unreachable && (node.overview?.state === 'starting' || node.overview?.state === 'stopping'),
)

async function copy(kind: string, value: string | null | undefined) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    const area = document.createElement('textarea')
    area.value = value
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.insetInlineStart = '-9999px'
    document.body.appendChild(area)
    area.select()
    document.execCommand('copy')
    area.remove()
  }
  copied.value = kind
  toast.show(t('copied'))
  window.setTimeout(() => {
    if (copied.value === kind) copied.value = ''
  }, 1600)
}
</script>

<template>
  <div class="page">
    <PageHeader :title="t('nodePageTitle')" :description="t('nodePageDescription')" />
    <Card :tone="loading ? 'none' : tone" :aria-busy="busy">
      <div v-if="loading" class="node-loading">
        <p class="sr-only" role="status">{{ sentence }}</p>
        <Skeleton width="7rem" height="28px" />
        <Skeleton width="72%" height="22px" />
        <Skeleton width="48%" height="14px" />
      </div>
      <template v-else>
        <div class="node-lead">
          <StatusPill :tone="tone" :pulse="pulsing">{{ t(statusLabelKey(node.overview, node.unreachable)) }}</StatusPill>
          <p class="node-sentence" role="status">{{ sentence }}</p>
          <p v-if="hint" class="node-hint">{{ hint }}</p>
        </div>
        <Table
          v-if="showVersion || showPeer || node.overview?.repoPath"
          class="node-facts"
          variant="plain"
          :caption="t('nodeFactsCaption')"
          caption-hidden
        >
          <tbody>
            <tr v-if="showVersion">
              <th scope="row">{{ t('nodeVersion') }}</th>
              <td>
                <div class="fact">
                  <span class="mono">{{ node.overview?.version }}</span>
                  <Tooltip :text="copied === 'version' ? t('copied') : t('copyVersion')" align="end">
                    <IconButton
                      data-copy="version"
                      :label="t('copyVersion')"
                      @click="copy('version', node.overview?.version)"
                    >
                      <Check v-if="copied === 'version'" :size="16" :stroke-width="1.75" aria-hidden="true" />
                      <Copy v-else :size="16" :stroke-width="1.75" aria-hidden="true" />
                    </IconButton>
                  </Tooltip>
                </div>
              </td>
            </tr>
            <tr v-if="showPeer">
              <th scope="row">{{ t('nodePeerId') }}</th>
              <td>
                <div class="fact">
                  <span class="peer mono">{{ node.overview?.peerId }}</span>
                  <Tooltip :text="copied === 'peer' ? t('copied') : t('copyPeerId')" align="end">
                    <IconButton
                      data-copy="peer"
                      :label="t('copyPeerId')"
                      @click="copy('peer', node.overview?.peerId)"
                    >
                      <Check v-if="copied === 'peer'" :size="16" :stroke-width="1.75" aria-hidden="true" />
                      <Copy v-else :size="16" :stroke-width="1.75" aria-hidden="true" />
                    </IconButton>
                  </Tooltip>
                </div>
                <p class="peer-hint">{{ t('peerHint') }}</p>
              </td>
            </tr>
            <tr v-if="node.overview?.repoPath">
              <th scope="row">{{ t('nodeFolder') }}</th>
              <td>
                <div class="fact">
                  <span class="mono path">
                    <template v-for="(part, index) in folderParts" :key="`${index}-${part}`">
                      <template v-if="index > 0">/<wbr></template>{{ part }}
                    </template>
                  </span>
                  <Tooltip :text="copied === 'folder' ? t('copied') : t('copyFolder')" align="end">
                    <IconButton
                      data-copy="folder"
                      :label="t('copyFolder')"
                      @click="copy('folder', node.overview?.repoPath)"
                    >
                      <Check v-if="copied === 'folder'" :size="16" :stroke-width="1.75" aria-hidden="true" />
                      <Copy v-else :size="16" :stroke-width="1.75" aria-hidden="true" />
                    </IconButton>
                  </Tooltip>
                </div>
              </td>
            </tr>
          </tbody>
        </Table>
        <div v-if="detail" class="node-details">
          <div class="label">{{ t('nodeDetails') }}</div>
          <pre>{{ detail }}</pre>
        </div>
        <div v-if="actions.length" class="node-actions">
          <Button
            v-if="actions.includes('start')"
            variant="primary"
            data-node-action="start"
            :disabled="node.acting"
            @click="node.start()"
          >
            {{ t('nodeStart') }}
          </Button>
          <Button
            v-if="actions.includes('stop')"
            variant="primary"
            data-node-action="stop"
            :disabled="node.acting"
            @click="node.stop()"
          >
            {{ t('nodeStop') }}
          </Button>
          <Button
            v-if="actions.includes('restart')"
            variant="quiet"
            data-node-action="restart"
            :disabled="node.acting"
            @click="node.restart()"
          >
            {{ t('nodeRestart') }}
          </Button>
          <Button
            v-if="actions.includes('retry')"
            variant="primary"
            data-node-action="retry"
            :disabled="node.acting"
            @click="node.retry()"
          >
            {{ t('nodeRetry') }}
          </Button>
        </div>
      </template>
    </Card>
  </div>
</template>
