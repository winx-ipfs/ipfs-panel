import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

describe('dev proxy', () => {
  it('forwards /api and /health to the manager default', () => {
    const source = readFileSync(new URL('../../vite.config.ts', import.meta.url), 'utf8')
    expect(source).toContain("'/api'")
    expect(source).toContain("'/health'")
    expect(source).toContain('http://127.0.0.1:8080')
    expect(source).not.toContain('5001')
  })
})