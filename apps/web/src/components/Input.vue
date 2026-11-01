<script setup lang="ts">
const model = defineModel<string>({ default: '' })
const emit = defineEmits<{ blur: [] }>()

withDefaults(
  defineProps<{
    label: string
    hint?: string
    error?: string
    type?: string
    placeholder?: string
    autocomplete?: string
    name?: string
    spellcheck?: boolean
    required?: boolean
  }>(),
  {
    type: 'text',
    hint: '',
    error: '',
    placeholder: '',
    autocomplete: undefined,
    name: undefined,
    spellcheck: undefined,
    required: false,
  },
)

const controlId = `input-${Math.random().toString(36).slice(2, 9)}`
</script>

<template>
  <label class="ui-field" :for="controlId">
    <span class="ui-field-label">{{ label }}</span>
    <input
      :id="controlId"
      v-model="model"
      class="ui-input"
      :type="type"
      :placeholder="placeholder"
      :autocomplete="autocomplete"
      :name="name"
      :spellcheck="spellcheck"
      :required="required || undefined"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="error || hint ? `${controlId}-note` : undefined"
      @blur="emit('blur')"
    >
    <p v-if="error" :id="`${controlId}-note`" class="ui-field-error">{{ error }}</p>
    <p v-else-if="hint" :id="`${controlId}-note`" class="ui-field-hint">{{ hint }}</p>
  </label>
</template>
