// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { createRouter, createWebHistory } from 'vue-router'
import { parseAccess, parseSessions } from '@/access/model'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import SettingsView from '@/views/SettingsView.vue'

const snap = {
  loopback: false,
  remote: true,
  scheme: 'https',
  tls: 'auto',
  insecure: false,
  trustProxy: false,
  fingerprint: 'ab:cd:ef',
  listen: ['0.0.0.0:8443'],
  reachable: ['192.0.2.10:8443'],
  hosts: ['panel.example'],
}

const sessions = {
  sessions: [
    { id: 'current-id', current: true, created: '2026-11-27T00:00:00Z', seen: '2026-11-27T01:00:00Z', ip: '192.0.2.20' },
    { id: 'other-id', current: false, created: '2026-11-27T00:00:00Z', seen: '2026-11-26T01:00:00Z', ip: '198.51.100.8' },
  ],
}

async function mountSettings(locale: 'en' | 'zh', body = snap) {
  const fetchMock = vi.fn().mockImplementation((url: string) => {
    const path = String(url)
    if (path.endsWith('/api/access/sessions/revoke-others')) {
      return Promise.resolve(new Response(null, { status: 204 }))
    }
    if (path.endsWith('/api/access/sessions/revoke')) {
      return Promise.resolve(new Response(null, { status: 204 }))
    }
    if (path.endsWith('/api/access/sessions')) {
      return Promise.resolve(new Response(JSON.stringify(sessions), { status: 200 }))
    }
    if (path.endsWith('/api/access')) {
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
    }
    return Promise.resolve(new Response(JSON.stringify({ state: 'up' }), { status: 200 }))
  })
  vi.stubGlobal('fetch', fetchMock)
  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/', name: 'node', component: { template: '<div />' } },
      { path: '/settings', name: 'settings', component: SettingsView },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
  })
  await router.push('/settings')
  const wrapper = mount(SettingsView, {
    global: {
      plugins: [createPinia(), router, createI18n({ legacy: false, locale, messages: { en, zh } })],
    },
  })
  await flushPromises()
  return { wrapper, fetchMock, router }
}

describe('access settings', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
  })

  it('parses the access document and drops a Kubo address if one were added', () => {
    const parsed = parseAccess(snap)
    expect(parsed?.fingerprint).toBe('ab:cd:ef')
    expect(parsed?.listen).toEqual(['0.0.0.0:8443'])
    expect(parseAccess({ tls: 'off' })).toBeNull()
    expect(parseSessions(sessions)?.map((row) => row.id)).toEqual(['current-id', 'other-id'])
    expect(JSON.stringify(parsed)).not.toContain('5001')
  })

  it('shows the remote warning, the fingerprint, and both sessions', async () => {
    const { wrapper } = await mountSettings('en')
    await wrapper.get('[id$="-tab-access"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Compare the certificate fingerprint')
    expect(wrapper.text()).toContain('ab:cd:ef')
    expect(wrapper.text()).toContain('0.0.0.0:8443')
    expect(wrapper.text()).toContain('192.0.2.10:8443')
    expect(wrapper.text()).toContain('This browser')
    expect(wrapper.text()).toContain('Another browser')
    expect(wrapper.text()).toContain('Kubo\'s control port stays on this computer')
    expect(wrapper.text()).not.toContain('5001')
  })

  it('shows the cleartext warning in Chinese', async () => {
    const { wrapper } = await mountSettings('zh', { ...snap, tls: 'off', insecure: true, scheme: 'http', fingerprint: '' })
    await wrapper.get('[id$="-tab-access"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('连接没有加密')
    expect(wrapper.text()).toContain('只在你信任的网络上这样用')
    expect(wrapper.text()).not.toContain('ab:cd')
  })

  it('asks before revoking another session', async () => {
    const { wrapper, fetchMock } = await mountSettings('en')
    await wrapper.get('[id$="-tab-access"]').trigger('click')
    await flushPromises()
    const revoke = wrapper.findAll('button').filter((button) => button.text() === 'Revoke')
    expect(revoke.length).toBe(2)
    await revoke[1]?.trigger('click')
    expect(wrapper.text()).toContain('That browser will need to sign in again.')
    const confirm = wrapper.findAll('button').find((button) => button.text() === 'Revoke' && button.classes().some((name) => name.includes('primary') || name.includes('default')))
    await (confirm ?? wrapper.findAll('button').find((button) => button.text() === 'Revoke'))?.trigger('click')
    await flushPromises()
    const calls = fetchMock.mock.calls.map((call) => String(call[0]))
    expect(calls.some((url) => url.includes('/api/access/sessions/revoke'))).toBe(true)
  })
})
