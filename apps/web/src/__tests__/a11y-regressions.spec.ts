// @vitest-environment happy-dom
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import Dialog from '@/components/Dialog.vue'
import Toast from '@/components/Toast.vue'
import { filesPreview } from '@/dev/mockFiles'
import { pinsPreview } from '@/dev/mockPins'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import { useFilesStore } from '@/stores/files'
import { usePinsStore } from '@/stores/pins'
import { useToastStore } from '@/stores/toast'
import FilesView from '@/views/FilesView.vue'
import PinsView from '@/views/PinsView.vue'

function i18n() {
  return createI18n({ legacy: false, locale: 'en', messages: { en, zh } })
}

function routerFor(path: string, component: typeof FilesView | typeof PinsView) {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path, name: path.slice(1), component }],
  })
}

async function mountFilesTable() {
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() => new Promise(() => undefined)))
  const pinia = createPinia()
  setActivePinia(pinia)
  const shot = filesPreview('?files=list')
  if (!shot) throw new Error('files preview fixture missing')
  useFilesStore(pinia).showPreview(shot.list, shot.uploads, shot.blocked)
  const wrapper = mount(FilesView, {
    global: { plugins: [pinia, routerFor('/files', FilesView), i18n()] },
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

async function mountPinsTable() {
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() => new Promise(() => undefined)))
  const pinia = createPinia()
  setActivePinia(pinia)
  const shot = pinsPreview('?pins=list')
  if (!shot) throw new Error('pins preview fixture missing')
  usePinsStore(pinia).showPreview(shot.list, shot.jobs, shot.blocked)
  const wrapper = mount(PinsView, {
    global: { plugins: [pinia, routerFor('/pins', PinsView), i18n()] },
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

function expectTableParity(wrapper: VueWrapper) {
  const table = wrapper.get('table')
  const headers = table.findAll('thead th').length
  expect(headers).toBeGreaterThan(0)
  for (const row of table.findAll('tbody tr')) {
    expect(row.findAll('td, th').length, 'every table body row must match its header column count').toBe(headers)
  }
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  window.history.replaceState({}, '', '/')
})

describe('table accessibility regressions', () => {
  it('keeps every FilesView body row aligned with its five headers', async () => {
    const wrapper = await mountFilesTable()
    expectTableParity(wrapper)
    wrapper.unmount()
  })

  it('keeps every PinsView body row aligned with its five headers', async () => {
    const wrapper = await mountPinsTable()
    expectTableParity(wrapper)
    wrapper.unmount()
  })
})

describe('dialog autofocus regression', () => {
  it('skips a disabled data-autofocus target and keeps focus inside the panel', async () => {
    const wrapper = mount(Dialog, {
      props: { open: false, title: 'Dialog', closeLabel: 'Close' },
      slots: { default: '<button disabled data-autofocus>Disabled</button><button class="target">Enabled</button>' },
      attachTo: document.body,
    })
    await wrapper.setProps({ open: true })
    await nextTick()
    await nextTick()
    const disabled = wrapper.get('button[disabled]').element
    const active = document.activeElement
    expect(active).not.toBe(document.body)
    expect(active).not.toBe(disabled)
    expect((active as HTMLElement | null)?.closest('.ui-dialog-panel')).not.toBeNull()
    wrapper.unmount()
  })
})

describe('toast accessibility and lifecycle regressions', () => {
  it('uses exactly one live-region semantic for each toast message', async () => {
    const pinia = createPinia()
    const wrapper = mount(Toast, { global: { plugins: [pinia] } })
    const toast = useToastStore(pinia)
    toast.show('x')
    await nextTick()
    await nextTick()
    const root = wrapper.element
    const live = root.matches('[aria-live], [role="status"], [role="alert"]') ? 1 : 0
    expect(live + root.querySelectorAll('[aria-live], [role="status"], [role="alert"]').length).toBe(1)
    wrapper.unmount()
    toast.$dispose()
  })

  it('clears toast timers when the store is disposed', async () => {
    vi.useFakeTimers()
    const pinia = createPinia()
    const wrapper = mount(Toast, { global: { plugins: [pinia] } })
    const toast = useToastStore(pinia)
    toast.show('x')
    await nextTick()
    expect(toast.items).toHaveLength(1)
    wrapper.unmount()
    toast.$dispose()
    vi.advanceTimersByTime(2400)
    expect(toast.items).toHaveLength(1)
  })
})