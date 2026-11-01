<script setup lang="ts">
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
  <div class="ui-table-scroll">
    <table class="ui-table" :class="{ 'is-plain': variant === 'plain' }">
      <caption v-if="caption" :class="{ 'sr-only': captionHidden }">{{ caption }}</caption>
      <slot />
    </table>
  </div>
</template>
