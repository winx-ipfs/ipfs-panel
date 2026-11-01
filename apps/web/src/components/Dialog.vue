<script setup lang="ts">
import { nextTick, onUnmounted, ref, useId, watch } from 'vue'
import { X } from '@lucide/vue'
import { focusable } from '@/lib/focus'
import IconButton from '@/components/IconButton.vue'

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

const titleId = useId()
const panel = ref<HTMLElement | null>(null)
let previous: HTMLElement | null = null

function onKey(event: KeyboardEvent) {
  if (!open.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    open.value = false
    return
  }
  if (event.key !== 'Tab' || !panel.value) return
  const items = focusable(panel.value)
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
    const items = panel.value ? focusable(panel.value) : []
    const preferred = panel.value?.querySelector<HTMLElement>('[data-autofocus]')
    ;(preferred ?? items[0] ?? panel.value)?.focus()
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
  <div v-if="open" class="ui-dialog-root">
    <button type="button" class="ui-dialog-scrim" tabindex="-1" :aria-label="closeLabel" @click="open = false" />
    <div
      ref="panel"
      class="ui-dialog-panel"
      :class="{ 'is-drawer': variant === 'drawer', 'is-scroll': scrollable }"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      tabindex="-1"
    >
      <div class="ui-dialog-head">
        <h2 :id="titleId" :class="{ 'sr-only': hideTitle }">{{ title }}</h2>
        <IconButton :label="closeLabel" @click="open = false">
          <X :size="18" :stroke-width="1.75" aria-hidden="true" />
        </IconButton>
      </div>
      <slot />
    </div>
  </div>
</template>
