<script setup lang="ts">
import { ref } from 'vue'

withDefaults(defineProps<{ text: string; align?: 'center' | 'end' }>(), { align: 'center' })

const open = ref(false)
</script>

<template>
  <span class="ui-tip" :class="{ 'is-end': align === 'end' }" @mouseenter="open = true" @mouseleave="open = false" @focusin="open = true" @focusout="open = false">
    <slot />
    <span v-if="open" class="ui-tip-bubble" role="tooltip">{{ text }}</span>
  </span>
</template>
