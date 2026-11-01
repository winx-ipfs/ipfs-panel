<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'primary' | 'quiet' | 'danger' | 'ghost'
    size?: 'md' | 'sm'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }>(),
  {
    variant: 'quiet',
    size: 'md',
    type: 'button',
    disabled: false,
  },
)
</script>

<template>
  <button
    :type="type"
    class="ui-btn"
    :class="[variant, `is-${variant}`, `is-${size}`]"
    :disabled="disabled"
  >
    <slot />
  </button>
</template>
