import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useToastStore = defineStore('toast', () => {
  const items = ref<{ id: number; message: string }[]>([])
  let next = 1

  function show(message: string) {
    const id = next
    next += 1
    items.value = [...items.value, { id, message }]
    window.setTimeout(() => {
      items.value = items.value.filter((item) => item.id !== id)
    }, 2400)
  }

  return { items, show }
})
