import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In dev, /api goes to the Go server. In containers nginx does the same (docker/nginx.conf).
export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': 'http://localhost:8080' } },
})
