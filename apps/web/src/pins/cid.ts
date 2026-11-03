const b58 = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz'
const b32 = 'abcdefghijklmnopqrstuvwxyz234567'
const maxTarget = 2048
const maxName = 255
const maxDigest = 128

/** A CID or /ipfs/ path Kubo can pin. Surrounding whitespace is ignored. */
export function parseTarget(raw: string): string | null {
  const value = raw.trim()
  if (!value || byteLength(value) > maxTarget) return null
  if (value.startsWith('/ipfs/')) return parseIPFSPath(value)
  if (value.includes('/') || /[ \t]/.test(value)) return null
  return normalizeCID(value)
}

export function validPinName(name: string): boolean {
  if (!name) return true
  if (name.trim() !== name || byteLength(name) > maxName) return false
  if (/[/\\"]/.test(name)) return false
  for (const char of name) {
    const code = char.codePointAt(0) ?? 0
    if (code < 0x20 || code === 0x7f) return false
  }
  return true
}

function parseIPFSPath(value: string): string | null {
  if (value.includes('\\') || value.includes('\u0000')) return null
  const parts = value.slice('/ipfs/'.length).split('/')
  const head = parts[0]
  if (!head) return null
  const cid = normalizeCID(head)
  if (!cid) return null
  let out = `/ipfs/${cid}`
  for (const part of parts.slice(1)) {
    if (!part || part === '.' || part === '..' || byteLength(part) > maxName) return null
    for (const char of part) {
      const code = char.codePointAt(0) ?? 0
      if (code < 0x20 || code === 0x7f) return null
    }
    out += `/${part}`
  }
  if (byteLength(out) > maxTarget) return null
  return out
}

function normalizeCID(value: string): string | null {
  if (!value || value.includes('\u0000')) return null
  if (value.startsWith('Qm')) {
    const raw = decodeBase58(value)
    return raw && cidV0(raw) ? value : null
  }
  if (value.length < 2) return null
  const prefix = value[0]
  const body = value.slice(1)
  if (prefix === 'b' || prefix === 'B') {
    const raw = decodeBase32(body)
    return raw && cidV1(raw) ? `b${body.toLowerCase()}` : null
  }
  if (prefix === 'z') {
    const raw = decodeBase58(body)
    return raw && cidV1(raw) ? value : null
  }
  if (prefix === 'f' || prefix === 'F') {
    const raw = decodeBase16(body)
    return raw && cidV1(raw) ? `f${body.toLowerCase()}` : null
  }
  return null
}

function cidV0(raw: Uint8Array): boolean {
  return raw.length === 34 && raw[0] === 0x12 && raw[1] === 0x20
}

function cidV1(raw: Uint8Array): boolean {
  if (raw.length < 4 || raw.length > 256) return false
  let rest = raw
  const ver = readUvarint(rest)
  if (!ver || ver.value !== 1n) return false
  rest = rest.slice(ver.n)
  const codec = readUvarint(rest)
  if (!codec) return false
  rest = rest.slice(codec.n)
  const code = readUvarint(rest)
  if (!code) return false
  rest = rest.slice(code.n)
  const length = readUvarint(rest)
  if (!length || length.value === 0n || length.value > BigInt(maxDigest)) return false
  rest = rest.slice(length.n)
  return Number(length.value) === rest.length
}

function readUvarint(bytes: Uint8Array): { value: bigint; n: number } | null {
  let value = 0n
  let shift = 0n
  for (let i = 0; i < bytes.length; i += 1) {
    if (i > 9) return null
    const byte = bytes[i] ?? 0
    if (byte < 0x80) {
      if (i === 9 && byte > 1) return null
      return { value: value | (BigInt(byte) << shift), n: i + 1 }
    }
    value |= BigInt(byte & 0x7f) << shift
    shift += 7n
  }
  return null
}

function decodeBase58(value: string): Uint8Array | null {
  if (!value) return null
  let n = 0n
  for (const char of value) {
    const index = b58.indexOf(char)
    if (index < 0) return null
    n = n * 58n + BigInt(index)
  }
  let zeros = 0
  for (const char of value) {
    if (char !== '1') break
    zeros += 1
  }
  const body = n === 0n ? new Uint8Array() : hexToBytes(n.toString(16))
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
    const index = b32.indexOf(char)
    if (index < 0) return null
    buf = (buf << 5) | index
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

function hexToBytes(hex: string): Uint8Array {
  const text = hex.length % 2 ? `0${hex}` : hex
  const out = new Uint8Array(text.length / 2)
  for (let i = 0; i < out.length; i += 1) {
    out[i] = Number.parseInt(text.slice(i * 2, i * 2 + 2), 16)
  }
  return out
}

function byteLength(value: string): number {
  return new TextEncoder().encode(value).length
}
