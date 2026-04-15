import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import fs from 'fs'
import yaml from 'js-yaml'

// 读取后端配置
const getBackendConfig = () => {
    try {
        const configPath = resolve(__dirname, '../config.yaml')
        const fileContents = fs.readFileSync(configPath, 'utf8')
        return yaml.load(fileContents) as any
    } catch (e) {
        console.warn('无法加载后端配置，使用默认值')
        return {}
    }
}

const backendConfig = getBackendConfig()
const apiPort = backendConfig?.server?.http_port || 8080
const wsPort = backendConfig?.server?.websocket_port || 8081

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [vue()],
    resolve: {
        alias: {
            '@': resolve(__dirname, 'src'),
        },
    },
    server: {
        port: 5173,
        proxy: {
            '/api': {
                target: `http://localhost:${apiPort}`,
                changeOrigin: true,
            },
            '/ws': {
                target: `ws://localhost:${wsPort}`,
                ws: true,
            },
        },
    },
})
