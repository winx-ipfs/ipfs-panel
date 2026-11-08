<script setup lang="ts">
withDefaults(
  defineProps<{
    title: string
    description?: string
  }>(),
  { description: '' },
)
</script>

<template>
  <header class="ui-page-header">
    <h1>{{ title }}</h1>
    <p v-if="description">{{ description }}</p>
    <slot />
  </header>
</template>
