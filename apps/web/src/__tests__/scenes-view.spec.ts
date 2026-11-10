// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { createRouter, createWebHistory } from 'vue-router'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import { changePhraseKey, resultKey } from '@/scenes/model'
import ScenesView from '@/views/ScenesView.vue'

const catalog = {
  ready: true,
  gatewayPort: 8081,
  nodeState: 'down',
  revert: true,
  scenes: [
    {
      id: 'quiet',
      active: false,
      restart: true,
      changes: [
        { key: 'Routing.Type', code: 'autoclient' },
        { key: 'Swarm.ConnMgr', code: 'quiet' },
      ],
    },
    { id: 'share', active: false, restart: true, changes: [{ key: 'Provide.Strategy', code: 'mfs' }] },
    { id: 'pinset', active: false, restart: true, changes: [{ key: 'Provide.Strategy', code: 'pinned' }] },
    {
      id: 'gateway',
      active: false,
      restart: true,
      changes: [{ key: 'Addresses.Gateway', code: 'public' }, { key: 'Gateway.NoFetch', code: 'local' }],
    },
  ],
}

async function mountPage(body = catalog, locale: 'en' | 'zh' = 'en') {
  const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (String(url).includes('/api/node')) {
      return Promise.resolve(
        new Response(JSON.stringify({ state: 'down', peerId: null, version: null, repoPath: '/tmp', error: null }), {
          status: 200,
        }),
      )
    }
    if (init?.method === 'POST') {
      return Promise.resolve(new Response(JSON.stringify({ id: 'quiet', restart: 'later' }), { status: 200 }))
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  })
  vi.stubGlobal('fetch', fetchMock)
  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/scenes', component: ScenesView },
    ],
  })
  await router.push('/scenes')
  const wrapper = mount(ScenesView, {
    global: {
      plugins: [createPinia(), router, createI18n({ legacy: false, locale, messages: { en, zh } })],
    },
  })
  await flushPromises()
  return { wrapper, fetchMock }
}

describe('scene phrases', () => {
  it('names the public gateway without a raw address', () => {
    expect(changePhraseKey({ key: 'Addresses.Gateway', code: 'public' })).toBe('scenesChangeGatewayPublic')
    expect(resultKey('gateway', 'started')).toBe('scenesResultGatewayStarted')
    expect(resultKey('quiet', 'later')).toBe('scenesResultQuietLater')
    expect(en.scenesChangeGatewayPublic).not.toMatch(/5001|0\.0\.0\.0/)
    expect(zh.scenesGatewaySummary).toContain('{port}')
  })
})

describe('ScenesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('lists the four scenes and previews a change while the node is stopped', async () => {
    const { wrapper } = await mountPage()
    expect(wrapper.text()).toContain('A quiet node on this machine')
    expect(wrapper.text()).toContain('Share a folder with someone')
    expect(wrapper.text()).not.toContain('5001')
    await wrapper.get('[data-scene="gateway"]').trigger('click')
    expect(wrapper.text()).toContain('The gateway listens for other computers.')
    expect(wrapper.text()).toContain('The node is stopped. This takes effect the next time you start it.')
    expect(wrapper.text()).not.toContain('0.0.0.0')
    wrapper.unmount()
  })

  it('shows the Chinese result sentence after apply', async () => {
    const { wrapper } = await mountPage(catalog, 'zh')
    await wrapper.get('[data-scene="quiet"]').trigger('click')
    await wrapper.get('[data-scene-apply]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('已保存。下次启动时，它会少为网络做一些事。')
    wrapper.unmount()
  })
})
