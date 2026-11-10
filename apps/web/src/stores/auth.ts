import { defineStore } from 'pinia'
import { ref } from 'vue'
import { csrfToken, setCSRFToken } from '@/lib/csrf'

export type AuthStatus = 'loading' | 'setup' | 'anonymous' | 'authenticated'
export type AuthBanner = '' | 'invalid' | 'rate' | 'network' | 'forbidden'
export type PasswordChangeResult = 'ok' | 'invalid' | 'weak' | 'same' | 'rate' | 'network'

interface StatusBody {
  setupRequired?: boolean
  setupTokenRequired?: boolean
  csrfToken?: string
}

interface SessionBody {
  username?: string
  csrfToken?: string
  error?: string
}

function devAuthMode(): 'setup' | 'login' | 'login-error' | 'session' | 'node' | null {
  if (!import.meta.env.DEV || typeof window === 'undefined') return null
  const params = new URLSearchParams(window.location.search)
  const mode = params.get('auth')
  if (mode === 'setup' || mode === 'login' || mode === 'login-error' || mode === 'session') return mode
  if (params.has('mock') || params.has('files') || params.has('explore') || params.has('peers') || params.has('pins') || params.has('scenes')) return 'node'
  return null
}

async function readJSON(res: Response): Promise<SessionBody> {
  try {
    return (await res.json()) as SessionBody
  } catch {
    return {}
  }
}

export const useAuthStore = defineStore('auth', () => {
  const status = ref<AuthStatus>('loading')
  const username = ref<string | null>(null)
  const setupTokenRequired = ref(false)
  const banner = ref<AuthBanner>('')
  const pending = ref(false)
  let boot: Promise<void> | null = null
  let generation = 0

  function applyDevMock(): boolean {
    const mode = devAuthMode()
    if (!mode) return false
    if (mode === 'setup') {
      const params = new URLSearchParams(window.location.search)
      status.value = 'setup'
      setupTokenRequired.value = params.get('token') === '1'
      return true
    }
    if (mode === 'login' || mode === 'login-error') {
      status.value = 'anonymous'
      banner.value = mode === 'login-error' ? 'invalid' : ''
      return true
    }
    status.value = 'authenticated'
    username.value = 'ada'
    setCSRFToken('dev-mock')
    return true
  }

  async function load(mine: number) {
    if (applyDevMock()) return
    try {
      const res = await fetch('/api/auth/setup-status', { credentials: 'same-origin' })
      if (mine !== generation) return
      if (!res.ok) throw new Error(String(res.status))
      const body = (await res.json()) as StatusBody
      if (mine !== generation) return
      setCSRFToken(typeof body.csrfToken === 'string' ? body.csrfToken : '')
      setupTokenRequired.value = body.setupTokenRequired === true
      if (body.setupRequired) {
        status.value = 'setup'
        return
      }
      const session = await fetch('/api/auth/session', { credentials: 'same-origin' })
      if (mine !== generation) return
      if (session.status === 401) {
        status.value = 'anonymous'
        return
      }
      if (!session.ok) throw new Error(String(session.status))
      const me = (await session.json()) as SessionBody
      if (mine !== generation) return
      username.value = typeof me.username === 'string' ? me.username : ''
      if (typeof me.csrfToken === 'string') setCSRFToken(me.csrfToken)
      status.value = 'authenticated'
    } catch {
      if (mine !== generation) return
      status.value = 'anonymous'
      banner.value = 'network'
    }
  }

  async function bootstrap() {
    if (status.value !== 'loading') return
    const mine = generation
    if (!boot) boot = load(mine)
    await boot
  }

  async function retry() {
    generation += 1
    status.value = 'loading'
    banner.value = ''
    boot = null
    await bootstrap()
  }

  async function login(name: string, password: string) {
    pending.value = true
    banner.value = ''
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken(),
        },
        body: JSON.stringify({ username: name, password }),
      })
      if (res.status === 401) {
        banner.value = 'invalid'
        return false
      }
      if (res.status === 429) {
        banner.value = 'rate'
        return false
      }
      if (res.status === 409) {
        status.value = 'setup'
        return false
      }
      if (res.status === 403) {
        banner.value = 'forbidden'
        return false
      }
      if (!res.ok) {
        banner.value = 'network'
        return false
      }
      const body = await readJSON(res)
      username.value = typeof body.username === 'string' ? body.username : name
      if (typeof body.csrfToken === 'string') setCSRFToken(body.csrfToken)
      status.value = 'authenticated'
      banner.value = ''
      return true
    } catch {
      banner.value = 'network'
      return false
    } finally {
      pending.value = false
    }
  }

  async function setup(name: string, password: string, setupToken: string) {
    pending.value = true
    banner.value = ''
    try {
      const res = await fetch('/api/auth/setup', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken(),
        },
        body: JSON.stringify({
          username: name,
          password,
          ...(setupToken ? { setupToken } : {}),
        }),
      })
      if (res.status === 403) {
        banner.value = 'forbidden'
        return false
      }
      if (res.status === 409) {
        status.value = 'anonymous'
        return false
      }
      if (res.status === 400) {
        banner.value = 'invalid'
        return false
      }
      if (!res.ok) {
        banner.value = 'network'
        return false
      }
      const body = await readJSON(res)
      username.value = typeof body.username === 'string' ? body.username : name
      if (typeof body.csrfToken === 'string') setCSRFToken(body.csrfToken)
      status.value = 'authenticated'
      banner.value = ''
      return true
    } catch {
      banner.value = 'network'
      return false
    } finally {
      pending.value = false
    }
  }

  async function changePassword(current: string, next: string): Promise<PasswordChangeResult> {
    pending.value = true
    try {
      const res = await fetch('/api/auth/password', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken(),
        },
        body: JSON.stringify({ currentPassword: current, newPassword: next }),
      })
      if (res.status === 401) return 'invalid'
      if (res.status === 429) return 'rate'
      if (res.status === 400) {
        const body = await readJSON(res)
        if (body.error === 'password_unchanged') return 'same'
        return 'weak'
      }
      if (!res.ok) return 'network'
      const body = await readJSON(res)
      if (typeof body.csrfToken === 'string') setCSRFToken(body.csrfToken)
      return 'ok'
    } catch {
      return 'network'
    } finally {
      pending.value = false
    }
  }

  async function logout() {
    try {
      await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken(),
        },
      })
    } catch {
      // The local session still ends. The cookie clear is best-effort.
    }
    username.value = null
    setCSRFToken('')
    banner.value = ''
    status.value = 'anonymous'
  }

  function markSignedOut() {
    username.value = null
    setCSRFToken('')
    if (status.value === 'authenticated') {
      status.value = 'anonymous'
      banner.value = ''
    }
  }

  return {
    status,
    username,
    setupTokenRequired,
    banner,
    pending,
    bootstrap,
    retry,
    login,
    setup,
    changePassword,
    logout,
    markSignedOut,
  }
})
