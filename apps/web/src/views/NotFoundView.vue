<script setup lang="ts">
import { SearchX } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Button from '@/components/Button.vue'
import EmptyState from '@/components/EmptyState.vue'

const { t } = useI18n()
const router = useRouter()
</script>

<template>
  <EmptyState center :title="t('notFoundTitle')" :description="t('notFoundBody')">
    <template #icon>
      <SearchX :size="22" :stroke-width="1.75" aria-hidden="true" />
    </template>
    <template #actions>
      <Button variant="primary" @click="router.push('/')">{{ t('notFoundAction') }}</Button>
    </template>
  </EmptyState>
</template>
