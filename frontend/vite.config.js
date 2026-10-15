import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In dev, /api goes to the FastAPI server. In containers nginx does the same (nginx.conf).
export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': 'http://localhost:8000' } },
})
