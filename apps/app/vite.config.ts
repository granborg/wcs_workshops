import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Reachable from a phone on the same network during development.
    host: true,
    port: 5173,
    // Same-origin API during development, matching how the Go binary serves production.
    proxy: {
      '/api': 'http://localhost:3000',
    },
  },
})
