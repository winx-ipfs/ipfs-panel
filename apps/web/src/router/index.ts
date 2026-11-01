import { createRouter, createWebHistory } from 'vue-router'
import type { MessageSchema } from '@/locales/en'
import ComingSoon from '@/views/ComingSoon.vue'
import NodeView from '@/views/NodeView.vue'
import NotFoundView from '@/views/NotFoundView.vue'
import SettingsView from '@/views/SettingsView.vue'

declare module 'vue-router' {
  interface RouteMeta {
    titleKey?: keyof MessageSchema
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'node', component: NodeView, meta: { titleKey: 'nodePageTitle' } },
    { path: '/files', name: 'files', component: ComingSoon, meta: { titleKey: 'comingFilesTitle' } },
    { path: '/pins', name: 'pins', component: ComingSoon, meta: { titleKey: 'comingPinsTitle' } },
    { path: '/scenes', name: 'scenes', component: ComingSoon, meta: { titleKey: 'comingScenesTitle' } },
    { path: '/settings', name: 'settings', component: SettingsView, meta: { titleKey: 'settingsTitle' } },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView, meta: { titleKey: 'notFoundTitle' } },
  ],
})

export default router
