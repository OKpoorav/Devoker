import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In dev, /api goes to the Go server. In Kubernetes the Ingress does the same routing.
export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': 'http://localhost:8080' } },
})
