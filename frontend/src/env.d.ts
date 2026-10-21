/** 文件作用：声明 uni-app Vue 文件、微信图片接口和构建环境变量的类型边界。 */
/// <reference types="vite/client" />
declare module '*.vue' { import type { DefineComponent } from 'vue'; const component: DefineComponent; export default component }
declare module '@dcloudio/uni-ui/lib/uni-icons/uni-icons.vue' { import type { DefineComponent } from 'vue'; const component: DefineComponent; export default component }
