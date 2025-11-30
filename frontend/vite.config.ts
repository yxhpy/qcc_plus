import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import compression from 'vite-plugin-compression'

// Vite build optimizations:
// - manualChunks to split heavy deps (vendor, charts, ui) for faster first load
// - dual gzip + brotli precompression to reduce transfer size
// - modern build target & css splitting
export default defineConfig({
  plugins: [
    react(),
    // Generate .gz assets for most CDNs / proxies
    compression({
      algorithm: 'gzip',
      threshold: 1024,
      ext: '.gz',
    }),
    // Generate .br assets for capable clients
    compression({
      algorithm: 'brotliCompress',
      threshold: 1024,
      ext: '.br',
    }),
  ],
  base: '/',
  build: {
    target: 'es2020',
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
    cssCodeSplit: true,
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        // Simplified chunk strategy to avoid React duplication issues
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined

          // Bundle all React ecosystem together to prevent hook conflicts
          if (id.includes('react') || id.includes('@dnd-kit')) {
            return 'vendor'
          }

          // Separate chart.js (large independent library)
          if (id.includes('chart.js')) {
            return 'charts'
          }

          // Let Rollup auto-chunk the rest
          return undefined
        },
      },
    },
  },
})
