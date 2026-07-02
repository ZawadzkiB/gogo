import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// base: './' so the built dist/ is path-agnostic -- server.py serves it from the
// site root, but relative asset URLs mean it would also work under any subpath.
export default defineConfig({
  base: './',
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
