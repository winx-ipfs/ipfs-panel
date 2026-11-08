# Design system

**English** · [中文](DESIGN-SYSTEM.zh-CN.md)

This is the visual foundation for **ipfs-panel**. The product name stays ipfs-panel. It sits under the WinX brand. **winx.run** is the front door, not a second product. The node sentences and the manager actions still come from [Node lifecycle](DESIGN-node-lifecycle.md). This file owns the chrome around them.

## Brand

Calm, plain, and a little green. The references are 1Panel, the Vercel dashboard, Linear, Raycast, and the Tailscale admin: lots of quiet surface, one accent, status you can read without a legend. The people using the first screens include beginners, so the page stays sparse. Density can come later, in expert mode, and not on the home card.

The mark is the WinX hexagon from [winx-ipfs/Mirror-Web](https://github.com/winx-ipfs/Mirror-Web) `src/assets/winx.svg`, `winx-light.svg`, and `winx-dark.svg`. The in-app mark keeps that geometry and uses `currentColor`, with a heavier stroke so it still reads at 28px. The favicon is a simplified hexagon on a dark tile. The full drawing turns to noise at 16px.

The brand green is `#20ffa9`. On a dark surface it is the primary button and the mark. On white it is about 1.3:1, so it is not used for text or buttons in the light theme. Light theme primary is `#0c7048`.

## What we did not adopt

The components are in-house. CSS variables hold the tokens. Vue components in `apps/web/src/components` are thin.

Naive UI and Element Plus were set aside. They ship a second visual language, and a dense admin look, which is the opposite of this page. Restyling them would cost more than drawing the small set we need.

reka-ui (Radix Vue) was the serious alternative. It would have given a focus trap and a listbox. The set here is small enough to own. The select is a native `<select>`, so a phone gets the system picker and a Chinese IME is not our problem. The dialog's focus trap is local. A headless kit stays the escape hatch if a later slice needs a real combobox. It is not a dependency today.

[Contributing](../CONTRIBUTING.md) asks for a yes before a new framework. This change does not add one.

Icons are [Lucide](https://lucide.dev) via `@lucide/vue`. Import one icon at a time so the bundle can drop the rest. Default size is 18px, stroke width 1.75. Decorative icons are `aria-hidden`. If an icon is the only content of a button, the button has an accessible name.

Runtime font requests do not go to a CDN. Latin text uses Inter, self-hosted from `@fontsource/inter` (latin 400, 500, 600, woff2). Chinese comes from the machine:

`Inter, Segoe UI, PingFang SC, Hiragino Sans GB, Noto Sans SC, Noto Sans CJK SC, Source Han Sans SC, Microsoft YaHei, WenQuanYi Zen Hei, WenQuanYi Micro Hei, sans-serif`

Mono, for the peer id and the folder path:

`ui-monospace, SFMono-Regular, Cascadia Mono, Sarasa Mono SC, Noto Sans Mono CJK SC, PingFang SC, Microsoft YaHei, monospace`

## Color

Tokens live in `apps/web/src/styles/tokens.css` on `:root`, and again under `:root[data-theme="dark"]`. Names are semantic. Do not use a raw hex in a component.

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `--bg` | `#e4ece7` | `#0c1210` | Page canvas |
| `--surface` | `#ffffff` | `#161d1a` | Cards, sidebar, dialogs |
| `--surface-sunken` | `#f2f6f3` | `#101714` | Wells, details, badges |
| `--border` | `#d5e0d9` | `#2c3b34` | Hairlines |
| `--border-strong` | `#b7c6bd` | `#3e5248` | Inputs, quiet buttons |
| `--text` | `#13241c` | `#e7f3ec` | Body |
| `--muted` | `#3e5248` | `#a8bdb3` | Secondary text |
| `--primary` | `#0c7048` | `#20ffa9` | The one accent, and primary buttons |
| `--primary-ink` | `#ffffff` | `#05281a` | Text on a primary button |
| `--primary-soft` | `#e7f6ee` | `#143528` | Selected nav, soft fills |
| `--success` / `--success-soft` | `#0c7048` / `#e7f6ee` | `#3ee6a4` / `#143528` | Running |
| `--warning` / `--warning-soft` | `#8a5a00` / `#fff4d6` | `#f5c16c` / `#3a2c12` | Stopping, unreachable |
| `--danger` / `--danger-soft` | `#b42318` / `#fdeceb` | `#ffb4ab` / `#3a1c1a` | Needs attention |
| `--info` / `--info-soft` | `#0f5c86` / `#e7f2f8` | `#9fd4f5` / `#122833` | Starting |
| `--neutral` / `--neutral-soft` | `#3e5248` / `#e7eeea` | `#d5e4dc` / `#24302b` | Stopped |
| `--focus` | `#0c7048` | `#20ffa9` | Focus ring |

Text and muted text are at least 4.5:1 on `--bg`, `--surface`, and the soft fills they sit on. Primary button ink is at least 4.5:1 on `--primary`. The status pill uses the same rule against its soft fill.

Node state colors, always with a word, never color alone:

| Situation | Tone |
| --- | --- |
| `down` | neutral |
| `starting` | info, dot pulses |
| `up` | success |
| `stopping` | warning, dot pulses |
| `failed` | danger |
| manager unreachable | warning |

Stopped is not an error. Stopping is not a failure. The primary button stays the accent. It is not red. Red means the node needs attention.

## Type, space, radius, shadow, motion

| Token | Value |
| --- | --- |
| `--text-xs` | 0.75rem |
| `--text-sm` | 0.875rem |
| `--text-md` | 1rem |
| `--text-lg` | 1.125rem |
| `--text-xl` | 1.375rem |
| `--text-2xl` | 1.75rem |
| `--leading-body` | 1.6 |
| `--space-1` … `--space-12` | 4, 8, 12, 16, 20, 24, 32, 40, 48 |
| `--radius-sm` / `md` / `lg` | 8 / 12 / 16 |
| `--radius-full` | 999px |
| `--shadow-sm` | a 1px lift |
| `--shadow-md` | lift plus a soft drop, so a white card separates from the canvas |
| `--duration-fast` / `--duration` | 120ms / 180ms |
| `--ease` | `cubic-bezier(0.2, 0, 0, 1)` |

Latin headings use a slight negative tracking. Under `:lang(zh)` tracking is 0. Do not tighten Chinese.

`prefers-reduced-motion: reduce` cuts animation and transition time to nearly nothing. The pulsing status dot and the skeleton shimmer stop. Do not add a second motion switch. The operating system already has one.

## Components

All of them are in `apps/web/src/components`. Prefer these over a new one-off button.

| Component | Use |
| --- | --- |
| `Button` | `primary` is the one obvious action. `quiet` is the second action (Restart). `danger` is rare and means something hard to undo. `ghost` is for low-emphasis actions in a bar. |
| `IconButton` | Needs a `label`. The label is the accessible name. Pair it with `Tooltip` when the name also helps sighted people. |
| `Card` | One subject. `tone` paints a 3px status bar. `none` is a plain card. |
| `StatusPill` | A word plus a dot. Tones match the color table. `pulse` only for starting and stopping. |
| `Input` | Label above, hint or error below. Wire `aria-invalid` through the `error` prop. Setup, login, and change-password use it. The node page still must not grow a path field. |
| `Select` | Native select. Two or more options that should use the platform picker. Language on the Settings page. The top bar keeps a two-way segment for English / 中文, because two choices do not need a menu. |
| `Switch` | A boolean the person can flip immediately. Theme "match this device" is the example. |
| `Tabs` | Real panels, not a radio by another name. Arrow keys move the tab. Settings uses General, Account, and About. |
| `Dialog` | `dialog` for a short decision. `drawer` for the narrow-screen nav. Escape closes. Focus returns to the opener. The title is required even when it is visually hidden. |
| `Toast` | Short confirmation, `aria-live="polite"`. "Copied" is the one we show. |
| `Tooltip` | Extra name on hover and focus. Do not hide the only copy of a required sentence in a tooltip. |
| `Skeleton` | The shape of the card while the first overview is in flight. Not a spinner on a blank page. |
| `EmptyState` | A title, one or two sentences, one next step. 404 and the placeholder sections. |
| `PageHeader` | One `h1` per page, plus a description that says what the page is for. |
| `Table` | Rows of facts or a short record. Put a caption on it. `plain` is for a card. The default is for a record like the About table. Stack the plain table on narrow screens so a peer id is not squeezed into a third column. |
| `BrandMark` | The adapted WinX mark. Do not recolor it outside `--mark`. |

Rules:

- One primary button in a group. Restart stays quiet, and only while the node is up.
- Do not invent a sixth node state. Restart is stop, then start.
- Do not show a Kubo RPC address, or port 5001. The files page may copy a local gateway link. The label says it only works on this computer.
- Placeholder sections say they are not built yet, in both languages, and offer a way back to the node. They do not render fake files or fake pins.

## Empty, loading, error

| Moment | What the page does |
| --- | --- |
| First load, no snapshot yet | Skeleton. The status text is "Loading the node status." It is not "The node is stopped." |
| `starting` or `stopping` | The sentence, a hint that the page updates on its own, no manager buttons. |
| `failed` | The translated sentence for `error.code`. `error.detail`, when present, sits in a preformatted well labeled Details. It is not the sentence. |
| Manager request failed | The unreachable sentence, the last snapshot kept, Retry as the only manager action. |
| Section not built | Empty state, Soon in the nav, no disabled item that cannot be opened. |
| Unknown URL | 404 with one action back to the node. The router does not silently redirect. |

## Accessibility

- Body text, muted text, and button ink meet WCAG AA (4.5:1). The focus ring is 2px, offset 2px, and uses `:focus-visible` so a mouse click does not leave a ring.
- The first control is "Skip to content".
- The narrow-screen nav is a dialog: labelled, modal, Escape to close, focus trapped, focus restored.
- Tabs use `tab`, `tablist`, and `tabpanel`. The switch uses `role="switch"` and `aria-checked`.
- Status is a sentence in a `role="status"` element, plus the pill. The pill is not the only carrier.
- `html lang` is `en` or `zh-Hans`, from the locale store.
- Hit targets for actions and icon buttons are at least 40px. Primary actions on the node card are 44px tall.

## Layout and the two languages

- A new string lands in `apps/web/src/locales/en.ts` and `zh.ts` in the same change. `zh` is Simplified Chinese.
- Do not ellipsize a sentence, a button label, a peer id, a path, or an error. Wrapping is the layout. A long detail block may scroll inside its well. The text is still in the page.
- Folder paths get a break opportunity after each `/`. Peer ids have no breaks in them, so they wrap anywhere.
- Buttons may grow and wrap onto a second line. Do not fix their width in `ch` of the English word. Chinese labels are often shorter. English hints are often longer. The column is `min(760px, 100%)` so neither language has a line that runs the full desktop.
- The sidebar labels wrap instead of clipping. Scenes sits under "Coming soon" / "即将推出", at lower opacity, with `aria-disabled`. It has no badge. Files and Pins are normal items.

## Shell

- Sidebar from 880px up. Below that, a menu button opens the drawer.
- Top bar: English / 中文, and a theme button. The button cycles light, dark, and match-device. Its icon and, from 720px up, its label show the current mode. The accessible name and the tooltip say the same mode. Both choices persist in `localStorage` (`ipfs-panel.lang`, `ipfs-panel.theme`). The theme is applied from a small script in `index.html` before paint, so the first frame is not the wrong theme.
- From 1200px the node page is two columns. The status card is on the left. On the right, while the node is up, a Connection card shows the version and a shortened peer id. The copy button copies the full peer id. The overview does not include a gateway address or an API address, so the card does not invent them. Otherwise the right card explains what a node is and links to the IPFS docs.
- Nav: Node, Files, Pins, then Later (Scenes), then Settings. Files and Pins are real pages. Scenes stays under "Coming soon". Expert mode is not a nav item.
- `winx.run` sits in the sidebar footer. It opens in a new tab. It is the portal, labeled as such.

Dev-only, the node page reads `?mock=` when `import.meta.env.DEV` is true: `down`, `starting`, `up`, `stopping`, `failed`, or `unreachable`. The production bundle does not include those fixtures. The Vite proxy to the manager is unchanged.

## Screenshots

From `apps/web`, after `npx playwright install chromium`:

```bash
npm run shots
```

That writes the node page in all five states, the files page (list, empty, uploading, delete dialog, node stopped), the pins page (list, add dialog with a validation error, a job in progress, empty), the setup screen, the login screen with an error, and the settings password form, light and dark, English and 中文, at 1440 and 390, plus the 404 page, to `SHOTS_DIR` or `/opt/cursor/artifacts`. The script starts Vite itself. It does not need a running manager. Auth screens use a dev-only `?auth=` fixture. The files page uses `?files=`. The pins page uses `?pins=`.

This is not a CI job. A Chromium install and the machine's CJK fonts make a pixel run slow and easy to flake, and `ci-ok` should stay the fast check. The unit tests still cover the sentences, the manager actions, and the form controls. Re-run `npm run shots` when the shell or the node card changes, and look at the pictures.

![Needs attention, dark theme, English](screenshots/node-failed-dark-en.png)

![Unknown page, light theme, English](screenshots/not-found-light-en.png)
