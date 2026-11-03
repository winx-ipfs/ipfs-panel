<script setup lang="ts">
const model = defineModel<boolean>({ required: true })

withDefaults(
  defineProps<{
    label: string
    hint?: string
  }>(),
  { hint: '' },
)
</script>

<template>
  <button
    type="button"
    class="ui-switch"
    role="switch"
    :aria-checked="model"
    @click="model = !model"
  >
    <span class="ui-switch-track" aria-hidden="true">
      <span class="ui-switch-thumb" />
    </span>
    <span class="ui-switch-copy">
      <span>{{ label }}</span>
      <small v-if="hint">{{ hint }}</small>
    </span>
  </button>
</template>
