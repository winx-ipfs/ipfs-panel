<script setup lang="ts">
import { nextTick, onUnmounted, ref, useId, watch } from 'vue'
import { X } from '@lucide/vue'
import { AnimatePresence, motion } from 'motion-v'
import { focusable } from '@/lib/focus'
import IconButton from '@/components/IconButton.vue'
import { useMotion } from '@/lib/motion'

const open = defineModel<boolean>('open', { default: false })

withDefaults(
  defineProps<{
    title: string
    closeLabel: string
    variant?: 'dialog' | 'drawer'
    hideTitle?: boolean
    scrollable?: boolean
  }>(),
  { variant: 'dialog', hideTitle: false, scrollable: false },
)

const { enter, shown, dialogTransition } = useMotion()
const titleId = useId()
const panel = ref<{ $el?: HTMLElement } | HTMLElement | null>(null)
let previous: HTMLElement | null = null

function panelEl(): HTMLElement | null {
  const value = panel.value
  if (!value) return null
  if (value instanceof HTMLElement) return value
  return value.$el ?? null
}

function onKey(event: KeyboardEvent) {
  if (!open.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    open.value = false
    return
  }
  const root = panelEl()
  if (event.key !== 'Tab' || !root) return
  const items = focusable(root)
  if (!items.length) return
  const first = items[0]
  const last = items[items.length - 1]
  if (!first || !last) return
  const active = document.activeElement
  if (event.shiftKey && active === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(open, async (value) => {
  document.documentElement.classList.toggle('scroll-lock', value)
  if (value) {
    previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    document.addEventListener('keydown', onKey)
    await nextTick()
    const root = panelEl()
    const items = root ? focusable(root) : []
    const preferred = root?.querySelector<HTMLElement>('[data-autofocus]')
    ;(preferred ?? items[0] ?? root)?.focus()
  } else {
    document.removeEventListener('keydown', onKey)
    previous?.focus()
    previous = null
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKey)
  document.documentElement.classList.remove('scroll-lock')
})
</script>

<template>
  <AnimatePresence>
    <div v-if="open" class="ui-dialog-root">
      <motion.button
        type="button"
        class="ui-dialog-scrim"
        tabindex="-1"
        :aria-label="closeLabel"
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :exit="{ opacity: 0 }"
        :transition="dialogTransition"
        @click="open = false"
      />
      <motion.div
        ref="panel"
        class="ui-dialog-panel"
        :class="{ 'is-drawer': variant === 'drawer', 'is-scroll': scrollable }"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
        :initial="variant === 'drawer' ? { opacity: 1, x: -12 } : { ...enter, scale: 0.98 }"
        :animate="{ ...shown, scale: 1, x: 0 }"
        :exit="variant === 'drawer' ? { opacity: 0, x: -12 } : { opacity: 0, y: 6, scale: 0.98 }"
        :transition="dialogTransition"
      >
        <div class="ui-dialog-head">
          <h2 :id="titleId" :class="{ 'sr-only': hideTitle }">{{ title }}</h2>
          <IconButton :label="closeLabel" @click="open = false">
            <X :size="18" :stroke-width="1.75" aria-hidden="true" />
          </IconButton>
        </div>
        <slot />
      </motion.div>
    </div>
  </AnimatePresence>
</template>
