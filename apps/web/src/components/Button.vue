<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'
import { cn } from 'cn'
import { buttonVariants } from '@/components/ui/button'
import { useMotion } from '@/lib/motion'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'quiet' | 'danger' | 'ghost'
    size?: 'md' | 'sm'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    pressed?: boolean
  }>(),
  {
    variant: 'quiet',
    size: 'md',
    type: 'button',
    disabled: false,
  },
)

const { press, transition } = useMotion()
const shadVariant = computed(() => {
  if (props.variant === 'primary') return 'default' as const
  if (props.variant === 'danger') return 'destructive' as const
  if (props.variant === 'ghost') return 'ghost' as const
  return 'outline' as const
})
</script>

<template>
  <motion.button
    :type="type"
    :disabled="disabled"
    :aria-pressed="typeof pressed === 'boolean' ? pressed : undefined"
    :while-press="press"
    :transition="transition"
    :class="
      cn(
        buttonVariants({ variant: shadVariant, size: size === 'sm' ? 'sm' : 'lg' }),
        'ui-btn',
        variant,
        `is-${variant}`,
        `is-${size}`,
      )
    "
  >
    <slot />
  </motion.button>
</template>
