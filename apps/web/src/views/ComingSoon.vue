<script setup lang="ts">
import { LayoutGrid, Pin } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Button from '@/components/Button.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const kind = computed(() => (route.path.startsWith('/scenes') ? 'scenes' : 'pins'))

const title = computed(() => (kind.value === 'scenes' ? t('comingScenesTitle') : t('comingPinsTitle')))

const description = computed(() => (kind.value === 'scenes' ? t('comingScenesBody') : t('comingPinsBody')))
</script>

<template>
  <div class="page">
    <PageHeader :title="title" />
    <EmptyState :title="t('comingLead')" :description="description">
      <template #icon>
        <LayoutGrid v-if="kind === 'scenes'" :size="22" :stroke-width="1.75" aria-hidden="true" />
        <Pin v-else :size="22" :stroke-width="1.75" aria-hidden="true" />
      </template>
      <template #actions>
        <Button variant="primary" @click="router.push('/')">{{ t('comingBack') }}</Button>
      </template>
    </EmptyState>
  </div>
</template>
