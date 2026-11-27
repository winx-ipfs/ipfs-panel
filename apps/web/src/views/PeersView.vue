<script setup lang="ts">
import { ChevronRight, Copy, Users } from '@lucide/vue'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import Button from '@/components/Button.vue'
import MotionIn from '@/components/MotionIn.vue'
import Card from '@/components/Card.vue'
import EmptyState from '@/components/EmptyState.vue'
import IconButton from '@/components/IconButton.vue'
import Input from '@/components/Input.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import { parseAddress } from '@/peers/address'
import { filterPeers, type PeerRow } from '@/peers/model'
import type { PermanentPeer } from '@/peering/model'
import { useNodeStore } from '@/stores/node'
import { usePeersStore } from '@/stores/peers'
import { usePeeringStore } from '@/stores/peering'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const route = useRoute()
const store = usePeersStore()
const peering = usePeeringStore()
const node = useNodeStore()
const toast = useToastStore()
const draft = ref('')
const query = ref('')
const busy = ref('')
const busyPeer = ref('')
const actionError = ref('')
let poll = 0

const fieldError = computed(() => {
  if (!draft.value.trim()) return ''
  return parseAddress(draft.value) ? '' : t('peersAddressError')
})
const canUseAddress = computed(() => Boolean(parseAddress(draft.value)) && busy.value === '')
const shown = computed(() => filterPeers(store.peers, query.value))
const filtering = computed(() => query.value.trim().length > 0)
const keptIds = computed(() => new Set(peering.peers.map((peer) => peer.peerId)))
const working = computed(() => store.status === 'ready' && peering.status !== 'blocked' && peering.status !== 'loading')
const marker = computed(() => {
  if (store.status === 'blocked' || peering.status === 'blocked') return 'down'
  if (store.status === 'loading' || (store.status === 'ready' && peering.status === 'loading')) return 'loading'
  if (store.status === 'failed') return 'failed'
  if (busy.value === 'keep') return 'adding'
  if (fieldError.value || actionError.value) return 'error'
  if (peering.sync === 'restart') return 'restart'
  if (peering.sync === 'apply') return 'apply'
  if (peering.peers.length > 0) return 'kept'
  if (store.status === 'ready' && store.peers.length === 0) return 'empty'
  if (store.status === 'ready') return 'list'
  return ''
})
const blockedTitle = computed(() => {
  const state = store.nodeState || peering.nodeState
  if (state === 'starting') return t('peersNodeStartingTitle')
  if (state === 'stopping') return t('peersNodeStoppingTitle')
  if (state === 'failed') return t('peersNodeFailedTitle')
  return t('peersNodeDownTitle')
})
const blockedBody = computed(() => {
  const state = store.nodeState || peering.nodeState
  if (state === 'starting') return t('peersNodeStartingBody')
  if (state === 'stopping') return t('peersNodeStoppingBody')
  if (state === 'failed') return t('peersNodeFailedBody')
  return t('peersNodeDownBody')
})
const listError = computed(() => {
  if (store.status === 'failed') return problemText(store.error || 'kubo_error')
  if (peering.status === 'failed') return problemText(peering.error || 'kubo_error')
  if (store.status === 'ready' && store.error) return problemText(store.error)
  if (peering.error) return problemText(peering.error)
  return ''
})

function problemText(code: string) {
  if (code === 'invalid_address') return t('peersAddressError')
  if (code === 'invalid_peer') return t('peersErrorPeer')
  if (code === 'not_connected') return t('peersErrorNotConnected')
  if (code === 'too_many') return t('peersErrorTooMany')
  if (code === 'connect_failed') return t('peersErrorConnect')
  if (code === 'timeout') return t('peersErrorTimeout')
  if (code === 'too_large') return t('peersErrorLarge')
  if (code === 'network') return t('peersUnreachableBody')
  if (!code || code === 'node_not_running' || code === 'unauthorized') return ''
  return t('peersErrorGeneric')
}

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

function softwareText(peer: PeerRow) {
  return peer.agent || '—'
}

function stateText(peer: PermanentPeer) {
  if (peer.state === 'connected') return t('peersStateConnected')
  if (peer.state === 'reconnecting') return t('peersStateReconnecting')
  return t('peersStatePending')
}

async function copyText(value: string) {
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
  toast.show(t('copied'))
}

function clearPoll() {
  window.clearInterval(poll)
  poll = 0
}

function armPoll() {
  clearPoll()
  if (store.preview || peering.preview || store.status !== 'ready') return
  poll = window.setInterval(() => {
    void store.load(true)
    void peering.load(true)
  }, 5000)
}

async function reload() {
  await Promise.all([store.load(true), peering.load(true)])
}

async function keepAddress() {
  const parsed = parseAddress(draft.value)
  if (!parsed || busy.value) return
  busy.value = 'keep'
  actionError.value = ''
  const code = await peering.addAddress(parsed.address)
  busy.value = ''
  if (code) {
    actionError.value = problemText(code)
    return
  }
  draft.value = ''
  toast.show(t('peersKeptToast'))
  await store.load(true)
}

async function connectOnce() {
  const parsed = parseAddress(draft.value)
  if (!parsed || busy.value) return
  busy.value = 'connect'
  actionError.value = ''
  const code = await store.connect(parsed.address)
  busy.value = ''
  if (code) {
    actionError.value = problemText(code)
    return
  }
  draft.value = ''
  toast.show(t('peersConnected'))
}

async function keepRow(peerId: string) {
  if (busy.value || keptIds.value.has(peerId)) return
  busy.value = 'keep-row'
  busyPeer.value = peerId
  actionError.value = ''
  const code = await peering.addPeer(peerId)
  busy.value = ''
  busyPeer.value = ''
  if (code) {
    actionError.value = problemText(code)
    return
  }
  toast.show(t('peersKeptToast'))
}

async function removePeer(peerId: string) {
  if (busy.value) return
  busy.value = 'remove'
  busyPeer.value = peerId
  actionError.value = ''
  const code = await peering.remove(peerId)
  busy.value = ''
  busyPeer.value = ''
  if (code) {
    actionError.value = problemText(code)
    return
  }
  toast.show(t('peersRemoved'))
}

async function applySaved() {
  if (busy.value) return
  busy.value = 'apply'
  actionError.value = ''
  const code = await peering.apply()
  busy.value = ''
  if (code) {
    actionError.value = problemText(code)
    return
  }
  if (peering.sync === 'restart') {
    actionError.value = t('peersRestartBody')
    return
  }
  toast.show(t('peersApplied'))
}

async function restartNode() {
  if (busy.value) return
  busy.value = 'restart'
  actionError.value = ''
  await node.restart()
  busy.value = ''
  if (node.unreachable) {
    actionError.value = t('peersUnreachableBody')
    return
  }
  await reload()
}

async function retry() {
  await Promise.all([store.load(false), peering.load(false)])
}

async function openRoute() {
  clearPoll()
  busy.value = ''
  busyPeer.value = ''
  actionError.value = ''
  if (import.meta.env.DEV) {
    const { peersPreview } = await import('@/dev/mockPeers')
    const shot = peersPreview(window.location.search)
    if (shot) {
      draft.value = shot.draft
      if (shot.blocked) {
        store.showBlocked(shot.blocked)
        peering.clear()
      } else {
        store.showList(shot.peers)
        peering.show(shot.permanent)
      }
      busy.value = shot.busy
      return
    }
  }
  await Promise.all([store.load(false), peering.load(false)])
  armPoll()
}

watch(() => store.status, () => {
  if (!store.preview) armPoll()
})

watch(() => route.fullPath, () => {
  void openRoute()
}, { immediate: true })

onUnmounted(() => {
  clearPoll()
})
</script>

<template>
  <div class="peers-page" :data-peers="marker">
    <PageHeader :title="t('peersTitle')" :description="t('peersDescription')" />
    <details class="ui-disclosure">
      <summary>
        <ChevronRight class="ui-disclosure-chevron" :size="16" :stroke-width="1.75" aria-hidden="true" />
        <span>{{ t('peersExplainSummary') }}</span>
      </summary>
      <p>{{ t('peersExplainBody') }}</p>
    </details>

    <EmptyState
      v-if="store.status === 'unreachable'"
      :title="t('peersUnreachableTitle')"
      :description="t('peersUnreachableBody')"
    >
      <template #actions>
        <Button variant="primary" @click="retry">{{ t('peersRetry') }}</Button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="store.status === 'blocked' || peering.status === 'blocked'"
      :title="blockedTitle"
      :description="blockedBody"
    >
      <template #icon>
        <Users :size="22" :stroke-width="1.75" aria-hidden="true" />
      </template>
      <template #actions>
        <RouterLink class="ui-btn is-primary" to="/">{{ t('peersGoNode') }}</RouterLink>
      </template>
    </EmptyState>

    <template v-else>
      <div v-if="store.status === 'loading' || peering.status === 'loading'" class="peers-loading" role="status">
        <span class="sr-only">{{ t('peersLoading') }}</span>
        <Skeleton height="18px" width="28%" />
        <Skeleton height="52px" />
        <Skeleton height="52px" />
        <Skeleton height="52px" />
      </div>

      <Card v-else-if="store.status === 'failed' && store.peers.length === 0">
        <p class="peers-problem">{{ listError }}</p>
        <Button variant="primary" @click="retry">{{ t('peersRetry') }}</Button>
      </Card>

      <Card v-else class="peers-board">
        <form class="peers-toolbar" @submit.prevent="keepAddress">
          <Input
            v-model="draft"
            class="peers-address"
            name="peer-address"
            autocomplete="off"
            :spellcheck="false"
            :label="t('peersAddressLabel')"
            :hint="fieldError ? '' : t('peersAddressHint')"
            :error="fieldError"
          />
          <Button class="peers-keep" type="submit" variant="primary" :disabled="!canUseAddress">
            {{ busy === 'keep' ? t('peersKeeping') : t('peersKeep') }}
          </Button>
          <Button class="peers-once" type="button" variant="quiet" :disabled="!canUseAddress" @click="connectOnce">
            {{ busy === 'connect' ? t('peersConnecting') : t('peersConnectOnce') }}
          </Button>
          <Input
            v-if="store.peers.length > 0"
            v-model="query"
            class="peers-filter"
            name="peer-filter"
            autocomplete="off"
            :spellcheck="false"
            :label="t('peersFilterLabel')"
            :hint="t('peersFilterHint')"
            @keydown.escape="query = ''"
          />
        </form>
        <p v-if="actionError" class="peers-problem" role="alert">{{ actionError }}</p>
        <p v-else-if="listError" class="peers-problem">{{ listError }}</p>

        <div v-if="working || peering.status === 'ready' || peering.status === 'failed'" class="work-scroll">
          <section class="peers-kept" :data-peering-sync="peering.sync">
            <h2 class="peers-section">{{ t('peersKeptHeading') }}</h2>
            <div v-if="peering.sync === 'apply'" class="peers-banner" role="status">
              <p>{{ t('peersApplyBody') }}</p>
              <p>{{ t('peersApplyHint') }}</p>
              <div class="peers-banner-actions">
                <Button variant="primary" type="button" :disabled="busy !== ''" @click="applySaved">
                  {{ busy === 'apply' ? t('peersApplying') : t('peersApply') }}
                </Button>
                <Button variant="quiet" type="button" :disabled="busy !== ''" @click="restartNode">
                  {{ busy === 'restart' ? t('peersRestarting') : t('peersRestart') }}
                </Button>
              </div>
            </div>
            <div v-else-if="peering.sync === 'restart'" class="peers-banner" role="status">
              <p>{{ t('peersRestartBody') }}</p>
              <div class="peers-banner-actions">
                <Button variant="primary" type="button" :disabled="busy !== ''" @click="restartNode">
                  {{ busy === 'restart' ? t('peersRestarting') : t('peersRestart') }}
                </Button>
                <Button variant="quiet" type="button" :disabled="busy !== ''" @click="applySaved">
                  {{ busy === 'apply' ? t('peersApplying') : t('peersTryAgain') }}
                </Button>
              </div>
            </div>
            <p v-if="peering.peers.length === 0" class="peers-empty">{{ t('peersKeptEmpty') }}</p>
            <template v-else>
              <p class="peers-summary">{{ t('peersKeptCount', { count: peering.peers.length }) }}</p>
              <table class="peers-table peers-kept-table">
                <caption class="sr-only">{{ t('peersKeptHeading') }}</caption>
                <thead>
                  <tr>
                    <th scope="col">{{ t('peersColPeer') }}</th>
                    <th scope="col">{{ t('peersColAddress') }}</th>
                    <th scope="col">{{ t('peersColState') }}</th>
                    <th scope="col"><span class="sr-only">{{ t('peersRemove') }}</span></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="peer in peering.peers" :key="peer.peerId">
                    <td>
                      <div class="peers-id">
                        <code :title="peer.peerId">{{ peer.peerId }}</code>
                        <IconButton :label="t('peersCopyId')" @click="copyText(peer.peerId)">
                          <Copy :size="16" :stroke-width="1.75" aria-hidden="true" />
                        </IconButton>
                      </div>
                    </td>
                    <td class="peers-addrs">
                      <span v-if="peer.addrs.length === 0">{{ t('peersNoAddress') }}</span>
                      <code v-for="addr in peer.addrs" :key="addr" :title="addr">{{ addr }}</code>
                    </td>
                    <td>{{ stateText(peer) }}</td>
                    <td>
                      <Button class="peers-row-action" size="sm" type="button" variant="ghost" :disabled="busy !== ''" @click="removePeer(peer.peerId)">
                        {{ busy === 'remove' && busyPeer === peer.peerId ? t('peersRemoving') : t('peersRemove') }}
                      </Button>
                    </td>
                  </tr>
                </tbody>
              </table>
              <ul class="peers-cards peers-kept-cards">
                <li v-for="peer in peering.peers" :key="`kept-${peer.peerId}`">
                  <div class="peers-id">
                    <code>{{ peer.peerId }}</code>
                    <IconButton :label="t('peersCopyId')" @click="copyText(peer.peerId)">
                      <Copy :size="16" :stroke-width="1.75" aria-hidden="true" />
                    </IconButton>
                  </div>
                  <dl>
                    <div>
                      <dt>{{ t('peersColAddress') }}</dt>
                      <dd>
                        <span v-if="peer.addrs.length === 0">{{ t('peersNoAddress') }}</span>
                        <code v-for="addr in peer.addrs" :key="addr" :title="addr">{{ addr }}</code>
                      </dd>
                    </div>
                    <div>
                      <dt>{{ t('peersColState') }}</dt>
                      <dd>{{ stateText(peer) }}</dd>
                    </div>
                  </dl>
                  <Button class="peers-row-action" size="sm" type="button" variant="ghost" :disabled="busy !== ''" @click="removePeer(peer.peerId)">
                    {{ busy === 'remove' && busyPeer === peer.peerId ? t('peersRemoving') : t('peersRemove') }}
                  </Button>
                </li>
              </ul>
            </template>
          </section>

          <section class="peers-live">
            <h2 class="peers-section">{{ t('peersLiveHeading') }}</h2>
            <p v-if="store.peers.length === 0" class="peers-empty">{{ t('peersEmpty') }}</p>
            <template v-else>
              <p v-if="filtering && shown.length > 0" class="peers-summary">{{ t('peersSummaryMatch', { count: shown.length }) }}</p>
              <p v-else-if="!filtering" class="peers-summary">{{ t('peersSummary', { count: store.total }) }}</p>
              <p v-if="store.truncated && !filtering" class="peers-note">{{ t('peersTruncated', { shown: store.peers.length, total: store.total }) }}</p>
              <p v-if="shown.length === 0" class="peers-note">{{ t('peersNoMatch') }}</p>
              <MotionIn v-else>
                <table class="peers-table">
                  <caption class="sr-only">{{ t('peersLiveHeading') }}</caption>
                  <thead>
                    <tr>
                      <th scope="col">{{ t('peersColPeer') }}</th>
                      <th scope="col">{{ t('peersColDirection') }}</th>
                      <th scope="col">{{ t('peersColLatency') }}</th>
                      <th scope="col">{{ t('peersColSoftware') }}</th>
                      <th scope="col"><span class="sr-only">{{ t('peersKeep') }}</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="peer in shown" :key="peer.peerId">
                      <td>
                        <div class="peers-id">
                          <code :title="peer.peerId">{{ peer.peerId }}</code>
                          <IconButton :label="t('peersCopyId')" @click="copyText(peer.peerId)">
                            <Copy :size="16" :stroke-width="1.75" aria-hidden="true" />
                          </IconButton>
                        </div>
                      </td>
                      <td>{{ directionText(peer) }}</td>
                      <td>{{ latencyText(peer) }}</td>
                      <td>{{ softwareText(peer) }}</td>
                      <td>
                        <span v-if="keptIds.has(peer.peerId)" class="peers-kept-mark">{{ t('peersKeptRow') }}</span>
                        <Button v-else class="peers-row-action" size="sm" type="button" variant="ghost" :disabled="busy !== ''" @click="keepRow(peer.peerId)">
                          {{ busy === 'keep-row' && busyPeer === peer.peerId ? t('peersKeeping') : t('peersKeepRow') }}
                        </Button>
                      </td>
                    </tr>
                  </tbody>
                </table>
                <ul class="peers-cards">
                  <li v-for="peer in shown" :key="`card-${peer.peerId}`">
                    <div class="peers-id">
                      <code>{{ peer.peerId }}</code>
                      <IconButton :label="t('peersCopyId')" @click="copyText(peer.peerId)">
                        <Copy :size="16" :stroke-width="1.75" aria-hidden="true" />
                      </IconButton>
                    </div>
                    <dl>
                      <div>
                        <dt>{{ t('peersColDirection') }}</dt>
                        <dd>{{ directionText(peer) }}</dd>
                      </div>
                      <div>
                        <dt>{{ t('peersColLatency') }}</dt>
                        <dd>{{ latencyText(peer) }}</dd>
                      </div>
                      <div>
                        <dt>{{ t('peersColSoftware') }}</dt>
                        <dd>{{ softwareText(peer) }}</dd>
                      </div>
                    </dl>
                    <span v-if="keptIds.has(peer.peerId)" class="peers-kept-mark">{{ t('peersKeptRow') }}</span>
                    <Button v-else class="peers-row-action" size="sm" type="button" variant="ghost" :disabled="busy !== ''" @click="keepRow(peer.peerId)">
                      {{ busy === 'keep-row' && busyPeer === peer.peerId ? t('peersKeeping') : t('peersKeepRow') }}
                    </Button>
                  </li>
                </ul>
              </MotionIn>
            </template>
          </section>
        </div>
      </Card>
    </template>
  </div>
</template>
