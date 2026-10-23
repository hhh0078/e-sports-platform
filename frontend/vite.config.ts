/** 文件作用：配置 uni-app 的 H5 与微信小程序构建。 */
import { defineConfig, loadEnv } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'
import { resolveAPIBase } from './src/lib/api-environment'

/** 兼容官方插件 CommonJS 在 ESM 项目下的 default 导出层级。 */
const uniPlugin = typeof uni === 'function' ? uni : (uni as unknown as { default: typeof uni }).default
export default defineConfig(({ command, mode }) => {
  const production = command === 'build' && mode !== 'local-test'
  // 私人的 .env.local 只影响开发；正式构建不继承其中的本机地址。
  const configured = production ? process.env.VITE_API_BASE : loadEnv(mode, process.cwd(), 'VITE_').VITE_API_BASE
  const apiBase = resolveAPIBase(production, configured)
  return {
    define: { 'import.meta.env.VITE_API_BASE': JSON.stringify(apiBase) },
    plugins: [uniPlugin()],
    server: { host: '0.0.0.0', port: 5173, strictPort: true },
  }
})
