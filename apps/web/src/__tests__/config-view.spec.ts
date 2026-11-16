// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { createRouter, createWebHistory } from 'vue-router'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import SettingsView from '@/views/SettingsView.vue'

vi.mock('@/components/JsonCode.vue', () => ({
  default: {
    name: 'JsonCode',
    props: ['modelValue', 'label', 'invalidLabel', 'readOnly'],
    emits: ['update:modelValue'],
    template:
      '<textarea class="json-stand-in" :aria-label="label" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

const document = {
  text: '{\n  "Identity": {\n    "PeerID": "abc"\n  },\n  "Discovery": {\n    "MDNS": {\n      "Enabled": true\n    }\n  },\n  "Routing": {\n    "Type": "autoclient"\n  }\n}\n',
  scene: 'quiet',
  sceneKeys: ['Routing.Type'],
  backup: true,
}

async function openAdvanced(wrapper: Awaited<ReturnType<typeof mountSettings>>) {
  await wrapper.get('[id$="-tab-advanced"]').trigger('click')
  await vi.dynamicImportSettled()
  await flushPromises()
}

async function mountSettings(locale: 'en' | 'zh' = 'en') {
  const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (String(url).includes('/api/node')) {
      return Promise.resolve(new Response(JSON.stringify({ state: 'up', peerId: 'abc', version: 'kubo', repoPath: '/tmp', error: null }), { status: 200 }))
    }
    if (init?.method === 'POST' && String(url).endsWith('/api/config')) {
      return Promise.resolve(
        new Response(
          JSON.stringify({ ...document, text: JSON.parse((init.body as string) || '{}').text, restart: 'needed', diverged: true, scene: null }),
          { status: 200 },
        ),
      )
    }
    return Promise.resolve(new Response(JSON.stringify(document), { status: 200 }))
  })
  vi.stubGlobal('fetch', fetchMock)
  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/settings', component: SettingsView },
    ],
  })
  await router.push('/settings')
  const wrapper = mount(SettingsView, {
    global: {
      plugins: [createPinia(), router, createI18n({ legacy: false, locale, messages: { en, zh } })],
    },
  })
  await flushPromises()
  return wrapper
}

describe('Settings advanced config', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps the editor off the first tab and out of the private key', async () => {
    const wrapper = await mountSettings()
    const advanced = wrapper.get('[id$="-tab-advanced"]')
    expect(advanced.attributes('aria-selected')).toBe('false')
    expect(advanced.text()).toBe('Advanced')
    await openAdvanced(wrapper)
    expect(advanced.attributes('aria-selected')).toBe('true')
    expect(wrapper.get('.config-expert').text()).toBe('Expert')
    expect(wrapper.text()).toContain('private key')
    expect(wrapper.text()).not.toContain('PrivKey')
    expect(wrapper.text()).not.toContain('secret-key')
    const save = wrapper.findAll('button').find((button) => button.text() === 'Save')
    expect(save).toBeTruthy()
  })

  it('disables save until the JSON changes, then asks before writing', async () => {
    const wrapper = await mountSettings()
    await openAdvanced(wrapper)
    const save = wrapper.findAll('button').find((button) => button.text() === 'Save')
    expect(save?.attributes('disabled')).toBeDefined()
    await wrapper.get('textarea').setValue(`${document.text.slice(0, -2)},\n}\n`)
    expect(wrapper.text()).toContain('not valid')
    expect(save?.attributes('disabled')).toBeDefined()
    await wrapper.get('textarea').setValue(document.text.replace('true', 'false'))
    expect(save?.attributes('disabled')).toBeUndefined()
    await save?.trigger('click')
    expect(wrapper.text()).toContain('Review the change')
    await wrapper.get('button[class*="primary"], button').trigger('click')
  })

  it('says in Chinese that a restart is required after save', async () => {
    const wrapper = await mountSettings('zh')
    await openAdvanced(wrapper)
    await wrapper.get('textarea').setValue(document.text.replace('autoclient', 'auto'))
    const save = wrapper.findAll('button').find((button) => button.text() === '保存')
    await save?.trigger('click')
    const confirm = wrapper.findAll('button').find((button) => button.text() === '保存配置')
    await confirm?.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('重启节点才会用上')
    expect(wrapper.text()).toContain('离开')
  })
})
