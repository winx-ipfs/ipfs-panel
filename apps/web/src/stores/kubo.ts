import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchKubo, postKubo } from '@/kubo/api'
import type { KuboStatus } from '@/kubo/model'

export const useKuboStore = defineStore('kubo', () => {
  const status = ref<KuboStatus | null>(null)
  const acting = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null
  let session = 0

  function delay(): number {
    return status.value?.job?.state === 'running' ? 1000 : 5000
  }

  function clearTimer() {
    if (timer) clearTimeout(timer)
    timer = null
  }

  function arm() {
    clearTimer()
    const mine = session
    timer = setTimeout(() => {
      if (mine === session) void refresh()
    }, delay())
  }

  async function refresh(): Promise<void> {
    const mine = session
    clearTimer()
    try {
      const next = await fetchKubo()
      if (mine !== session || !next) return
      status.value = next
    } finally {
      if (mine === session) arm()
    }
  }

  async function run(action: 'install' | 'upgrade'): Promise<void> {
    const mine = session
    acting.value = true
    clearTimer()
    try {
      const job = await postKubo(action)
      if (mine !== session) return
      if (job && status.value) status.value = { ...status.value, job }
      const next = await fetchKubo()
      if (mine !== session || !next) return
      status.value = next
    } finally {
      acting.value = false
      if (mine === session) arm()
    }
  }

  function startPolling(): Promise<void> {
    session += 1
    clearTimer()
    return refresh()
  }

  function stopPolling() {
    session += 1
    clearTimer()
    acting.value = false
  }

  return {
    status,
    acting,
    startPolling,
    stopPolling,
    refresh,
    install: () => run('install'),
    upgrade: () => run('upgrade'),
  }
})
