import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  // Read PORT from the root .env so the dev proxy targets the correct backend port.
  const rootEnv = loadEnv(mode, new URL('..', import.meta.url).pathname, '')
  const backendPort = rootEnv.PORT || '8001'

  return {
    plugins: [react()],
    build: {
      // Warn when any chunk exceeds this size (kB, uncompressed). The current
      // baseline is ~152 kB gzip; raise this threshold only after profiling.
      chunkSizeWarningLimit: 300,
      rollupOptions: {
        output: {
          // Split markdown + syntax-highlighting into a separate vendor chunk
          // so the main app chunk stays lean on first load.
          // Function form required by rolldown (Vite 8).
          manualChunks(id) {
            if (id.includes('react-markdown') || id.includes('rehype-highlight') || id.includes('highlight.js')) {
              return 'vendor-markdown'
            }
          },
        },
      },
    },
    server: {
      proxy: {
        '/api': {
          target: `http://localhost:${backendPort}`,
          changeOrigin: true,
        },
      },
    },
    test: {
      environment: 'jsdom',
      globals: true,
      css: false,
      setupFiles: ['./src/test-setup.js'],
    },
  }
})
