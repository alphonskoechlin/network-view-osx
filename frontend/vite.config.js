import { defineConfig } from 'vite'
import svelte from 'vite-plugin-svelte'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      '/discover': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
