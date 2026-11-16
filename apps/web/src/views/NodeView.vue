<script setup lang="ts">
import { Check, Copy } from '@lucide/vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import Button from '@/components/Button.vue'
import IconButton from '@/components/IconButton.vue'
import InlineText from '@/components/InlineText.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import StatusPill from '@/components/StatusPill.vue'
import Tooltip from '@/components/Tooltip.vue'
import { formatBytes } from '@/files/model'
import { shortenMiddle } from '@/lib/inline'
import { hintKey, sentenceKey, statusLabelKey, statusTone, visibleActions } from '@/node/present'
import type { PeerRow } from '@/peers/model'
import { useNodeStore } from '@/stores/node'
import { usePeersStore } from '@/stores/peers'
import { usePinsStore } from '@/stores/pins'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const node = useNodeStore()
const peersStore = usePeersStore()
const pinsStore = usePinsStore()
const toast = useToastStore()
const copied = ref('')
let companionTimer = 0
let companionPreview = false

function clearCompanions() {
  window.clearInterval(companionTimer)
  companionTimer = 0
}

async function loadCompanions() {
  if (companionPreview) return
  if (import.meta.env.DEV && typeof window !== 'undefined') {
    const search = window.location.search
    const { peersPreview } = await import('@/dev/mockPeers')
    const { pinsPreview } = await import('@/dev/mockPins')
    const peersShot = peersPreview(search)
    const pinsShot = pinsPreview(search)
    if (peersShot || pinsShot) {
      companionPreview = true
      if (peersShot && !peersShot.blocked) peersStore.showList(peersShot.peers)
      if (pinsShot && !pinsShot.blocked) pinsStore.showPreview(pinsShot.list, pinsShot.jobs, '')
      return
    }
  }
  await Promise.all([peersStore.load(false), pinsStore.load()])
}

onMounted(() => {
  void node.startPolling()
})

onUnmounted(() => {
  node.stopPolling()
  clearCompanions()
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

watch(connected, (up) => {
  clearCompanions()
  if (!up) return
  void loadCompanions()
  companionTimer = window.setInterval(() => {
    if (!companionPreview) void peersStore.load(true)
  }, 5000)
})
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
const gateway = computed(() => (connected.value ? pinsStore.localGateway : ''))
const peerRows = computed(() => peersStore.peers.slice(0, 8))
const pinRows = computed(() => pinsStore.pins.slice(0, 5))

function directionText(peer: PeerRow) {
  if (peer.direction === 'in') return t('peersDirectionIn')
  if (peer.direction === 'out') return t('peersDirectionOut')
  if (peer.direction === 'both') return t('peersDirectionBoth')
  return '—'
}

function latencyText(peer: PeerRow) {
  if (peer.latencyMs === null) return '—'
  if (peer.latencyMs <= 0) return t('peersLatencySub')
  if (peer.latencyMs >= 1000) return t('peersLatencySeconds', { n: (peer.latencyMs / 1000).toFixed(1) })
  return t('peersLatencyMs', { n: peer.latencyMs })
}

function pinSize(size: number | null) {
  return size === null ? '—' : formatBytes(size)
}
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
    <div class="work" :class="{ 'is-fit': !connected }" :aria-busy="busy">
      <div v-if="loading" class="node-loading">
        <p class="sr-only" role="status">{{ sentence }}</p>
        <Skeleton width="7rem" height="28px" />
        <Skeleton width="72%" height="22px" />
        <Skeleton width="48%" height="14px" />
      </div>
      <template v-else>
        <div class="node-band">
          <div class="node-lead">
            <StatusPill :tone="tone" :pulse="pulsing">{{ t(statusLabelKey(node.overview, node.unreachable)) }}</StatusPill>
            <p class="node-sentence" role="status"><InlineText :text="sentence" /></p>
            <p v-if="hint" class="node-hint">{{ hint }}</p>
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
        </div>
        <div class="work-scroll node-facts">
          <dl class="prop-list">
            <div v-if="node.overview?.repoPath">
              <dt>{{ t('nodeFolder') }}</dt>
              <dd>
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
              </dd>
            </div>
            <div v-if="detail" class="node-details">
              <dt>{{ t('nodeDetails') }}</dt>
              <dd><pre>{{ detail }}</pre></dd>
            </div>
          </dl>
          <section v-if="connected" class="prop-section" :aria-label="t('connectionTitle')">
            <h2 class="side-title">{{ t('connectionTitle') }}</h2>
            <dl class="prop-list">
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
                <dt :title="t('peerHint')">{{ t('nodePeerId') }}<span class="sr-only">. {{ t('peerHint') }}</span></dt>
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
                </dd>
              </div>
              <div v-if="peers !== null">
                <dt :title="t('nodePeersHint')">{{ t('nodePeers') }}<span class="sr-only">. {{ t('nodePeersHint') }}</span></dt>
                <dd>
                  <RouterLink class="peers-count-link" to="/peers">
                    <span class="mono">{{ peers }}</span>
                    <span class="sr-only">{{ t('nodePeersOpen') }}</span>
                  </RouterLink>
                </dd>
              </div>
              <div v-if="showTraffic">
                <dt>{{ t('nodeTraffic') }}</dt>
                <dd>{{ t('nodeTrafficIn') }} {{ formatBytes(rateIn ?? 0) }}/s · {{ t('nodeTrafficOut') }} {{ formatBytes(rateOut ?? 0) }}/s</dd>
              </div>
              <div v-if="gateway">
                <dt>{{ t('nodeGateway') }}</dt>
                <dd class="mono">{{ gateway }}</dd>
              </div>
            </dl>
          </section>
          <section v-else class="prop-section explain-card">
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
          </section>
        </div>
        <div v-if="connected" class="node-split">
          <section class="node-section" :aria-label="t('nodeSectionPins')">
            <div class="node-section-head">
              <h2>{{ t('nodeSectionPins') }}</h2>
              <RouterLink to="/pins">{{ t('nodeViewAll') }}</RouterLink>
            </div>
            <div class="node-scroll">
              <p v-if="pinsStore.status === 'loading'" class="node-section-note">{{ t('pinsLoading') }}</p>
              <p v-else-if="pinRows.length === 0" class="node-section-note">
                {{ pinsStore.status === 'ready' ? t('nodeSectionPinsEmpty') : t('pinsUnreachableBody') }}
              </p>
              <table v-else class="node-table">
                <caption class="sr-only">{{ t('nodeSectionPins') }}</caption>
                <thead>
                  <tr>
                    <th scope="col">{{ t('pinsName') }}</th>
                    <th scope="col">{{ t('pinsType') }}</th>
                    <th scope="col">{{ t('pinsSize') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="pin in pinRows" :key="pin.cid">
                    <td><span class="node-name" :title="pin.cid">{{ pin.name || shortenMiddle(pin.cid, 8, 6) }}</span></td>
                    <td>{{ pin.type === 'direct' ? t('pinsFilterDirect') : t('pinsFilterRecursive') }}</td>
                    <td class="mono">{{ pinSize(pin.size) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
          <section class="node-section node-peers" :aria-label="t('nodeSectionPeers')">
            <div class="node-section-head">
              <h2>{{ t('nodeSectionPeers') }}</h2>
              <RouterLink to="/peers">{{ t('nodeViewAll') }}</RouterLink>
            </div>
            <div class="node-scroll">
              <p v-if="peersStore.status === 'loading'" class="node-section-note">{{ t('peersLoading') }}</p>
              <p v-else-if="peerRows.length === 0" class="node-section-note">
                {{ peersStore.status === 'ready' ? t('nodeSectionPeersEmpty') : t('peersErrorGeneric') }}
              </p>
              <table v-else class="node-table">
                <caption class="sr-only">{{ t('nodeSectionPeers') }}</caption>
                <thead>
                  <tr>
                    <th scope="col">{{ t('peersColPeer') }}</th>
                    <th scope="col">{{ t('peersColDirection') }}</th>
                    <th scope="col">{{ t('peersColLatency') }}</th>
                    <th scope="col">{{ t('peersColSoftware') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="peer in peerRows" :key="peer.peerId">
                    <td><code :title="peer.peerId">{{ shortenMiddle(peer.peerId, 10, 8) }}</code></td>
                    <td>{{ directionText(peer) }}</td>
                    <td>{{ latencyText(peer) }}</td>
                    <td>{{ peer.agent || '—' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </template>
    </div>
  </div>
</template>
