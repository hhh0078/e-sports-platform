/** 文件作用：配置 uni-app 的 H5 与微信小程序构建。 */
import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

/** 兼容官方插件 CommonJS 在 ESM 项目下的 default 导出层级。 */
const uniPlugin = typeof uni === 'function' ? uni : (uni as unknown as { default: typeof uni }).default
export default defineConfig({
  plugins: [uniPlugin()],
  server: { host: '0.0.0.0', port: 5173, strictPort: true },
})
