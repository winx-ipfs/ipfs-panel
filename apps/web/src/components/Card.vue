<script setup lang="ts">
withDefaults(
  defineProps<{
    tone?: 'none' | 'neutral' | 'info' | 'success' | 'warning' | 'danger'
  }>(),
  { tone: 'none' },
)
</script>

<template>
  <section class="ui-card" :data-tone="tone">
    <div v-if="tone !== 'none'" class="ui-card-accent" aria-hidden="true" />
    <slot />
  </section>
</template>
