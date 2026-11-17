#!/usr/bin/env node
// Fails when a row menu is rendered but its items cannot be clicked.
//
// Every list row keeps its actions in an absolutely positioned menu inside the
// actions cell. That cell also sets overflow: hidden so long names and CIDs get
// an ellipsis, and the menu was clipped away entirely: the items existed in the
// DOM, were announced by a screen reader, and no pointer could reach them.
//
// This checks the only thing that matters — does a pointer hit each item — at
// both a desktop and a phone width.
import { spawn } from 'node:child_process'
import { existsSync, readdirSync } from 'node:fs'
import { createServer } from 'node:net'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from '@playwright/test'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

function freePort() {
  return new Promise((resolve, reject) => {
    const server = createServer()
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      if (!address || typeof address === 'string') {
        server.close()
        reject(new Error('no port'))
        return
      }
      const { port } = address
      server.close((error) => (error ? reject(error) : resolve(port)))
    })
  })
}

function waitForServer(url, child) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      child.kill('SIGTERM')
      reject(new Error(`dev server did not answer ${url}`))
    }, 30000)
    const tick = async () => {
      try {
        const response = await fetch(url)
        if (response.ok || response.status === 404) {
          clearTimeout(timeout)
          resolve()
          return
        }
      } catch {
        // The server is still starting.
      }
      setTimeout(tick, 200)
    }
    tick()
  })
}

// Playwright's headless shell is what CI installs. A developer machine may only
// have the full Chromium build, so fall back to whichever is present.
function findChromium() {
  const base = process.env.LOCALAPPDATA
    ? path.join(process.env.LOCALAPPDATA, 'ms-playwright')
    : path.join(process.env.HOME ?? '', '.cache', 'ms-playwright')
  let entries
  try {
    entries = readdirSync(base)
  } catch {
    return ''
  }
  for (const entry of entries.filter((name) => name.startsWith('chromium-')).sort().reverse()) {
    for (const candidate of [
      path.join(base, entry, 'chrome-win64', 'chrome.exe'),
      path.join(base, entry, 'chrome-win', 'chrome.exe'),
      path.join(base, entry, 'chrome-linux', 'chrome'),
      path.join(base, entry, 'chrome-mac', 'Chromium.app', 'Contents', 'MacOS', 'Chromium'),
    ]) {
      if (existsSync(candidate)) return candidate
    }
  }
  return ''
}

const targets = [
  {
    name: 'files',
    path: '/files?files=list&auth=session',
    table: 'table.files-table',
    menu: '.files-menu',
  },
  {
    name: 'pins',
    path: '/pins?pins=list&auth=session',
    table: 'table.pins-table',
    menu: '.pins-menu',
  },
]

const port = await freePort()
const origin = `http://127.0.0.1:${port}`
const server = spawn(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['vite', '--host', '127.0.0.1', '--port', String(port), '--strictPort'],
  { cwd: root, stdio: ['ignore', 'pipe', 'pipe'], shell: process.platform === 'win32' },
)
let logs = ''
server.stdout.on('data', (chunk) => {
  logs += chunk
})
server.stderr.on('data', (chunk) => {
  logs += chunk
})

const failures = []
let checked = 0

try {
  await waitForServer(origin, server)

  let browser
  try {
    browser = await chromium.launch()
  } catch (error) {
    const fallback = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE || findChromium()
    if (!fallback) throw error
    browser = await chromium.launch({ executablePath: fallback })
  }

  try {
    for (const target of targets) {
      for (const width of [1440, 390]) {
        const context = await browser.newContext({
          viewport: { width, height: width === 390 ? 844 : 900 },
          reducedMotion: 'reduce',
        })
        await context.addInitScript(() => {
          localStorage.setItem('ipfs-panel.lang', 'zh')
          localStorage.setItem('ipfs-panel.theme', 'light')
        })
        const page = await context.newPage()
        await page.goto(`${origin}${target.path}`, { waitUntil: 'domcontentloaded' })
        await page.waitForSelector(`${target.table} tbody tr`, { timeout: 20000 })
        const triggers = page.getByRole('button', { name: /操作|Actions/ })
        const rows = await triggers.count()
        if (rows === 0) {
          failures.push(`${target.name} @${width}px: no row actions found`)
          await context.close()
          continue
        }
        // Every row, not just the first: a menu that opens below the fold or
        // past the scroll container is unusable for the rows a user sees last.
        for (let row = 0; row < rows; row += 1) {
          await triggers.nth(row).click()
          await page.waitForSelector(target.menu, { state: 'attached', timeout: 5000 })
          const result = await page.evaluate((menuSelector) => {
            const menu = document.querySelector(menuSelector)
            if (!menu) return { items: 0, reachable: 0, clippedBy: [], offscreen: false }
            const items = [...menu.querySelectorAll('a,button,[role="menuitem"]')]
            const reachable = items.filter((item) => {
              const box = item.getBoundingClientRect()
              if (box.width === 0 || box.height === 0) return false
              const hit = document.elementFromPoint(
                box.left + box.width / 2,
                box.top + box.height / 2,
              )
              return hit === item || item.contains(hit)
            })
            const clippedBy = []
            let node = menu.parentElement
            while (node && node !== document.body) {
              const style = getComputedStyle(node)
              if (style.overflow !== 'visible') {
                const cls = String(node.className).split(' ')[0] || ''
                clippedBy.push(`${node.tagName.toLowerCase()}${cls ? '.' + cls : ''}:${style.overflow}`)
              }
              node = node.parentElement
            }
            const box = menu.getBoundingClientRect()
            return {
              items: items.length,
              reachable: reachable.length,
              clippedBy,
              offscreen: box.top < 0 || box.bottom > window.innerHeight,
            }
          }, target.menu)

          const label = `${target.name} @${width}px row ${row + 1}/${rows}`
          checked += 1
          if (result.items === 0) {
            failures.push(`${label}: the menu opened but contained no items`)
          } else if (result.reachable !== result.items) {
            failures.push(
              `${label}: ${result.reachable}/${result.items} menu items reachable by pointer` +
                (result.clippedBy.length ? ` (clipped by ${result.clippedBy.join(', ')})` : '') +
                (result.offscreen ? ' (menu drawn outside the viewport)' : ''),
            )
          } else if (result.offscreen) {
            failures.push(`${label}: every item reachable but the menu is drawn outside the viewport`)
          } else {
            console.log(`${label}: ${result.reachable}/${result.items} menu items reachable`)
          }
          await page.keyboard.press('Escape')
        }
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
} finally {
  server.kill('SIGTERM')
}

if (checked === 0) {
  console.error('row menu check: nothing was checked, which is a failure')
  process.exit(1)
}
if (failures.length) {
  console.error('\nrow menu check failed:')
  for (const failure of failures) console.error(`  ${failure}`)
  if (logs.trim()) console.error(`\ndev server output:\n${logs}`)
  process.exit(1)
}
console.log('\nrow menu check: every item is reachable by pointer')
// On Windows the dev server is spawned through a shell, where SIGTERM does not
// reach the child tree; exit explicitly so the check cannot hang CI.
process.exit(0)
