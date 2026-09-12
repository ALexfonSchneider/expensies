import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv } from 'vite';

// In development /api is proxied to the Go server. When the dev server runs
// inside WSL the Windows host is not "localhost": set VITE_API_TARGET to
// http://<windows-host-ip>:8080 (see README.md).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', 'VITE_');
  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': {
          target: env.VITE_API_TARGET || 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  };
});
