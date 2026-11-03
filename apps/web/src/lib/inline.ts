export interface InlinePart {
  code: boolean
  text: string
}

/** Split locale text on backtick pairs. The backticks are markers, not characters to show. */
export function inlineParts(source: string): InlinePart[] {
  const parts: InlinePart[] = []
  const pattern = /`([^`]*)`/g
  let last = 0
  for (const match of source.matchAll(pattern)) {
    const index = match.index ?? 0
    if (index > last) parts.push({ code: false, text: source.slice(last, index) })
    parts.push({ code: true, text: match[1] ?? '' })
    last = index + match[0].length
  }
  if (last < source.length) parts.push({ code: false, text: source.slice(last) })
  if (parts.length === 0) parts.push({ code: false, text: source })
  return parts
}

/** Shorten a long token in the middle. Short values stay whole. */
export function shortenMiddle(value: string, head = 10, tail = 8): string {
  if (value.length <= head + tail + 1) return value
  return `${value.slice(0, head)}…${value.slice(-tail)}`
}
