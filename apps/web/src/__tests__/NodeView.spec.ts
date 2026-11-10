// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import type { NodeOverview } from '@/node/types'
import NodeView from '@/views/NodeView.vue'

function overview(patch: Partial<NodeOverview> = {}): NodeOverview {
  return {
    state: 'down',
    peerId: null,
    version: null,
    repoPath: '/srv/ipfs-panel/kubo',
    error: null,
    ...patch,
  }
}

function mountCard(body: NodeOverview | null, locale: 'en' | 'zh' = 'en', fail = false) {
  const fetchMock = vi.fn().mockImplementation(() => {
    if (fail || !body) return Promise.reject(new Error('offline'))
    return Promise.resolve(
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
  vi.stubGlobal('fetch', fetchMock)
  const wrapper = mount(NodeView, {
    global: {
      plugins: [
        createPinia(),
        createI18n({ legacy: false, locale, messages: { en, zh } }),
      ],
    },
  })
  return { wrapper, fetchMock }
}

describe('NodeView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('shows the stopped sentence and only Start', async () => {
    const { wrapper } = mountCard(overview())
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('The node is stopped.')
    expect(wrapper.findAll('[data-node-action]').map((button) => button.text())).toEqual(['Start'])
    expect(wrapper.text()).toContain('Node folder')
    expect(wrapper.text()).toContain('/srv/ipfs-panel/kubo')
    expect(wrapper.text()).not.toContain('5001')
    wrapper.unmount()
  })

  it('shows version, peer id, Stop, and a quieter Restart while running', async () => {
    const { wrapper } = mountCard(
      overview({
        state: 'up',
        peerId: '12D3KooWPeer',
        version: 'kubo/0.32.1/',
        activity: { peers: 4, rateIn: 2048, rateOut: 0 },
      }),
    )
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('The node is running.')
    expect(wrapper.text()).toContain('Peers')
    expect(wrapper.text()).toContain('4')
    expect(wrapper.text()).toContain('In 2.0 KB/s')
    expect(wrapper.text()).toContain('Version')
    expect(wrapper.text()).toContain('kubo/0.32.1/')
    expect(wrapper.get('.peer').text()).toBe('12D3KooWPeer')
    const buttons = wrapper.findAll('[data-node-action]')
    expect(buttons.map((button) => button.text())).toEqual(['Stop', 'Restart'])
    expect(buttons[1]?.classes()).toContain('quiet')
    expect(wrapper.find('[data-copy="peer"]').exists()).toBe(true)
    expect(wrapper.find('[data-copy="version"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('hides buttons while starting and for a busy folder', async () => {
    const starting = mountCard(overview({ state: 'starting' }))
    await flushPromises()
    expect(starting.wrapper.get('[role="status"]').text()).toBe('The node is starting.')
    expect(starting.wrapper.findAll('[data-node-action]')).toHaveLength(0)
    starting.wrapper.unmount()

    const busy = mountCard(
      overview({ state: 'failed', error: { code: 'repo_busy', detail: 'held' } }),
    )
    await flushPromises()
    expect(busy.wrapper.get('[role="status"]').text()).toContain('Another Kubo')
    expect(busy.wrapper.findAll('[data-node-action]')).toHaveLength(0)
    expect(busy.wrapper.get('pre').text()).toBe('held')
    busy.wrapper.unmount()
  })

  it('does not replace a running node with the stopped sentence when the request fails', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify(overview({ state: 'up', peerId: '12D3KooWPeer', version: 'kubo/1/' })),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      )
      .mockRejectedValueOnce(new Error('offline'))
    vi.stubGlobal('fetch', fetchMock)
    vi.useFakeTimers()
    const wrapper = mount(NodeView, {
      global: {
        plugins: [createPinia(), createI18n({ legacy: false, locale: 'en', messages: { en, zh } })],
      },
    })
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('The node is running.')
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe(
      'The panel cannot reach the manager on this machine.',
    )
    expect(wrapper.text()).not.toContain('The node is stopped.')
    expect(wrapper.text()).toContain('12D3KooWPeer')
    expect(wrapper.findAll('[data-node-action]').map((button) => button.text())).toEqual(['Retry'])
    wrapper.unmount()
    vi.useRealTimers()
  })

  it('renders a missing Kubo command as a code chip and offers install help', async () => {
    const { wrapper } = mountCard(
      overview({
        state: 'failed',
        error: { code: 'binary_not_found', detail: 'ipfs not found on PATH' },
      }),
    )
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe(
      'Kubo is not installed, or the panel cannot find the ipfs command.',
    )
    expect(wrapper.get('[role="status"]').text()).not.toContain('`')
    expect(wrapper.get('code').text()).toBe('ipfs')
    expect(wrapper.findAll('[data-node-action]').map((button) => button.text())).toEqual(['Retry'])
    const link = wrapper.get('a[href="https://docs.ipfs.tech/install/command-line/"]')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.text()).toBe('How to install Kubo')
    wrapper.unmount()
  })

  it('shows the Chinese sentence', async () => {
    const { wrapper } = mountCard(overview(), 'zh')
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('节点已停止。')
    expect(wrapper.get('[data-node-action="start"]').text()).toBe('启动')
    wrapper.unmount()
  })

  it('posts start and disables the button while the request is in flight', async () => {
    let release: (value: Response) => void = () => {}
    const pending = new Promise<Response>((resolve) => {
      release = resolve
    })
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify(overview()), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockImplementationOnce(() => pending)
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mount(NodeView, {
      global: {
        plugins: [createPinia(), createI18n({ legacy: false, locale: 'en', messages: { en, zh } })],
      },
    })
    await flushPromises()
    const pendingClick = wrapper.get('[data-node-action="start"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-node-action="start"]').attributes('disabled')).toBeDefined()
    expect(fetchMock).toHaveBeenLastCalledWith('/api/node/start', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': '' },
      body: '{}',
    })
    release(
      new Response(JSON.stringify(overview({ state: 'starting' })), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await pendingClick
    await flushPromises()
    expect(wrapper.findAll('[data-node-action]')).toHaveLength(0)
    wrapper.unmount()
  })
})
