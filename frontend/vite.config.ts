import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    port: 5173,
    host: true,
    // In dev the Go backend runs separately; proxying keeps the app on one
    // origin so the auth cookie and the WebSocket both work.
    proxy: {
      '/api': {
        target: process.env.VITE_BACKEND_URL ?? 'http://localhost:8080',
        changeOrigin: false,
        ws: true,
      },
    },
  },
});
