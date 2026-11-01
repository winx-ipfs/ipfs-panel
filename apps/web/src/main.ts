import { createApp } from 'vue'
import { createPinia } from 'pinia'

import '@fontsource/inter/latin-400.css'
import '@fontsource/inter/latin-500.css'
import '@fontsource/inter/latin-600.css'

import App from './App.vue'
import { i18n } from './i18n'
import router from './router'
import { useLocaleStore } from './stores/locale'
import { useThemeStore } from './stores/theme'
import './styles/tokens.css'
import './styles/base.css'
import './styles/components.css'
import './styles/shell.css'
import './styles/auth.css'
import './styles/files.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)

useLocaleStore().init()
useThemeStore().init()

app.mount('#app')
