import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/mail/',
  server: {
    proxy: {
      '/mail/api': {
        target: 'http://localhost:8081',
        rewrite: (path) => path.replace(/^\/mail\/api/, '/api')
      }
    }
  }
})
