import { defineStore } from 'pinia'
import { onScopeDispose, ref } from 'vue'

export const useToastStore = defineStore('toast', () => {
  const items = ref<{ id: number; message: string }[]>([])
  const timers = new Map<number, number>()
  let next = 1

  function dismiss(id: number) {
    const timer = timers.get(id)
    if (timer !== undefined) {
      window.clearTimeout(timer)
      timers.delete(id)
    }
    items.value = items.value.filter((item) => item.id !== id)
  }

  function show(message: string) {
    const id = next
    next += 1
    items.value = [...items.value, { id, message }]
    timers.set(
      id,
      window.setTimeout(() => {
        dismiss(id)
      }, 2400) as unknown as number,
    )
  }

  function clearTimers() {
    for (const timer of timers.values()) window.clearTimeout(timer)
    timers.clear()
  }

  onScopeDispose(clearTimers)

  return { items, show, dismiss }
})
