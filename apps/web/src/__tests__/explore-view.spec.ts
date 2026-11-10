// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import ExploreView from '@/views/ExploreView.vue'

const cid = 'bafkqac3imvwgy3zao5xxe3de'

async function mountExplore(query?: Record<string, string>) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'node', component: { template: '<div />' } },
      { path: '/explore', name: 'explore', component: ExploreView },
    ],
  })
  await router.push({ name: 'explore', query })
  await router.isReady()
  const i18n = createI18n({ legacy: false, locale: 'en', messages: { en, zh } })
  const wrapper = mount(ExploreView, {
    global: { plugins: [pinia, router, i18n] },
  })
  return { wrapper, router, i18n }
}

describe('ExploreView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    window.history.replaceState({}, '', '/')
  })

  it('starts empty and does not call the manager', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const { wrapper } = await mountExplore()
    await flushPromises()
    expect(wrapper.text()).toContain('Nothing is open yet')
    expect(wrapper.text()).toContain('Explore')
    expect(wrapper.text()).not.toContain('5001')
    expect(fetchMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows a folder and a local link without a Kubo RPC address', async () => {
    const body = {
      target: `/ipfs/${cid}`,
      cid,
      version: 1,
      codec: 'dag-pb',
      hash: 'sha2-256',
      kind: 'folder',
      links: [
        { name: 'notes.txt', cid: 'bafkreibceirceirceirceirceirceirceirceirceirceirceirceircei', size: 2480 },
        { name: '会议记录.txt', cid: 'bafkreibceirceirceirceirceirceirceirceirceirceirceirceircem', size: 18432 },
      ],
      localGateway: 'http://127.0.0.1:8081',
    }
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))))
    const { wrapper } = await mountExplore({ target: cid })
    await flushPromises()
    await flushPromises()
    expect(wrapper.text()).toContain('Folder')
    expect(wrapper.text()).toContain('notes.txt')
    expect(wrapper.text()).toContain('会议记录.txt')
    expect(wrapper.text()).toContain('Copy local link')
    expect(wrapper.text()).not.toContain('5001')
    expect(wrapper.text()).not.toContain('/api/v0')
    wrapper.unmount()
  })

  it('shows the Chinese start state', async () => {
    const { wrapper, i18n } = await mountExplore()
    i18n.global.locale.value = 'zh'
    await flushPromises()
    expect(wrapper.text()).toContain('还没有打开任何东西')
    expect(wrapper.text()).toContain('探索')
    wrapper.unmount()
  })

  it('reports a missing block and keeps the value', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ error: 'not_found' }), {
      status: 404,
      headers: { 'Content-Type': 'application/json' },
    }))))
    const { wrapper } = await mountExplore({ target: cid })
    await flushPromises()
    await flushPromises()
    expect(wrapper.text()).toContain('The node could not find that')
    expect(wrapper.text()).not.toContain('5001')
    wrapper.unmount()
  })
})
