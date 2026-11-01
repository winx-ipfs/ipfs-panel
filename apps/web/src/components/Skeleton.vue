<script setup lang="ts">
withDefaults(defineProps<{ width?: string; height?: string }>(), {
  width: '100%',
  height: '14px',
})
</script>

<template>
  <span class="ui-skeleton" aria-hidden="true" :style="{ width, height }" />
</template>
