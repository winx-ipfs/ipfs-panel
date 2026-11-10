<script setup lang="ts">
import { Check, Copy } from '@lucide/vue'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import IconButton from '@/components/IconButton.vue'
import InlineText from '@/components/InlineText.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import StatusPill from '@/components/StatusPill.vue'
import Table from '@/components/Table.vue'
import Tooltip from '@/components/Tooltip.vue'
import { formatBytes } from '@/files/model'
import { shortenMiddle } from '@/lib/inline'
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
const missingKubo = computed(
  () => !node.unreachable && node.overview?.state === 'failed' && node.overview.error?.code === 'binary_not_found',
)
const connected = computed(() => node.overview?.state === 'up')
const peerId = computed(() => (connected.value ? node.overview?.peerId ?? '' : ''))
const peerShort = computed(() => shortenMiddle(peerId.value))
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
const peers = computed(() => (connected.value ? node.overview?.activity?.peers ?? null : null))
const rateIn = computed(() => (connected.value ? node.overview?.activity?.rateIn ?? null : null))
const rateOut = computed(() => (connected.value ? node.overview?.activity?.rateOut ?? null : null))
const showTraffic = computed(() => rateIn.value !== null && rateOut.value !== null)
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
  <div class="page page-node">
    <PageHeader :title="t('nodePageTitle')" :description="t('nodePageDescription')" />
    <div class="node-layout">
      <Card :aria-busy="busy">
        <div v-if="loading" class="node-loading">
          <p class="sr-only" role="status">{{ sentence }}</p>
          <Skeleton width="7rem" height="28px" />
          <Skeleton width="72%" height="22px" />
          <Skeleton width="48%" height="14px" />
        </div>
        <template v-else>
          <div class="node-lead">
            <StatusPill :tone="tone" :pulse="pulsing">{{ t(statusLabelKey(node.overview, node.unreachable)) }}</StatusPill>
            <p class="node-sentence" role="status"><InlineText :text="sentence" /></p>
            <p v-if="hint" class="node-hint">{{ hint }}</p>
          </div>
          <Table
            v-if="node.overview?.repoPath"
            class="node-facts"
            variant="plain"
            :caption="t('nodeFactsCaption')"
            caption-hidden
          >
            <tbody>
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
              @click="missingKubo ? node.start() : node.retry()"
            >
              {{ t('nodeRetry') }}
            </Button>
            <a
              v-if="missingKubo"
              class="ui-btn quiet is-quiet"
              href="https://docs.ipfs.tech/install/command-line/"
              target="_blank"
              rel="noopener noreferrer"
              :aria-label="t('nodeInstallKuboLabel')"
            >
              {{ t('nodeInstallKubo') }}
            </a>
          </div>
        </template>
      </Card>
      <Card v-if="connected" class="connection-card">
        <h2 class="side-title">{{ t('connectionTitle') }}</h2>
        <dl class="connection-facts">
          <div v-if="showVersion">
            <dt>{{ t('nodeVersion') }}</dt>
            <dd>
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
            </dd>
          </div>
          <div v-if="showPeer">
            <dt>{{ t('nodePeerId') }}</dt>
            <dd>
              <div class="fact">
                <span v-if="peerShort === peerId" class="peer mono">{{ peerId }}</span>
                <template v-else>
                  <span class="mono" aria-hidden="true">{{ peerShort }}</span>
                  <span class="peer sr-only">{{ peerId }}</span>
                </template>
                <Tooltip :text="copied === 'peer' ? t('copied') : t('copyPeerId')" align="end">
                  <IconButton
                    data-copy="peer"
                    :label="t('copyPeerId')"
                    @click="copy('peer', peerId)"
                  >
                    <Check v-if="copied === 'peer'" :size="16" :stroke-width="1.75" aria-hidden="true" />
                    <Copy v-else :size="16" :stroke-width="1.75" aria-hidden="true" />
                  </IconButton>
                </Tooltip>
              </div>
              <p class="peer-hint">{{ t('peerHint') }}</p>
            </dd>
          </div>
          <div v-if="peers !== null">
            <dt>{{ t('nodePeers') }}</dt>
            <dd>
              <RouterLink class="peers-count-link" to="/peers">
                <span class="mono">{{ peers }}</span>
                <span class="sr-only">{{ t('nodePeersOpen') }}</span>
              </RouterLink>
              <p class="peer-hint">{{ t('nodePeersHint') }}</p>
            </dd>
          </div>
          <div v-if="showTraffic">
            <dt>{{ t('nodeTraffic') }}</dt>
            <dd>{{ t('nodeTrafficIn') }} {{ formatBytes(rateIn ?? 0) }}/s · {{ t('nodeTrafficOut') }} {{ formatBytes(rateOut ?? 0) }}/s</dd>
          </div>
        </dl>
      </Card>
      <Card v-else class="explain-card">
        <h2 class="side-title">{{ t('nodeExplainTitle') }}</h2>
        <p class="explain-body">{{ t('nodeExplainBody') }}</p>
        <a
          class="explain-link"
          href="https://docs.ipfs.tech/concepts/what-is-ipfs/"
          target="_blank"
          rel="noopener noreferrer"
          :aria-label="t('nodeExplainLinkLabel')"
        >
          {{ t('nodeExplainLink') }}
        </a>
      </Card>
    </div>
  </div>
</template>
