import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // Load .env from the repo root so VITE_* vars and Go server vars live in one place.
  envDir: '..',
  server: {
    proxy: {
      '/api': 'http://localhost:8081',
    },
  },
})
