import { createRouter, createWebHistory } from 'vue-router'
import EmptyView from '@/views/EmptyView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'home', component: EmptyView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

export default router
