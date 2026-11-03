<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    disabled?: boolean
  }>(),
  { disabled: false },
)
</script>

<template>
  <button type="button" class="ui-icon-btn" :aria-label="label" :disabled="disabled">
    <slot />
  </button>
</template>
