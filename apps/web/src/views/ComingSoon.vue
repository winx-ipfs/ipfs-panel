<script setup lang="ts">
import { FolderOpen, LayoutGrid, Pin } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Button from '@/components/Button.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const kind = computed(() => {
  if (route.path.startsWith('/pins')) return 'pins'
  if (route.path.startsWith('/scenes')) return 'scenes'
  return 'files'
})

const title = computed(() => {
  if (kind.value === 'pins') return t('comingPinsTitle')
  if (kind.value === 'scenes') return t('comingScenesTitle')
  return t('comingFilesTitle')
})

const description = computed(() => {
  if (kind.value === 'pins') return t('comingPinsBody')
  if (kind.value === 'scenes') return t('comingScenesBody')
  return t('comingFilesBody')
})
</script>

<template>
  <div class="page">
    <PageHeader :title="title" />
    <EmptyState :title="t('comingLead')" :description="description">
      <template #icon>
        <Pin v-if="kind === 'pins'" :size="22" :stroke-width="1.75" aria-hidden="true" />
        <LayoutGrid v-else-if="kind === 'scenes'" :size="22" :stroke-width="1.75" aria-hidden="true" />
        <FolderOpen v-else :size="22" :stroke-width="1.75" aria-hidden="true" />
      </template>
      <template #actions>
        <Button variant="primary" @click="router.push('/')">{{ t('comingBack') }}</Button>
      </template>
    </EmptyState>
  </div>
</template>
