/** 文件作用：测试中转译真实 TS/Vue 脚本并注入实际业务 API，保留可控制的网络与生命周期边界。 */
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { newWalletRequestID } from './wallet'
/** 测试中等待明确返回的真实业务请求。 */
export interface DeferredRequest { path: string; method: string; body: Record<string, unknown>; resolve: (value: unknown) => void; reject: (error?: unknown) => void; taken: boolean }
/** 测试中等待原生图片选择完成的上传。 */
export interface DeferredUpload { purpose: string; options: object | undefined; resolve: (value: string) => void; reject: (error?: unknown) => void }
/** 页面实际注册的生命周期回调。 */
export interface PageLifecycle { load: (options: Record<string, string>) => void; show: () => void; hide: () => void; unload: () => void }
/** 移除模块声明并转译 TypeScript，VM 仍执行真实生产函数。 */
export function executable(source: string): string { const plain = source.replace(/\r\n/g, '\n').replace(/^[ \t]*import[\s\S]*?from\s*['"][^'"]+['"];?\s*$/gm, '').replace(/^export\s+/gm, ''); return ts.transpileModule(plain, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None } }).outputText }
/** 从真实 Vue 文件中提取带语言标记的脚本。 */
export function pageScript(path: string): string { const source = readFileSync(new URL('../pages/' + path, import.meta.url), 'utf8'); const match = source.match(/<script\s+setup(?:\s+lang="ts")?\s*>([\s\S]*?)<\/script>/); if (!match) throw new Error('页面脚本不存在：' + path); return match[1] }
/** 注入所有实际 API 的具名方法，传输边界转到可控 api 函数。 */
export function injectBusinessAPIs(context: vm.Context): void {
  context.isTimelinePreview ??= () => false
  context.enablePublicShareMenu ??= () => {}
  context.onShow ??= () => {}
  context.onShareAppMessage ??= () => {}
  context.onShareTimeline ??= () => {}
  vm.runInContext(executable(readFileSync(new URL('./use-public-share.ts', import.meta.url), 'utf8')), context)
  context.requestPublic = (path: string) => context.api(path)
  context.request = context.api
  context.newWalletRequestID = newWalletRequestID
  context.queryString = (query: object = {}) => { const entries = Object.entries(query).filter(([, value]) => value !== undefined && value !== null && value !== ''); return entries.length ? '?' + entries.map(([key, value]) => encodeURIComponent(key) + '=' + encodeURIComponent(String(value))).join('&') : '' }
  context.errorMessage = (error: unknown) => typeof error === 'object' && error !== null ? 'message' in error ? String(error.message) : 'errMsg' in error ? String(error.errMsg) : '操作失败，请重试' : String(error || '操作失败，请重试')
  context.errorStatus = (error: unknown) => typeof error === 'object' && error !== null && 'status' in error ? error.status : undefined
  context.isCancelled = (error: unknown) => typeof error === 'object' && error !== null && 'cancelled' in error && error.cancelled === true
  for (const name of ['auth', 'games', 'services', 'users', 'orders', 'clubs', 'admin', 'wallet', 'messages', 'ai', 'finance']) vm.runInContext(executable(readFileSync(new URL('../api/' + name + '.api.ts', import.meta.url), 'utf8')), context)
  vm.runInContext(executable(readFileSync(new URL('./finance.ts', import.meta.url), 'utf8')), context)
  vm.runInContext('const revokeSession = signOut', context)
}
