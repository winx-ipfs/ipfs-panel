// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import Input from '@/components/Input.vue'
import Select from '@/components/Select.vue'
import Switch from '@/components/Switch.vue'

describe('form controls', () => {
  it('associates a label with the input and surfaces an error', () => {
    const wrapper = mount(Input, {
      props: { label: 'Name', modelValue: '', error: 'Required' },
    })
    const input = wrapper.get('input')
    expect(wrapper.get('label').attributes('for')).toBe(input.attributes('id'))
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(wrapper.text()).toContain('Required')
  })

  it('renders every select option', () => {
    const wrapper = mount(Select, {
      props: {
        label: 'Language',
        modelValue: 'en',
        options: [
          { value: 'en', label: 'English' },
          { value: 'zh', label: '中文' },
        ],
      },
    })
    expect(wrapper.findAll('option').map((option) => option.text())).toEqual(['English', '中文'])
  })

  it('toggles a switch', async () => {
    const wrapper = mount(Switch, { props: { label: 'Match device', modelValue: false } })
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([true])
  })
})
