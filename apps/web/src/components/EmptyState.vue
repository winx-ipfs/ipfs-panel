<script setup lang="ts">
withDefaults(
  defineProps<{
    title: string
    description?: string
    center?: boolean
  }>(),
  { description: '', center: false },
)
</script>

<template>
  <div class="ui-empty" :class="{ 'is-center': center }">
    <div v-if="$slots.icon" class="ui-empty-icon">
      <slot name="icon" />
    </div>
    <h2>{{ title }}</h2>
    <p v-if="description">{{ description }}</p>
    <div v-if="$slots.actions" class="ui-empty-actions">
      <slot name="actions" />
    </div>
  </div>
</template>
