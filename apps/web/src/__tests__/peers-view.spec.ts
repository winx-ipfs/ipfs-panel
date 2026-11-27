// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import PeersView from '@/views/PeersView.vue'

const qm = 'QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN'

async function mountPeers(search = '') {
  const pinia = createPinia()
  setActivePinia(pinia)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'node', component: { template: '<div />' } },
      { path: '/peers', name: 'peers', component: PeersView },
    ],
  })
  await router.push({ path: `/peers${search}` })
  await router.isReady()
  const i18n = createI18n({ legacy: false, locale: 'en', messages: { en, zh } })
  const wrapper = mount(PeersView, {
    global: { plugins: [pinia, router, i18n] },
    attachTo: document.body,
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

describe('PeersView', () => {
  beforeEach(async () => {
    await import('@/dev/mockPeers')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
    window.history.replaceState({}, '', '/')
  })

  it('shows a stopped node without calling the manager for a dev shot', async () => {
    window.history.replaceState({}, '', '/peers?peers=down')
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = await mountPeers('?peers=down')
    expect(wrapper.get('[data-peers]').attributes('data-peers')).toBe('down')
    expect(wrapper.text()).toContain('The node is stopped')
    expect(wrapper.text()).toContain('Go to the node')
    expect(wrapper.text()).not.toContain('Peer address')
    expect(fetchMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows an empty list and a populated list in English', async () => {
    window.history.replaceState({}, '', '/peers?peers=empty')
    const empty = await mountPeers('?peers=empty')
    expect(empty.get('[data-peers]').attributes('data-peers')).toBe('empty')
    expect(empty.text()).toContain('No one is connected yet.')
    expect(empty.text()).toContain('Peer address')
    empty.unmount()

    window.history.replaceState({}, '', '/peers?peers=list')
    const list = await mountPeers('?peers=list')
    expect(list.get('[data-peers]').attributes('data-peers')).toBe('list')
    expect(list.text()).toContain('4 connected')
    expect(list.text()).toContain('They connected')
    expect(list.text()).toContain('We connected')
    expect(list.text()).toContain('Both ways')
    expect(list.text()).toContain('< 1 ms')
    expect(list.text()).toContain('1.4 s')
    expect(list.text()).toContain(qm)
    expect(list.text()).not.toContain('/ip4/')
    expect(list.text()).not.toContain('5001')
    list.unmount()
  })

  it('does not call the manager for an address the page can already reject', async () => {
    window.history.replaceState({}, '', '/peers?peers=empty')
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = await mountPeers('?peers=empty')
    const input = wrapper.get('input[name="peer-address"]')
    await input.setValue('http://127.0.0.1:5001')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('That address is not one this page can dial.')
    expect(fetchMock).not.toHaveBeenCalled()
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('shows kept peers, apply, restart, and an in-flight keep', async () => {
    window.history.replaceState({}, '', '/peers?peers=kept')
    const kept = await mountPeers('?peers=kept')
    expect(kept.get('[data-peers]').attributes('data-peers')).toBe('kept')
    expect(kept.text()).toContain('Kept connected')
    expect(kept.text()).toContain('Connected')
    expect(kept.text()).toContain('Reconnecting')
    expect(kept.text()).toContain('/ip4/203.0.113.10/tcp/4001')
    expect(kept.text()).toContain('Kept')
    expect(kept.text()).not.toContain('5001')
    kept.unmount()

    window.history.replaceState({}, '', '/peers?peers=apply')
    const apply = await mountPeers('?peers=apply')
    expect(apply.get('[data-peers]').attributes('data-peers')).toBe('apply')
    expect(apply.text()).toContain('The saved peers are not what this node is keeping yet.')
    expect(apply.text()).toContain('Apply')
    expect(apply.text()).toContain('Restart node')
    apply.unmount()

    window.history.replaceState({}, '', '/peers?peers=restart')
    const restart = await mountPeers('?peers=restart')
    expect(restart.get('[data-peers]').attributes('data-peers')).toBe('restart')
    expect(restart.text()).toContain('The saved peers will be kept after the node restarts.')
    restart.unmount()

    window.history.replaceState({}, '', '/peers?peers=adding')
    const adding = await mountPeers('?peers=adding')
    expect(adding.get('[data-peers]').attributes('data-peers')).toBe('adding')
    expect(adding.text()).toContain('Keeping…')
    expect(adding.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    adding.unmount()
  })
})
