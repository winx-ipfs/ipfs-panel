import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Dev proxy target matches the manager default (MANAGER_ADDR unset).
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080' },
      '/health': { target: 'http://127.0.0.1:8080' },
    },
  },
})
