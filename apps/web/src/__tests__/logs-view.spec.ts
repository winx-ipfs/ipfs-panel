// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import LogsView from '@/views/LogsView.vue'

async function mountLogs(search = '', locale: 'en' | 'zh' = 'en') {
  const pinia = createPinia()
  setActivePinia(pinia)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'node', component: { template: '<div />' } },
      { path: '/logs', name: 'logs', component: LogsView },
    ],
  })
  await router.push({ path: `/logs${search}` })
  await router.isReady()
  const i18n = createI18n({ legacy: false, locale, messages: { en, zh } })
  const wrapper = mount(LogsView, {
    global: { plugins: [pinia, router, i18n] },
    attachTo: document.body,
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

describe('LogsView', () => {
  beforeEach(async () => {
    await import('@/dev/mockLogs')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
    window.history.replaceState({}, '', '/')
  })

  it('shows streaming lines, pause, and a filter with no match', async () => {
    window.history.replaceState({}, '', '/logs?logs=streaming')
    const streaming = await mountLogs('?logs=streaming')
    expect(streaming.get('[data-logs]').attributes('data-logs')).toBe('streaming')
    expect(streaming.text()).toContain('daemon is ready')
    expect(streaming.text()).toContain('Following')
    expect(streaming.text()).not.toContain('<script>')
    const pause = streaming.findAll('button').find((button) => button.text() === 'Pause')
    expect(pause).toBeTruthy()
    await pause!.trigger('click')
    expect(streaming.get('[data-logs]').attributes('data-logs')).toBe('paused')
    expect(streaming.text()).toContain('Paused')
    streaming.unmount()

    window.history.replaceState({}, '', '/logs?logs=filtered')
    const filtered = await mountLogs('?logs=filtered')
    expect(filtered.get('[data-logs]').attributes('data-logs')).toBe('filtered')
    expect(filtered.text()).toContain('gateway')
    expect(filtered.text()).not.toContain('daemon is ready')
    const search = filtered.get('input[type="search"]')
    await search.setValue('no-such-line')
    await new Promise((resolve) => setTimeout(resolve, 250))
    await flushPromises()
    expect(filtered.text()).toContain('No lines match.')
    filtered.unmount()
  })

  it('shows a stopped node and an error in English, and Chinese chrome', async () => {
    window.history.replaceState({}, '', '/logs?logs=stopped')
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const stopped = await mountLogs('?logs=stopped')
    expect(stopped.get('[data-logs]').attributes('data-logs')).toBe('stopped')
    expect(stopped.text()).toContain('The node is stopped.')
    expect(stopped.text()).toContain('Go to the node')
    expect(fetchMock).not.toHaveBeenCalled()
    stopped.unmount()

    window.history.replaceState({}, '', '/logs?logs=error')
    const failed = await mountLogs('?logs=error')
    expect(failed.get('[data-logs]').attributes('data-logs')).toBe('error')
    expect(failed.text()).toContain('The log could not be loaded.')
    expect(failed.text()).toContain('Retry')
    failed.unmount()

    window.history.replaceState({}, '', '/logs?logs=empty')
    const chinese = await mountLogs('?logs=empty', 'zh')
    expect(chinese.text()).toContain('还没有内容。')
    expect(chinese.text()).toContain('日志')
    chinese.unmount()
  })

  it('renders a message as text', async () => {
    window.history.replaceState({}, '', '/logs?logs=streaming')
    const wrapper = await mountLogs('?logs=streaming')
    const html = wrapper.get('.logs-scroll').html()
    expect(html).not.toContain('<img')
    expect(wrapper.text()).toContain('manager listening')
    wrapper.unmount()
  })
})
