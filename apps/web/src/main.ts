import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import { i18n } from './i18n'
import router from './router'
import { useLocaleStore } from './stores/locale'
import './styles/base.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)

useLocaleStore().init()

app.mount('#app')
