export type PasswordLevel = 'empty' | 'short' | 'weak' | 'ok' | 'strong'

const denied = new Set([
  'password1234',
  'password12345',
  'passwordpassword',
  '123456789012',
  '1234567890123',
  'qwertyuiopas',
  'qwertyuiop123',
  'adminadminadmin',
  'letmeinletmein',
  'changeme1234',
  'ipfs-panel123',
  'kubokubokubo1',
])

const usernamePattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

export function usernameAcceptable(name: string): boolean {
  return usernamePattern.test(name)
}

function runeLength(value: string): number {
  return [...value].length
}

function repeated(value: string): boolean {
  const chars = [...value]
  if (chars.length === 0) return true
  return chars.every((char) => char === chars[0])
}

function classes(value: string): number {
  let bits = 0
  if (/[a-z]/.test(value)) bits += 1
  if (/[A-Z]/.test(value)) bits += 1
  if (/[0-9]/.test(value)) bits += 1
  if (/[^a-zA-Z0-9]/.test(value)) bits += 1
  return bits
}

/** Hint for the meter. The server still accepts some passwords the meter calls weak. */
export function passwordLevel(password: string): PasswordLevel {
  const length = runeLength(password)
  if (length === 0) return 'empty'
  if (length < 12 || length > 128) return 'short'
  if (repeated(password) || denied.has(password.toLowerCase())) return 'weak'
  const variety = classes(password)
  if (length >= 20 || (length >= 16 && variety >= 3)) return 'strong'
  if (length >= 16 || variety >= 2) return 'ok'
  return 'weak'
}

/** The rule the server enforces. A weak-but-long password can still pass. */
export function passwordAcceptable(password: string): boolean {
  const length = runeLength(password)
  if (length < 12 || length > 128) return false
  if (password.includes('\u0000')) return false
  if (repeated(password)) return false
  if (denied.has(password.toLowerCase())) return false
  return true
}
