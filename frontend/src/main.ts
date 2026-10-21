/** 文件作用：创建兼容 H5 与微信小程序的 Vue 应用。 */
import { createSSRApp } from 'vue'
import App from './App.vue'
/** 创建 uni-app 所需的应用实例。 */
export function createApp() { return { app: createSSRApp(App) } }
