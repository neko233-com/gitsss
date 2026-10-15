import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';

// Separate from the Vue bundle so the existing Git UI build stays unchanged.
export default defineConfig({
  plugins: [react()],
  publicDir: false,
  build: {
    outDir: 'public/assets',
    emptyOutDir: false,
    sourcemap: false,
    rollupOptions: {
      input: 'web_src/js/react/dashboard.tsx',
      output: {
        entryFileNames: 'js/gitsss-react-dashboard.js',
        format: 'iife',
      },
    },
  },
});
