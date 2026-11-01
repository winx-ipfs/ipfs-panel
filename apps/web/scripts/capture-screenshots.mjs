import { spawn } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import { createServer } from 'node:net'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from '@playwright/test'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const outDir = process.env.SHOTS_DIR ?? '/opt/cursor/artifacts'
const states = ['down', 'starting', 'up', 'stopping', 'failed']
const themes = ['light', 'dark']
const locales = ['en', 'zh']
const widths = [
  { name: '1440', width: 1440, height: 900 },
  { name: '390', width: 390, height: 844 },
]

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

async function shoot(page, file) {
  await page.waitForSelector('main')
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(150)
  await page.screenshot({ path: file, fullPage: true })
}

const port = await freePort()
const origin = `http://127.0.0.1:${port}`
const server = spawn('npx', ['vite', '--host', '127.0.0.1', '--port', String(port), '--strictPort'], {
  cwd: root,
  stdio: ['ignore', 'pipe', 'pipe'],
})
let logs = ''
server.stdout.on('data', (chunk) => {
  logs += chunk
})
server.stderr.on('data', (chunk) => {
  logs += chunk
})

try {
  await waitForServer(origin, server)
  await mkdir(outDir, { recursive: true })
  const browser = await chromium.launch()
  const index = []
  try {
    for (const theme of themes) {
      for (const locale of locales) {
        for (const size of widths) {
          const context = await browser.newContext({
            viewport: { width: size.width, height: size.height },
            deviceScaleFactor: 1,
            colorScheme: theme,
            reducedMotion: 'reduce',
          })
          await context.addInitScript(
            ({ themePreference, localePreference }) => {
              localStorage.setItem('ipfs-panel.theme', themePreference)
              localStorage.setItem('ipfs-panel.lang', localePreference)
            },
            { themePreference: theme, localePreference: locale },
          )
          const page = await context.newPage()
          for (const state of states) {
            await page.goto(`${origin}/?mock=${state}`, { waitUntil: 'domcontentloaded' })
            await page.waitForSelector('[role="status"]')
            const file = path.join(outDir, `node-${state}-${theme}-${locale}-${size.name}.png`)
            await shoot(page, file)
            index.push(file)
          }
          await page.goto(`${origin}/missing-page`, { waitUntil: 'domcontentloaded' })
          const missing = path.join(outDir, `not-found-${theme}-${locale}-${size.name}.png`)
          await shoot(page, missing)
          index.push(missing)
          await context.close()
        }
      }
    }
    const extra = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      reducedMotion: 'reduce',
    })
    await extra.addInitScript(() => {
      localStorage.setItem('ipfs-panel.theme', 'light')
      localStorage.setItem('ipfs-panel.lang', 'en')
    })
    const settings = await extra.newPage()
    await settings.goto(`${origin}/settings`, { waitUntil: 'domcontentloaded' })
    await settings.evaluate(() => document.fonts.ready)
    const settingsFile = path.join(outDir, 'settings-light-en-1440.png')
    await settings.screenshot({ path: settingsFile, fullPage: true })
    index.push(settingsFile)
    await settings.getByRole('tab', { name: 'About' }).click()
    const aboutFile = path.join(outDir, 'settings-about-light-en-1440.png')
    await settings.screenshot({ path: aboutFile, fullPage: true })
    index.push(aboutFile)
    await extra.close()

    const phone = await browser.newContext({
      viewport: { width: 390, height: 844 },
      reducedMotion: 'reduce',
    })
    await phone.addInitScript(() => {
      localStorage.setItem('ipfs-panel.theme', 'dark')
      localStorage.setItem('ipfs-panel.lang', 'zh')
    })
    const drawerPage = await phone.newPage()
    await drawerPage.goto(`${origin}/?mock=down`, { waitUntil: 'domcontentloaded' })
    await drawerPage.getByRole('button', { name: '打开菜单' }).click()
    await drawerPage.waitForSelector('[role="dialog"]')
    await drawerPage.evaluate(() => document.fonts.ready)
    const drawerFile = path.join(outDir, 'nav-drawer-dark-zh-390.png')
    await drawerPage.screenshot({ path: drawerFile })
    index.push(drawerFile)
    await phone.close()

    const tablet = await browser.newContext({
      viewport: { width: 768, height: 1024 },
      reducedMotion: 'reduce',
      colorScheme: 'light',
    })
    await tablet.addInitScript(() => {
      localStorage.setItem('ipfs-panel.theme', 'light')
      localStorage.setItem('ipfs-panel.lang', 'zh')
    })
    const tabletPage = await tablet.newPage()
    await tabletPage.goto(`${origin}/?mock=down`, { waitUntil: 'domcontentloaded' })
    await tabletPage.getByRole('button', { name: '打开菜单' }).click()
    await tabletPage.waitForSelector('[role="dialog"]')
    await tabletPage.evaluate(() => document.fonts.ready)
    await tabletPage.waitForTimeout(150)
    const tabletFile = path.join(outDir, 'nav-drawer-light-zh-768.png')
    await tabletPage.screenshot({ path: tabletFile })
    index.push(tabletFile)
    await tablet.close()
  } finally {
    await browser.close()
  }
  await writeFile(path.join(outDir, 'screenshot-index.txt'), `${index.join('\n')}\n`)
  console.log(`wrote ${index.length} screenshots to ${outDir}`)
} catch (error) {
  console.error(logs)
  console.error(error)
  process.exitCode = 1
} finally {
  server.kill('SIGTERM')
  setTimeout(() => {
    server.kill('SIGKILL')
    process.exit(process.exitCode ?? 0)
  }, 300).unref()
}
