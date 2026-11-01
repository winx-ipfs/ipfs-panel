// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import FilesView from '@/views/FilesView.vue'
import LoginView from '@/views/LoginView.vue'
import SetupView from '@/views/SetupView.vue'

function mountFiles(body: unknown, status = 200) {
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  }))))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'node', component: { template: '<div />' } },
      { path: '/files', name: 'files', component: FilesView },
    ],
  })
  const wrapper = mount(FilesView, {
    global: {
      plugins: [createPinia(), router, createI18n({ legacy: false, locale: 'en', messages: { en, zh } })],
    },
  })
  return wrapper
}

describe('FilesView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    window.history.replaceState({}, '', '/')
  })

  async function settle() {
    await import('@/dev/mockFiles')
    await flushPromises()
    await flushPromises()
  }

  it('lists files and does not show the node RPC port', async () => {
    const wrapper = mountFiles({
      path: '/',
      localGateway: 'http://127.0.0.1:8081',
      maxBytes: 512 * 1024 * 1024,
      entries: [
        { name: 'notes.txt', type: 'file', size: 2480, cid: 'bafybeigdyrzt5sfp7k4notes', modified: '2026-11-01T03:12:00Z' },
        { name: 'photos', type: 'directory', size: 0, cid: 'bafybeigdyrzt5sfp7k4photo', modified: null },
      ],
    })
    await settle()
    expect(wrapper.text()).toContain('notes.txt')
    expect(wrapper.text()).toContain('photos')
    expect(wrapper.text()).toContain('2.4 KB')
    expect(wrapper.text()).not.toContain('5001')
    await wrapper.get('[aria-label="Actions for notes.txt"]').trigger('click')
    expect(wrapper.text()).toContain('Copy local link')
    wrapper.unmount()
  })

  it('shows an empty folder and a stopped node', async () => {
    const empty = mountFiles({ path: '/', localGateway: '', maxBytes: 1024, entries: [] })
    await settle()
    expect(empty.text()).toContain('This folder is empty')
    empty.unmount()

    const stopped = mountFiles({ error: 'node_not_running', state: 'down' }, 503)
    await settle()
    expect(stopped.text()).toContain('The node is stopped')
    expect(stopped.get('a').attributes('href')).toBe('/')
    expect(stopped.text()).not.toContain('5001')
    stopped.unmount()
  })

  it('asks before deleting', async () => {
    const wrapper = mountFiles({
      path: '/',
      localGateway: 'http://127.0.0.1:8081',
      maxBytes: 1024,
      entries: [{ name: 'notes.txt', type: 'file', size: 4, cid: 'bafyabc', modified: null }],
    })
    await settle()
    await wrapper.get('[aria-label="Actions for notes.txt"]').trigger('click')
    await wrapper.get('.files-menu .is-danger').trigger('click')
    expect(wrapper.text()).toContain('Delete notes.txt?')
    expect(wrapper.get('[data-autofocus]').text()).toBe('Cancel')
    wrapper.unmount()
  })
})

describe('auth polish', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('keeps the username after a failed login and can show the password', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'invalid_credentials' }), { status: 401 })))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', name: 'node', component: { template: '<div />' } },
        { path: '/login', name: 'login', component: LoginView },
        { path: '/setup', name: 'setup', component: { template: '<div />' } },
      ],
    })
    const wrapper = mount(LoginView, {
      global: {
        plugins: [createPinia(), router, createI18n({ legacy: false, locale: 'en', messages: { en, zh } })],
      },
    })
    await wrapper.get('input[name=username]').setValue('ada')
    await wrapper.get('input[name=password]').setValue('not-the-password')
    expect(wrapper.get('input[name=password]').attributes('type')).toBe('password')
    await wrapper.get('[data-password-toggle]').trigger('click')
    expect(wrapper.get('input[name=password]').attributes('type')).toBe('text')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect((wrapper.get('input[name=username]').element as HTMLInputElement).value).toBe('ada')
    expect(wrapper.text()).toContain('That username or password is not right.')
    wrapper.unmount()
  })

  it('keeps setup disabled until the form is valid', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/setup', name: 'setup', component: SetupView }],
    })
    const wrapper = mount(SetupView, {
      global: {
        plugins: [createPinia(), router, createI18n({ legacy: false, locale: 'en', messages: { en, zh } })],
      },
    })
    const button = wrapper.get('button[type=submit]')
    expect(button.attributes('disabled')).toBeDefined()
    await wrapper.get('input[name=username]').setValue('ada')
    const passwords = wrapper.findAll('input[type=password]')
    await passwords[0]?.setValue('correct horse battery')
    await passwords[1]?.setValue('correct horse battery')
    expect(button.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})
