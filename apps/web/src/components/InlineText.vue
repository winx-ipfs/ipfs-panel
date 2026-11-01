<script setup lang="ts">
import { computed } from 'vue'
import { inlineParts } from '@/lib/inline'

const props = defineProps<{ text: string }>()
const parts = computed(() => inlineParts(props.text))
</script>

<template>
  <template v-for="(part, index) in parts" :key="index">
    <code v-if="part.code" class="inline-code">{{ part.text }}</code>
    <template v-else>{{ part.text }}</template>
  </template>
</template>
