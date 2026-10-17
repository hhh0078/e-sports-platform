/** 文件作用：配置 uni-app 的 H5 与微信小程序构建。 */
import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

export default defineConfig({
  plugins: [uni()],
  server: { host: '0.0.0.0', port: 5173, strictPort: true },
})
