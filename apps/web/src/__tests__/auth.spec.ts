import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { decide, safeNext } from '@/auth/guard'
import { passwordAcceptable, passwordLevel, usernameAcceptable } from '@/auth/strength'
import { csrfToken, setCSRFToken } from '@/lib/csrf'
import { useAuthStore } from '@/stores/auth'

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('password strength and username', () => {
  it('grades length, repetition, and a short denylist', () => {
    expect(passwordLevel('')).toBe('empty')
    expect(passwordLevel('short')).toBe('short')
    expect(passwordAcceptable('short')).toBe(false)
    expect(passwordLevel('aaaaaaaaaaaa')).toBe('weak')
    expect(passwordAcceptable('aaaaaaaaaaaa')).toBe(false)
    expect(passwordLevel('password1234')).toBe('weak')
    expect(passwordAcceptable('password1234')).toBe(false)
    expect(passwordLevel('abcdefghijkl')).toBe('weak')
    expect(passwordAcceptable('abcdefghijkl')).toBe(true)
    expect(passwordLevel('correct horse')).toBe('ok')
    expect(passwordAcceptable('correct horse')).toBe(true)
    expect(passwordLevel('correct horse battery staple')).toBe('strong')
    expect(usernameAcceptable('ada')).toBe(true)
    expect(usernameAcceptable('Ada_1')).toBe(true)
    expect(usernameAcceptable('')).toBe(false)
    expect(usernameAcceptable('has space')).toBe(false)
    expect(usernameAcceptable('.hidden')).toBe(false)
  })
})

describe('route guard', () => {
  it('sends anonymous visitors to login and back to a same-app path', () => {
    expect(safeNext('/settings?tab=account')).toBe('/settings?tab=account')
    expect(safeNext('https://evil.example')).toBeNull()
    expect(safeNext('//evil.example')).toBeNull()
    expect(safeNext('/login')).toBeNull()
    expect(safeNext('/setup?x=1')).toBeNull()

    expect(decide('anonymous', { name: 'node', fullPath: '/settings', redirect: undefined })).toEqual({
      type: 'route',
      name: 'login',
      query: { redirect: '/settings' },
    })
    expect(decide('anonymous', { name: 'login', fullPath: '/login', redirect: undefined })).toEqual({
      type: 'allow',
    })
    expect(decide('setup', { name: 'node', fullPath: '/', redirect: undefined })).toEqual({
      type: 'route',
      name: 'setup',
    })
    expect(decide('setup', { name: 'login', fullPath: '/login', redirect: undefined })).toEqual({
      type: 'route',
      name: 'setup',
    })
    expect(decide('authenticated', { name: 'login', fullPath: '/login', redirect: '/pins' })).toEqual({
      type: 'path',
      path: '/pins',
    })
    expect(decide('authenticated', { name: 'login', fullPath: '/login', redirect: 'https://evil' })).toEqual({
      type: 'path',
      path: '/',
    })
    expect(decide('authenticated', { name: 'settings', fullPath: '/settings', redirect: undefined })).toEqual({
      type: 'allow',
    })
    expect(decide('anonymous', { name: 'setup', fullPath: '/setup', redirect: undefined })).toEqual({
      type: 'route',
      name: 'login',
    })
  })
})

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    setCSRFToken('')
  })

  it('follows setup-status into a session', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json({ setupRequired: false, setupTokenRequired: false, csrfToken: 'pre' }))
      .mockResolvedValueOnce(json({ username: 'ada', csrfToken: 'sess' }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useAuthStore()
    await store.bootstrap()
    expect(store.status).toBe('authenticated')
    expect(store.username).toBe('ada')
    expect(csrfToken()).toBe('sess')
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/auth/setup-status')
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/auth/session')
    vi.unstubAllGlobals()
  })

  it('stays on setup when no admin exists', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(json({ setupRequired: true, setupTokenRequired: true, csrfToken: 'pre' })),
    )
    const store = useAuthStore()
    await store.bootstrap()
    expect(store.status).toBe('setup')
    expect(store.setupTokenRequired).toBe(true)
    expect(csrfToken()).toBe('pre')
    vi.unstubAllGlobals()
  })

  it('logs in with the csrf header and a generic failure', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json({ setupRequired: false, csrfToken: 'pre' }))
      .mockResolvedValueOnce(json({ error: 'unauthorized' }, 401))
      .mockResolvedValueOnce(json({ error: 'invalid_credentials' }, 401))
      .mockResolvedValueOnce(json({ username: 'ada', csrfToken: 'after' }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useAuthStore()
    await store.bootstrap()
    expect(store.status).toBe('anonymous')

    expect(await store.login('ada', 'wrong-password-here')).toBe(false)
    expect(store.banner).toBe('invalid')
    expect(fetchMock.mock.calls[2]?.[1]).toMatchObject({
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'pre' },
      body: JSON.stringify({ username: 'ada', password: 'wrong-password-here' }),
    })

    expect(await store.login('ada', 'correct horse battery')).toBe(true)
    expect(store.status).toBe('authenticated')
    expect(csrfToken()).toBe('after')
    vi.unstubAllGlobals()
  })

  it('changes the password and logs out', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json({ setupRequired: false, csrfToken: 'pre' }))
      .mockResolvedValueOnce(json({ username: 'ada', csrfToken: 'sess' }))
      .mockResolvedValueOnce(json({ username: 'ada', csrfToken: 'rotated' }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useAuthStore()
    await store.bootstrap()
    expect(await store.changePassword('correct horse battery', 'a different passphrase')).toBe('ok')
    expect(csrfToken()).toBe('rotated')
    await store.logout()
    expect(store.status).toBe('anonymous')
    expect(store.username).toBeNull()
    expect(csrfToken()).toBe('')
    expect(fetchMock.mock.calls[3]?.[0]).toBe('/api/auth/logout')
    vi.unstubAllGlobals()
  })

  it('reports a rate limit and a network failure without a stack trace', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json({ setupRequired: false, csrfToken: 'pre' }))
      .mockResolvedValueOnce(json({ error: 'unauthorized' }, 401))
      .mockResolvedValueOnce(json({ error: 'rate_limited' }, 429))
      .mockRejectedValueOnce(new Error('offline'))
    vi.stubGlobal('fetch', fetchMock)
    const store = useAuthStore()
    await store.bootstrap()
    expect(await store.login('ada', 'correct horse battery')).toBe(false)
    expect(store.banner).toBe('rate')
    expect(await store.login('ada', 'correct horse battery')).toBe(false)
    expect(store.banner).toBe('network')
    vi.unstubAllGlobals()
  })
})
