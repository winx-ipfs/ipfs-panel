<script setup lang="ts">
const model = defineModel<string>({ required: true })

withDefaults(
  defineProps<{
    label: string
    options: { value: string; label: string }[]
    hint?: string
  }>(),
  { hint: '' },
)

const controlId = `select-${Math.random().toString(36).slice(2, 9)}`
</script>

<template>
  <label class="ui-field" :for="controlId">
    <span class="ui-field-label">{{ label }}</span>
    <select
      :id="controlId"
      v-model="model"
      class="ui-select"
      :aria-describedby="hint ? `${controlId}-note` : undefined"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
    <p v-if="hint" :id="`${controlId}-note`" class="ui-field-hint">{{ hint }}</p>
  </label>
</template>
