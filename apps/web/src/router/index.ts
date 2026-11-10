import { createRouter, createWebHistory } from 'vue-router'
import { decide, type AuthPhase } from '@/auth/guard'
import type { MessageSchema } from '@/locales/en'
import { onUnauthorized } from '@/lib/session'
import { useAuthStore } from '@/stores/auth'
import ExploreView from '@/views/ExploreView.vue'
import FilesView from '@/views/FilesView.vue'
import PeersView from '@/views/PeersView.vue'
import PinsView from '@/views/PinsView.vue'
import LoginView from '@/views/LoginView.vue'
import NodeView from '@/views/NodeView.vue'
import NotFoundView from '@/views/NotFoundView.vue'
import ScenesView from '@/views/ScenesView.vue'
import SettingsView from '@/views/SettingsView.vue'
import SetupView from '@/views/SetupView.vue'

declare module 'vue-router' {
  interface RouteMeta {
    titleKey?: keyof MessageSchema
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'node', component: NodeView, meta: { titleKey: 'nodePageTitle' } },
    { path: '/files', name: 'files', component: FilesView, meta: { titleKey: 'filesTitle' } },
    { path: '/explore', name: 'explore', component: ExploreView, meta: { titleKey: 'exploreTitle' } },
    { path: '/peers', name: 'peers', component: PeersView, meta: { titleKey: 'peersTitle' } },
    { path: '/pins', name: 'pins', component: PinsView, meta: { titleKey: 'pinsTitle' } },
    { path: '/scenes', name: 'scenes', component: ScenesView, meta: { titleKey: 'scenesTitle' } },
    { path: '/settings', name: 'settings', component: SettingsView, meta: { titleKey: 'settingsTitle' } },
    { path: '/login', name: 'login', component: LoginView, meta: { titleKey: 'signInTitle' } },
    { path: '/setup', name: 'setup', component: SetupView, meta: { titleKey: 'setupTitle' } },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView, meta: { titleKey: 'notFoundTitle' } },
  ],
})

function phaseOf(status: string): AuthPhase {
  if (status === 'setup' || status === 'authenticated') return status
  return 'anonymous'
}

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.bootstrap()
  const decision = decide(phaseOf(auth.status), {
    name: typeof to.name === 'string' ? to.name : '',
    fullPath: to.fullPath,
    redirect: to.query.redirect,
  })
  if (decision.type === 'allow') return true
  if (decision.type === 'path') return decision.path
  return { name: decision.name, query: decision.query }
})

onUnauthorized(() => {
  const auth = useAuthStore()
  auth.markSignedOut()
  const current = router.currentRoute.value
  if (current.name === 'login' || current.name === 'setup') return
  const redirect = current.fullPath
  void router.push({
    name: 'login',
    query: redirect && redirect !== '/' ? { redirect } : {},
  })
})

export default router
