import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'http://localhost:8080/ws',
        ws: false,
      },
    },
    //allowedHosts: ['localhost','5d771ddb5d1e.ngrok-free.app', '127.0.0.1', '0.0.0.0'],
  },
})
