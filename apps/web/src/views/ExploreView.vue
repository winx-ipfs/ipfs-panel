<script setup lang="ts">
import { ChevronRight, Compass, Copy } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import EmptyState from '@/components/EmptyState.vue'
import IconButton from '@/components/IconButton.vue'
import Input from '@/components/Input.vue'
import PageHeader from '@/components/PageHeader.vue'
import Skeleton from '@/components/Skeleton.vue'
import { crumbs, exploreURL, nextTarget, type ExploreKind, type ExploreLink, type ExploreView } from '@/explore/model'
import { formatBytes } from '@/files/model'
import { shortenMiddle } from '@/lib/inline'
import { parseTarget } from '@/pins/cid'
import { useExploreStore } from '@/stores/explore'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useExploreStore()
const toast = useToastStore()
const draft = ref('')

const fieldError = computed(() => {
  if (!draft.value.trim()) return ''
  return parseTarget(draft.value) ? '' : t('exploreTargetError')
})
const canSubmit = computed(() => store.status !== 'loading')
const marker = computed(() => {
  if (store.status === 'blocked') return 'down'
  if (store.status === 'ready' && store.view) return 'result'
  if (store.status === 'loading') return 'loading'
  return 'start'
})
const problem = computed(() => {
  if (store.status === 'blocked' || store.status === 'unreachable' || store.status === 'loading') return ''
  return problemText(store.error)
})
const blockedTitle = computed(() => {
  if (store.nodeState === 'starting') return t('exploreNodeStartingTitle')
  if (store.nodeState === 'stopping') return t('exploreNodeStoppingTitle')
  if (store.nodeState === 'failed') return t('exploreNodeFailedTitle')
  return t('exploreNodeDownTitle')
})
const blockedBody = computed(() => {
  if (store.nodeState === 'starting') return t('exploreNodeStartingBody')
  if (store.nodeState === 'stopping') return t('exploreNodeStoppingBody')
  if (store.nodeState === 'failed') return t('exploreNodeFailedBody')
  return t('exploreNodeDownBody')
})

function problemText(code: string) {
  if (code === 'not_found') return t('exploreNotFound')
  if (code === 'timeout') return t('exploreErrorTimeout')
  if (code === 'too_large') return t('exploreErrorLarge')
  if (code === 'invalid_target') return t('exploreTargetError')
  if (!code) return ''
  return t('exploreErrorGeneric')
}

function kindLabel(kind: ExploreKind) {
  const labels: Record<ExploreKind, string> = {
    folder: t('exploreKindFolder'),
    file: t('exploreKindFile'),
    bytes: t('exploreKindBytes'),
    data: t('exploreKindData'),
    symlink: t('exploreKindSymlink'),
    block: t('exploreKindBlock'),
  }
  return labels[kind]
}

function kindBody(kind: ExploreKind) {
  const bodies: Record<ExploreKind, string> = {
    folder: t('exploreKindFolderBody'),
    file: t('exploreKindFileBody'),
    bytes: t('exploreKindBytesBody'),
    data: t('exploreKindDataBody'),
    symlink: t('exploreKindSymlinkBody'),
    block: t('exploreKindBlockBody'),
  }
  return bodies[kind]
}

function codecLine(view: ExploreView) {
  const hash = view.hash === 'sha2-256' ? t('exploreHashSha256') : view.hash === 'identity' ? t('exploreHashIdentity') : ''
  if (hash) return t('exploreCodecHash', { codec: view.codec, version: view.version, hash })
  return t('exploreCodecLine', { codec: view.codec, version: view.version })
}

function linkName(link: ExploreLink) {
  return link.name || t('exploreUnnamed')
}

function sizeText(size: number | null) {
  return size === null ? '—' : formatBytes(size)
}

function pretty(data: unknown) {
  if (data === undefined) return ''
  if (typeof data === 'string') return data
  try {
    return JSON.stringify(data, null, 2)
  } catch {
    return ''
  }
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

async function go(target: string) {
  if (store.preview) return
  draft.value = target
  if (route.query.target === target) {
    await store.lookup(target)
    return
  }
  await router.replace({ name: 'explore', query: { target } })
}

async function submit() {
  const parsed = parseTarget(draft.value)
  if (!parsed) return
  await go(parsed)
}

function openLink(link: ExploreLink) {
  if (!store.view) return
  void go(nextTarget(store.view.target, link))
}

function openCrumb(target: string, current: boolean) {
  if (current) return
  void go(target)
}

async function retry() {
  const target = typeof route.query.target === 'string' ? route.query.target : draft.value
  if (target) await store.lookup(target)
}

async function openRoute() {
  if (import.meta.env.DEV) {
    const { explorePreview } = await import('@/dev/mockExplore')
    const shot = explorePreview(window.location.search)
    if (shot) {
      draft.value = shot.draft
      if (shot.blocked) store.showBlocked(shot.blocked)
      else store.showPreview(shot.view)
      return
    }
  }
  const target = typeof route.query.target === 'string' ? route.query.target : ''
  draft.value = target
  if (!target) {
    store.idle()
    return
  }
  await store.lookup(target)
}

watch(() => route.fullPath, () => {
  void openRoute()
}, { immediate: true })
</script>

<template>
  <div class="explore-page" :data-explore="marker">
    <PageHeader :title="t('exploreTitle')" :description="t('exploreDescription')" />
    <details class="ui-disclosure">
      <summary>
        <ChevronRight class="ui-disclosure-chevron" :size="16" :stroke-width="1.75" aria-hidden="true" />
        <span>{{ t('exploreExplainSummary') }}</span>
      </summary>
      <p>{{ t('exploreExplainBody') }}</p>
    </details>

    <EmptyState
      v-if="store.status === 'unreachable'"
      :title="t('exploreUnreachableTitle')"
      :description="t('exploreUnreachableBody')"
    >
      <template #actions>
        <Button variant="primary" @click="retry">{{ t('exploreRetry') }}</Button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="store.status === 'blocked'"
      :title="blockedTitle"
      :description="blockedBody"
    >
      <template #icon>
        <Compass :size="22" :stroke-width="1.75" aria-hidden="true" />
      </template>
      <template #actions>
        <RouterLink class="ui-btn is-primary" to="/">{{ t('exploreGoNode') }}</RouterLink>
      </template>
    </EmptyState>

    <template v-else>
      <Card>
        <form class="explore-form" @submit.prevent="submit">
          <Input
            v-model="draft"
            :label="t('exploreTargetLabel')"
            :hint="fieldError ? '' : t('exploreTargetHint')"
            :error="fieldError"
            autocomplete="off"
            :spellcheck="false"
          />
          <Button variant="primary" type="submit" :disabled="!canSubmit">
            <Compass :size="18" :stroke-width="1.75" aria-hidden="true" />
            {{ t('exploreAction') }}
          </Button>
        </form>
        <p v-if="problem" class="explore-problem" role="status">{{ problem }}</p>
        <p v-else-if="store.status === 'idle'" class="explore-idle">{{ t('exploreIdle') }}</p>
      </Card>

      <div v-if="store.status === 'loading'" class="explore-loading" role="status">
        <span class="sr-only">{{ t('exploreLoading') }}</span>
        <Skeleton height="22px" width="42%" />
        <Skeleton />
        <Skeleton width="72%" />
      </div>

      <Card v-else-if="store.view" class="explore-result-card">
        <nav v-if="crumbs(store.view.target).length > 1" class="explore-crumbs" :aria-label="t('explorePathLabel')">
          <ol>
            <li v-for="(crumb, index) in crumbs(store.view.target)" :key="crumb.target">
              <span v-if="index > 0" class="explore-sep" aria-hidden="true">/</span>
              <span v-if="index === crumbs(store.view.target).length - 1" class="explore-crumb is-current" aria-current="page">
                {{ shortenMiddle(crumb.label, 10, 8) }}
              </span>
              <button
                v-else
                type="button"
                class="explore-crumb"
                @click="openCrumb(crumb.target, false)"
              >
                {{ shortenMiddle(crumb.label, 10, 8) }}
              </button>
            </li>
          </ol>
        </nav>

        <h2 class="explore-kind">{{ kindLabel(store.view.kind) }}</h2>
        <p class="explore-meta">{{ codecLine(store.view) }}</p>
        <p class="explore-lead">{{ kindBody(store.view.kind) }}</p>
        <p v-if="store.view.focus" class="explore-focus">{{ t('exploreFocus', { path: store.view.focus }) }}</p>

        <dl class="explore-facts">
          <div class="explore-fact">
            <dt>{{ t('exploreCid') }}</dt>
            <dd>
              <code>{{ store.view.cid }}</code>
              <IconButton :label="t('exploreCopyCid')" @click="copyText(store.view.cid, t('exploreCopiedCid'))">
                <Copy :size="18" :stroke-width="1.75" aria-hidden="true" />
              </IconButton>
            </dd>
          </div>
          <div v-if="store.view.size !== null" class="explore-fact">
            <dt>{{ t('exploreSize') }}</dt>
            <dd>{{ formatBytes(store.view.size) }}</dd>
          </div>
        </dl>

        <pre v-if="store.view.text" class="explore-well">{{ store.view.text }}</pre>
        <pre v-else-if="pretty(store.view.data)" class="explore-well">{{ pretty(store.view.data) }}</pre>
        <p v-if="store.view.dataOmitted" class="explore-note">{{ t('exploreDataOmitted') }}</p>

        <section class="explore-links" :aria-label="t('exploreLinks')">
          <h3>{{ t('exploreLinks') }}</h3>
          <p v-if="store.view.links.length === 0" class="explore-note">{{ t('exploreNoLinks') }}</p>
          <template v-else>
            <table class="explore-table">
              <caption class="sr-only">{{ t('exploreLinks') }}</caption>
              <thead>
                <tr>
                  <th scope="col">{{ t('exploreLinkName') }}</th>
                  <th scope="col">{{ t('exploreCid') }}</th>
                  <th scope="col">{{ t('exploreSize') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(link, index) in store.view.links" :key="`${index}-${link.cid}`">
                  <td>
                    <button type="button" class="explore-name" :aria-label="t('exploreOpenLink', { name: linkName(link) })" @click="openLink(link)">
                      {{ linkName(link) }}
                    </button>
                  </td>
                  <td class="explore-mono">{{ shortenMiddle(link.cid, 8, 6) }}</td>
                  <td>{{ sizeText(link.size) }}</td>
                </tr>
              </tbody>
            </table>
            <ul class="explore-cards">
              <li v-for="(link, index) in store.view.links" :key="`card-${index}-${link.cid}`">
                <button type="button" class="explore-card" :aria-label="t('exploreOpenLink', { name: linkName(link) })" @click="openLink(link)">
                  <span class="explore-card-name">{{ linkName(link) }}</span>
                  <span class="explore-card-meta">{{ shortenMiddle(link.cid, 8, 6) }} · {{ sizeText(link.size) }}</span>
                </button>
              </li>
            </ul>
            <p v-if="store.view.linksTruncated" class="explore-note">
              {{ t('exploreLinksMore', { count: store.view.links.length }) }}
            </p>
          </template>
        </section>

        <div v-if="exploreURL(store.view.localGateway, store.view.target, store.view.cid)" class="explore-actions">
          <Button
            variant="quiet"
            @click="copyText(exploreURL(store.view.localGateway, store.view.target, store.view.cid) || '', t('exploreLocalCopied'))"
          >
            {{ t('exploreLocalLink') }}
          </Button>
        </div>
      </Card>
    </template>
  </div>
</template>
