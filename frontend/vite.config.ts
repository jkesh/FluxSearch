import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiPort = process.env.FLUXSEARCH_API_PORT || '8080'
const frontendPort = Number(process.env.FLUXSEARCH_FRONTEND_PORT || '5173')
const proxyTarget =
  process.env.FLUXSEARCH_VITE_API_PROXY || `http://127.0.0.1:${apiPort}`

export default defineConfig({
  plugins: [react()],
  server: {
    port: frontendPort,
    proxy: {
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
        ws: true,
      },
      '/healthz': {
        target: proxyTarget,
        changeOrigin: true,
      },
    },
  },
})
