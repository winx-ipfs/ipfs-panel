<script setup lang="ts">
import { nextTick, useId } from 'vue'

const model = defineModel<string>({ required: true })

const props = defineProps<{
  tabs: { id: string; label: string }[]
  label: string
}>()

const baseId = useId()

function tabId(id: string) {
  return `${baseId}-tab-${id}`
}

function panelId(id: string) {
  return `${baseId}-panel-${id}`
}

async function onKey(event: KeyboardEvent, index: number) {
  const key = event.key
  if (key !== 'ArrowRight' && key !== 'ArrowLeft' && key !== 'Home' && key !== 'End') return
  event.preventDefault()
  const last = props.tabs.length - 1
  let next = index
  if (key === 'ArrowRight') next = index === last ? 0 : index + 1
  if (key === 'ArrowLeft') next = index === 0 ? last : index - 1
  if (key === 'Home') next = 0
  if (key === 'End') next = last
  const tab = props.tabs[next]
  if (!tab) return
  model.value = tab.id
  await nextTick()
  document.getElementById(tabId(tab.id))?.focus()
}
</script>

<template>
  <div class="ui-tabs">
    <div class="ui-tabs-list" role="tablist" :aria-label="label">
      <button
        v-for="(tab, index) in tabs"
        :id="tabId(tab.id)"
        :key="tab.id"
        type="button"
        class="ui-tab"
        role="tab"
        :aria-selected="model === tab.id"
        :aria-controls="panelId(tab.id)"
        :tabindex="model === tab.id ? 0 : -1"
        @click="model = tab.id"
        @keydown="onKey($event, index)"
      >
        {{ tab.label }}
      </button>
    </div>
    <div
      v-for="tab in tabs"
      v-show="model === tab.id"
      :id="panelId(tab.id)"
      :key="tab.id"
      role="tabpanel"
      :aria-labelledby="tabId(tab.id)"
      tabindex="0"
    >
      <slot :name="tab.id" />
    </div>
  </div>
</template>
