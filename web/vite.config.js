import { defineConfig, loadEnv } from 'vite';
import vue from '@vitejs/plugin-vue';
import svgLoader from 'vite-svg-loader';
import { fileURLToPath, URL } from 'node:url';
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd());
  return {
    base: env.VITE_BASE_URL || './',
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
        '~': fileURLToPath(new URL('.', import.meta.url)),
      },
    },
    plugins: [vue(), svgLoader()],
    css: { preprocessorOptions: { less: { javascriptEnabled: false } } },
    build: { outDir: 'dist', emptyOutDir: true, cssCodeSplit: false, sourcemap: false, target: 'es2022' },
    server: {
      host: '127.0.0.1',
      port: 3001,
      proxy: { '/api': { target: env.HOMEWAF_BACKEND || 'http://127.0.0.1:26666', changeOrigin: true, ws: true } },
    },
  };
});
