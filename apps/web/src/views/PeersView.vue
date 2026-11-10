<script setup lang="ts">
import { ChevronRight, Copy, Users } from '@lucide/vue'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import EmptyState from '@/components/EmptyState.vue'
import IconButton from '@/components/IconButton.vue'
import Input from '@/components/Input.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import { parseAddress } from '@/peers/address'
import { filterPeers, type PeerRow } from '@/peers/model'
import { usePeersStore } from '@/stores/peers'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const route = useRoute()
const store = usePeersStore()
const toast = useToastStore()
const draft = ref('')
const query = ref('')
const connecting = ref(false)
const connectError = ref('')
let poll = 0

const fieldError = computed(() => {
  if (!draft.value.trim()) return ''
  return parseAddress(draft.value) ? '' : t('peersAddressError')
})
const canConnect = computed(() => Boolean(parseAddress(draft.value)) && !connecting.value)
const shown = computed(() => filterPeers(store.peers, query.value))
const filtering = computed(() => query.value.trim().length > 0)
const marker = computed(() => {
  if (store.status === 'blocked') return 'down'
  if (store.status === 'loading') return 'loading'
  if (store.status === 'ready' && store.peers.length === 0) return 'empty'
  if (store.status === 'ready') return 'list'
  if (store.status === 'failed') return 'failed'
  return ''
})
const blockedTitle = computed(() => {
  if (store.nodeState === 'starting') return t('peersNodeStartingTitle')
  if (store.nodeState === 'stopping') return t('peersNodeStoppingTitle')
  if (store.nodeState === 'failed') return t('peersNodeFailedTitle')
  return t('peersNodeDownTitle')
})
const blockedBody = computed(() => {
  if (store.nodeState === 'starting') return t('peersNodeStartingBody')
  if (store.nodeState === 'stopping') return t('peersNodeStoppingBody')
  if (store.nodeState === 'failed') return t('peersNodeFailedBody')
  return t('peersNodeDownBody')
})
const listError = computed(() => {
  if (store.status === 'failed') return problemText(store.error || 'kubo_error')
  if (store.status === 'ready' && store.error) return problemText(store.error)
  return ''
})

function problemText(code: string) {
  if (code === 'invalid_address') return t('peersAddressError')
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
  if (store.preview || store.status !== 'ready') return
  poll = window.setInterval(() => {
    void store.load(true)
  }, 5000)
}

async function submit() {
  const parsed = parseAddress(draft.value)
  if (!parsed || connecting.value) return
  connecting.value = true
  connectError.value = ''
  const code = await store.connect(parsed.address)
  connecting.value = false
  if (code) {
    connectError.value = problemText(code)
    return
  }
  draft.value = ''
  toast.show(t('peersConnected'))
}

async function retry() {
  await store.load(false)
}

async function openRoute() {
  clearPoll()
  if (import.meta.env.DEV) {
    const { peersPreview } = await import('@/dev/mockPeers')
    const shot = peersPreview(window.location.search)
    if (shot) {
      if (shot.blocked) store.showBlocked(shot.blocked)
      else store.showList(shot.peers)
      return
    }
  }
  await store.load(false)
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
      v-else-if="store.status === 'blocked'"
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
      <Card v-if="store.status !== 'loading'">
        <form class="peers-form" @submit.prevent="submit">
          <Input
            v-model="draft"
            name="peer-address"
            autocomplete="off"
            :spellcheck="false"
            :label="t('peersAddressLabel')"
            :hint="t('peersAddressHint')"
            :error="fieldError"
          />
          <Button type="submit" variant="primary" :disabled="!canConnect">
            {{ connecting ? t('peersConnecting') : t('peersConnect') }}
          </Button>
          <p v-if="connectError" class="peers-problem" role="alert">{{ connectError }}</p>
          <p v-else-if="store.status === 'ready' && store.peers.length === 0" class="peers-empty">{{ t('peersEmpty') }}</p>
        </form>
      </Card>

      <div v-if="store.status === 'loading'" class="peers-loading" role="status">
        <span class="sr-only">{{ t('peersLoading') }}</span>
        <Skeleton height="18px" width="28%" />
        <Skeleton height="52px" />
        <Skeleton height="52px" />
        <Skeleton height="52px" />
      </div>

      <Card v-else-if="store.status === 'failed'">
        <p class="peers-problem">{{ listError }}</p>
        <Button variant="primary" @click="retry">{{ t('peersRetry') }}</Button>
      </Card>

      <Card v-else-if="store.peers.length > 0">
        <p v-if="filtering && shown.length > 0" class="peers-summary">{{ t('peersSummaryMatch', { count: shown.length }) }}</p>
        <p v-else-if="!filtering" class="peers-summary">{{ t('peersSummary', { count: store.total }) }}</p>
        <p v-if="store.truncated && !filtering" class="peers-note">{{ t('peersTruncated', { shown: store.peers.length, total: store.total }) }}</p>
        <p v-if="listError" class="peers-problem">{{ listError }}</p>

        <div class="peers-filter">
          <Input
            v-model="query"
            name="peer-filter"
            autocomplete="off"
            :spellcheck="false"
            :label="t('peersFilterLabel')"
            :hint="t('peersFilterHint')"
            @keydown.escape="query = ''"
          />
        </div>

        <p v-if="shown.length === 0" class="peers-note">{{ t('peersNoMatch') }}</p>

        <template v-else>
          <table class="peers-table">
            <caption class="sr-only">{{ t('peersTitle') }}</caption>
            <thead>
              <tr>
                <th scope="col">{{ t('peersColPeer') }}</th>
                <th scope="col">{{ t('peersColDirection') }}</th>
                <th scope="col">{{ t('peersColLatency') }}</th>
                <th scope="col">{{ t('peersColSoftware') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="peer in shown" :key="peer.peerId">
                <td>
                  <div class="peers-id">
                    <code>{{ peer.peerId }}</code>
                    <IconButton :label="t('peersCopyId')" @click="copyText(peer.peerId)">
                      <Copy :size="16" :stroke-width="1.75" aria-hidden="true" />
                    </IconButton>
                  </div>
                </td>
                <td>{{ directionText(peer) }}</td>
                <td>{{ latencyText(peer) }}</td>
                <td>{{ softwareText(peer) }}</td>
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
            </li>
          </ul>
        </template>
      </Card>
    </template>
  </div>
</template>
