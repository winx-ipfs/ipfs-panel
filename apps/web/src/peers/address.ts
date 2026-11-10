const maxAddressBytes = 2048
const maxDigest = 128
const b58Alphabet = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz'
const b32Alphabet = 'abcdefghijklmnopqrstuvwxyz234567'

export interface DialAddress {
  address: string
  peerId: string
}

// parseAddress matches the manager's rules. An empty or rejected value does
// not get sent.
export function parseAddress(raw: string): DialAddress | null {
  const value = raw.trim()
  if (!value || value.length > maxAddressBytes || !value.startsWith('/')) return null
  for (const char of value) {
    const code = char.codePointAt(0) ?? 0
    if (code > 127 || code < 0x20 || code === 0x7f || '\\?#@'.includes(char) || /\s/u.test(char)) return null
  }
  const parts = value.split('/')
  if (parts.length < 3 || parts[0] !== '') return null
  let index = 1
  let endedWithPeer = false
  let peerId = ''
  let peerAt = -1
  while (index < parts.length) {
    const name = parts[index] ?? ''
    if (!name) return null
    if (bare(name)) {
      endedWithPeer = false
      index += 1
      continue
    }
    if (!valued(name) || index + 1 >= parts.length) return null
    const component = parts[index + 1] ?? ''
    if (!validValue(name, component)) return null
    if (name === 'p2p' || name === 'ipfs') {
      endedWithPeer = true
      peerId = canonicalPeer(component)
      peerAt = index + 1
    } else {
      endedWithPeer = false
    }
    index += 2
  }
  if (!endedWithPeer || !peerId || peerAt < 0) return null
  parts[peerAt] = peerId
  return { address: parts.join('/'), peerId }
}

function bare(name: string): boolean {
  return ['quic', 'quic-v1', 'webtransport', 'webrtc', 'webrtc-direct', 'tls', 'ws', 'wss', 'p2p-circuit'].includes(name)
}

function valued(name: string): boolean {
  return ['ip4', 'ip6', 'tcp', 'udp', 'dns', 'dns4', 'dns6', 'dnsaddr', 'sni', 'certhash', 'p2p', 'ipfs'].includes(name)
}

function validValue(name: string, value: string): boolean {
  if (name === 'ip4') return ipv4(value)
  if (name === 'ip6') return ipv6(value)
  if (name === 'tcp' || name === 'udp') return port(value)
  if (name === 'dns' || name === 'dns4' || name === 'dns6' || name === 'dnsaddr' || name === 'sni') return hostname(value)
  if (name === 'certhash') return token(value)
  if (name === 'p2p' || name === 'ipfs') return peerId(value)
  return false
}

function ipv4(value: string): boolean {
  const parts = value.split('.')
  if (parts.length !== 4) return false
  return parts.every((part) => {
    if (!/^[0-9]+$/.test(part) || (part.length > 1 && part.startsWith('0'))) return false
    const n = Number(part)
    return n >= 0 && n <= 255
  })
}

function ipv6(value: string): boolean {
  if (!value || value.includes('%')) return false
  return /^(?:[0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$/.test(value) && value.includes(':')
}

function port(value: string): boolean {
  if (!/^[0-9]+$/.test(value) || (value.length > 1 && value.startsWith('0'))) return false
  const n = Number(value)
  return n >= 1 && n <= 65535
}

function hostname(value: string): boolean {
  if (!value || value.length > 253) return false
  return value.split('.').every((label) => {
    if (!label || label.length > 63) return false
    return /^[A-Za-z0-9_](?:[A-Za-z0-9_-]*[A-Za-z0-9_])?$/.test(label) || /^[A-Za-z0-9_]$/.test(label)
  })
}

function token(value: string): boolean {
  return value.length > 0 && value.length <= 256 && /^[A-Za-z0-9+/=_-]+$/.test(value)
}

function canonicalPeer(value: string): string {
  if (value.startsWith('Qm') || value.startsWith('1') || value.startsWith('z')) return value
  if (value.length > 1 && (value[0] === 'b' || value[0] === 'B')) return `b${value.slice(1).toLowerCase()}`
  if (value.length > 1 && (value[0] === 'f' || value[0] === 'F')) return `f${value.slice(1).toLowerCase()}`
  return value
}

function peerId(value: string): boolean {
  if (!value || value.length > 256) return false
  if (value.startsWith('Qm')) {
    const raw = decodeBase58(value)
    return raw !== null && raw.length === 34 && raw[0] === 0x12 && raw[1] === 0x20
  }
  if (value.startsWith('1')) {
    const raw = decodeBase58(value)
    return raw !== null && identityOrSha2(raw)
  }
  const raw = cidBytes(value)
  if (!raw) return false
  const version = takeUvarint(raw, 0)
  if (!version || version.value !== 1) return false
  const codec = takeUvarint(raw, version.next)
  if (!codec || codec.value !== 0x72) return false
  return identityOrSha2(raw.subarray(codec.next))
}

function identityOrSha2(raw: Uint8Array): boolean {
  const code = takeUvarint(raw, 0)
  if (!code) return false
  const length = takeUvarint(raw, code.next)
  if (!length || length.value === 0 || length.value > maxDigest) return false
  if (raw.length - length.next !== length.value) return false
  if (code.value === 0x00) return true
  return code.value === 0x12 && length.value === 32
}

function cidBytes(value: string): Uint8Array | null {
  if (value.length < 2) return null
  const prefix = value[0]
  const body = value.slice(1)
  if (prefix === 'b' || prefix === 'B') return decodeBase32(body)
  if (prefix === 'z') return decodeBase58(body)
  if (prefix === 'f' || prefix === 'F') return decodeBase16(body)
  return null
}

function takeUvarint(bytes: Uint8Array, start: number): { value: number; next: number } | null {
  let x = 0
  let shift = 0
  for (let i = 0; i < 10; i += 1) {
    if (start + i >= bytes.length) return null
    const c = bytes[start + i] ?? 0
    if (c < 0x80) {
      if (i === 9 && c > 1) return null
      return { value: x | (c << shift), next: start + i + 1 }
    }
    x |= (c & 0x7f) << shift
    shift += 7
  }
  return null
}

function decodeBase58(value: string): Uint8Array | null {
  if (!value) return null
  let n = 0n
  for (const char of value) {
    const digit = b58Alphabet.indexOf(char)
    if (digit < 0) return null
    n = n * 58n + BigInt(digit)
  }
  let hex = n.toString(16)
  if (hex.length % 2) hex = `0${hex}`
  const body = hex === '00' && n === 0n ? new Uint8Array() : decodeBase16(hex)
  if (!body) return null
  let zeros = 0
  while (zeros < value.length && value[zeros] === '1') zeros += 1
  const out = new Uint8Array(zeros + body.length)
  out.set(body, zeros)
  return out
}

function decodeBase32(value: string): Uint8Array | null {
  const text = value.toLowerCase()
  if (!text || text.includes('=')) return null
  let buf = 0
  let bits = 0
  const out: number[] = []
  for (const char of text) {
    const digit = b32Alphabet.indexOf(char)
    if (digit < 0) return null
    buf = (buf << 5) | digit
    bits += 5
    if (bits >= 8) {
      bits -= 8
      out.push((buf >> bits) & 0xff)
      buf &= (1 << bits) - 1
    }
  }
  if (bits > 0 && buf !== 0) return null
  return Uint8Array.from(out)
}

function decodeBase16(value: string): Uint8Array | null {
  const text = value.toLowerCase()
  if (!text || text.length % 2 !== 0) return null
  const out = new Uint8Array(text.length / 2)
  for (let i = 0; i < out.length; i += 1) {
    const hi = fromHex(text[i * 2] ?? '')
    const lo = fromHex(text[i * 2 + 1] ?? '')
    if (hi < 0 || lo < 0) return null
    out[i] = (hi << 4) | lo
  }
  return out
}

function fromHex(char: string): number {
  const code = char.charCodeAt(0)
  if (code >= 48 && code <= 57) return code - 48
  if (code >= 97 && code <= 102) return code - 97 + 10
  return -1
}
