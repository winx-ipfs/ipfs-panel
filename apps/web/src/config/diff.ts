export type DiffRow =
  | { kind: 'add' | 'del' | 'same'; text: string }
  | { kind: 'skip'; count: number }

const maxLines = 2500

export function diffLines(before: string, after: string): DiffRow[] | 'too_large' {
  const left = split(before)
  const right = split(after)
  if (left.length > maxLines || right.length > maxLines) return 'too_large'
  return collapse(lcs(left, right), 2)
}

function split(text: string): string[] {
  if (text.length === 0) return []
  const lines = text.split('\n')
  if (lines[lines.length - 1] === '') lines.pop()
  return lines
}

function lcs(left: string[], right: string[]): DiffRow[] {
  const n = left.length
  const m = right.length
  const dp: number[][] = Array.from({ length: n + 1 }, () => Array<number>(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i -= 1) {
    const row = dp[i]
    const next = dp[i + 1]
    if (!row || !next) continue
    for (let j = m - 1; j >= 0; j -= 1) {
      const below = next[j] ?? 0
      const rightScore = row[j + 1] ?? 0
      row[j] = left[i] === right[j] ? (next[j + 1] ?? 0) + 1 : Math.max(below, rightScore)
    }
  }
  const out: DiffRow[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (left[i] === right[j]) {
      out.push({ kind: 'same', text: left[i] ?? '' })
      i += 1
      j += 1
      continue
    }
    const down = dp[i + 1]?.[j] ?? 0
    const across = dp[i]?.[j + 1] ?? 0
    if (down >= across) {
      out.push({ kind: 'del', text: left[i] ?? '' })
      i += 1
    } else {
      out.push({ kind: 'add', text: right[j] ?? '' })
      j += 1
    }
  }
  while (i < n) {
    out.push({ kind: 'del', text: left[i] ?? '' })
    i += 1
  }
  while (j < m) {
    out.push({ kind: 'add', text: right[j] ?? '' })
    j += 1
  }
  return out
}

function collapse(rows: DiffRow[], radius: number): DiffRow[] {
  const keep = rows.map((row) => row.kind !== 'same')
  for (let i = 0; i < rows.length; i += 1) {
    if (!keep[i]) continue
    for (let j = Math.max(0, i - radius); j <= Math.min(rows.length - 1, i + radius); j += 1) keep[j] = true
  }
  const out: DiffRow[] = []
  let skipped = 0
  for (let i = 0; i < rows.length; i += 1) {
    const row = rows[i]
    if (!row) continue
    if (!keep[i]) {
      skipped += 1
      continue
    }
    if (skipped > 0) {
      out.push({ kind: 'skip', count: skipped })
      skipped = 0
    }
    out.push(row)
  }
  if (skipped > 0) out.push({ kind: 'skip', count: skipped })
  return out
}
