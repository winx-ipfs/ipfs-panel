<script setup lang="ts">
withDefaults(
  defineProps<{
    tone?: 'neutral' | 'info' | 'success' | 'warning' | 'danger'
    pulse?: boolean
  }>(),
  { tone: 'neutral', pulse: false },
)
</script>

<template>
  <span class="ui-pill" :class="[`is-${tone}`, { 'is-pulse': pulse }]">
    <span class="ui-pill-dot" aria-hidden="true" />
    <slot />
  </span>
</template>
