import { computed } from 'vue'

const ease = [0.2, 0, 0, 1] as const

function reduced(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** Seconds. Zero when the operating system asks for less motion. */
export function motionSeconds(ms = 180): number {
  return reduced() ? 0 : ms / 1000
}

export function useMotion() {
  const press = computed(() => (reduced() ? {} : { scale: 0.98 }))
  const enter = computed(() => (reduced() ? { opacity: 1, y: 0 } : { opacity: 0, y: 6 }))
  const shown = { opacity: 1, y: 0 }
  const transition = computed(() => ({ duration: motionSeconds(180), ease }))
  const dialogTransition = computed(() => ({ duration: motionSeconds(200), ease }))
  return { press, enter, shown, transition, dialogTransition }
}
