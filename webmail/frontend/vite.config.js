import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
export default defineConfig({
    plugins: [react()],
    base: '/mail/',
    server: {
        proxy: {
            '/mail/api': {
                target: 'http://localhost:8082',
                rewrite: function (path) { return path.replace(/^\/mail\/api/, '/api'); }
            }
        }
    }
});
