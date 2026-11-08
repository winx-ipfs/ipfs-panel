<script setup lang="ts">
import { Eye, EyeOff } from '@lucide/vue'
import { computed, ref } from 'vue'

const model = defineModel<string>({ default: '' })
const emit = defineEmits<{ blur: [] }>()

const props = withDefaults(
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
    showPasswordLabel?: string
    hidePasswordLabel?: string
    autofocus?: boolean
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
    showPasswordLabel: 'Show password',
    hidePasswordLabel: 'Hide password',
    autofocus: false,
  },
)

const controlId = `input-${Math.random().toString(36).slice(2, 9)}`
const visible = ref(false)
const inputType = computed(() => (props.type === 'password' && visible.value ? 'text' : props.type))
const toggleLabel = computed(() => (visible.value ? props.hidePasswordLabel : props.showPasswordLabel))
</script>

<template>
  <div class="ui-field">
    <label class="ui-field-label" :for="controlId">{{ label }}</label>
    <div class="ui-input-wrap">
      <input
        :id="controlId"
        v-model="model"
        class="ui-input"
        :class="{ 'has-toggle': type === 'password' }"
        :type="inputType"
        :placeholder="placeholder"
        :autocomplete="autocomplete"
        :name="name"
        :spellcheck="spellcheck"
        :required="required || undefined"
        :aria-invalid="error ? 'true' : undefined"
        :aria-describedby="error || hint ? `${controlId}-note` : undefined"
        :data-autofocus="autofocus ? '' : undefined"
        @blur="emit('blur')"
      >
      <button
        v-if="type === 'password'"
        type="button"
        class="ui-password-toggle"
        data-password-toggle
        :aria-pressed="visible"
        :aria-label="toggleLabel"
        @click="visible = !visible"
      >
        <EyeOff v-if="visible" :size="18" :stroke-width="1.75" aria-hidden="true" />
        <Eye v-else :size="18" :stroke-width="1.75" aria-hidden="true" />
      </button>
    </div>
    <p v-if="error" :id="`${controlId}-note`" class="ui-field-error">{{ error }}</p>
    <p v-else-if="hint" :id="`${controlId}-note`" class="ui-field-hint">{{ hint }}</p>
  </div>
</template>
