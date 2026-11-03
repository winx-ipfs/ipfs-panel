// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import PinsView from '@/views/PinsView.vue'

const v1 = 'bafkreibceirceirceirceirceirceirceirceirceirceirceirceircei'

function mountPins(body: unknown, status = 200) {
  vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
    if (String(url).includes('/api/pins/jobs')) {
      return Promise.resolve(new Response(JSON.stringify({ jobs: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }))
    }
    return Promise.resolve(new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }))
  }))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'node', component: { template: '<div />' } },
      { path: '/pins', name: 'pins', component: PinsView },
    ],
  })
  return mount(PinsView, {
    global: {
      plugins: [createPinia(), router, createI18n({ legacy: false, locale: 'en', messages: { en, zh } })],
    },
  })
}

describe('PinsView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    window.history.replaceState({}, '', '/')
  })

  async function settle() {
    await import('@/dev/mockPins')
    await flushPromises()
    await flushPromises()
    await flushPromises()
  }

  it('lists pins and does not show the node RPC port', async () => {
    const wrapper = mountPins({
      namesFrom: 'kubo',
      localGateway: 'http://127.0.0.1:8081',
      maxBytes: 1024,
      pins: [
        { cid: v1, name: 'notes.txt', type: 'recursive', size: 2480 },
        { cid: 'QmPVGjYFugq4XUyBfoTHG6c3qxfBS26jEdaFM1gdAVuMZ2', name: '', type: 'direct' },
      ],
    })
    await settle()
    expect(wrapper.text()).toContain('notes.txt')
    expect(wrapper.text()).toContain('No name')
    expect(wrapper.text()).toContain('With its links')
    expect(wrapper.text()).toContain('2.4 KB')
    expect(wrapper.text()).not.toContain('5001')
    await wrapper.get('[aria-label="Actions for notes.txt"]').trigger('click')
    expect(wrapper.text()).toContain('Copy local link')
    expect(wrapper.text()).toContain('Stop keeping')
    wrapper.unmount()
  })

  it('shows an empty list and a stopped node', async () => {
    const empty = mountPins({ namesFrom: 'panel', maxBytes: 1024, pins: [] })
    await settle()
    expect(empty.text()).toContain('Nothing is pinned yet')
    empty.unmount()

    const stopped = mountPins({ error: 'node_not_running', state: 'down' }, 503)
    await settle()
    expect(stopped.text()).toContain('The node is stopped')
    expect(stopped.get('a.ui-btn').attributes('href')).toBe('/')
    expect(stopped.text()).not.toContain('5001')
    stopped.unmount()
  })

  it('explains a bad content ID and asks before unpinning', async () => {
    const wrapper = mountPins({
      namesFrom: 'kubo',
      localGateway: 'http://127.0.0.1:8081',
      maxBytes: 1024,
      pins: [{ cid: v1, name: 'notes.txt', type: 'recursive', size: 4 }],
    })
    await settle()
    await wrapper.get('button.ui-btn.is-primary').trigger('click')
    await wrapper.get('input[name="pin-target"]').setValue('not-a-cid')
    expect(wrapper.text()).toContain('That does not look like a content ID')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[aria-label="Close"]').trigger('click')
    await wrapper.get('[aria-label="Actions for notes.txt"]').trigger('click')
    await wrapper.get('.pins-menu .is-danger').trigger('click')
    expect(wrapper.text()).toContain('Stop keeping this?')
    expect(wrapper.get('[data-autofocus]').text()).toBe('Cancel')
    wrapper.unmount()
  })
})
