<script setup lang="ts">
import { cn } from 'cn'
import MotionIn from '@/components/MotionIn.vue'

withDefaults(
  defineProps<{
    caption?: string
    captionHidden?: boolean
    variant?: 'grid' | 'plain'
  }>(),
  { caption: '', variant: 'grid', captionHidden: false },
)
</script>

<template>
  <MotionIn>
    <div data-slot="table-container" class="ui-table-scroll relative w-full overflow-x-auto">
      <table data-slot="table" :class="cn('ui-table w-full text-sm', { 'is-plain': variant === 'plain' })">
        <caption v-if="caption" :class="{ 'sr-only': captionHidden }">{{ caption }}</caption>
        <slot />
      </table>
    </div>
  </MotionIn>
</template>
