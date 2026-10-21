/** 文件作用：检查集结前端 TypeScript、接口分层、导航归属与设计图目录的强制开发约定。 */
import { readFileSync, readdirSync, existsSync } from 'node:fs'
import { resolve, relative, dirname, extname } from 'node:path'
import { fileURLToPath } from 'node:url'
import assert from 'node:assert/strict'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const sourceRoot = resolve(root, 'frontend/src')
const issues: string[] = []

/**
 * 递归收集目录中的普通文件，忽略不存在的可选目录。
 * @param directory - 需要扫描的绝对目录。
 * @returns 目录内所有普通文件的绝对路径。
 */
function filesIn(directory: string): string[] {
  if (!existsSync(directory)) return []
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const path = resolve(directory, entry.name)
    return entry.isDirectory() ? filesIn(path) : entry.isFile() ? [path] : []
  })
}

/**
 * 提取 Vue 中的业务脚本，避免设计说明或模板文字触发请求检测。
 * @param text - Vue 或 TypeScript 文件内容。
 * @param vue - 当前是否为 Vue 文件。
 * @returns 可用于规则扫描的业务源代码。
 */
function businessScript(text: string, vue: boolean): string {
  return vue ? [...text.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/g)].map(match => match[1]).join('\n') : text
}

/**
 * 识别绕过领域 API 的网络调用或传输层导入。
 * @param script - 待检查的业务脚本。
 * @returns 是否存在跨层直接请求。
 */
function bypassesApi(script: string): boolean {
  return /\buni\s*\.\s*(request|uploadFile)\s*\(/.test(script)
    || /import\s*\{[^}]*\b(?:request|upload|queryString)\b[^}]*\}\s*from\s*['"][^'"]*api\/client['"]/.test(script)
    || /import\s*\*\s+as\s+\w+\s+from\s*['"][^'"]*api\/client['"]/.test(script)
    || /from\s*['"][^'"]*lib\/api['"]/.test(script)
    || /\bapi\s*\(\s*['"`]/.test(script)
}

/** 检查二级框架仍提供普通顶栏返回、统一悬浮返回组件或调用共享返回函数的按钮。 */
function hasUnifiedReturn(text: string): boolean {
  return /<PageHeader\b[^>]*\sback(?:\s|\/|>)/.test(text)
    || /<(?:FloatingBack|BackButton)\b/.test(text)
    || (/(?:@click|@tap)=["']back(?:\(\))?["']/.test(text) && /import\s*\{[^}]*\bback\b[^}]*\}\s*from\s*["'][^"']*lib\/navigation["']/.test(businessScript(text, true)))
}

/**
 * 微信自定义组件仅使用类选择器，避免跨端构建成功但真机静默忽略样式。
 * @param text - Vue 源文件。
 * @returns 第一个不兼容的选择器或空值。
 */
function unsupportedComponentSelector(text: string): string | undefined {
  const styles = [...text.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style>/g)].map(match => match[1]).join('\n')
  return [...styles.matchAll(/([^{}]+)\{/g)].map(match => match[1].trim()).find(selector => !selector.startsWith('@') && (/(?:^|[\s,>+~(])(?:view|text|button|image|input|textarea|switch|scroll-view)(?=[\s.:#>+~,\[)]|$)/.test(selector) || /\[[^\]]+\]|#[a-zA-Z_]|:first-of-type/.test(selector)))
}

/**
 * 为文件记录可定位的规范问题。
 * @param path - 问题所在文件。
 * @param message - 面向开发者的中文问题说明。
 * @returns 无返回值。
 */
function issue(path: string, message: string): void {
  issues.push(`${relative(root, path)}：${message}`)
}

// 保证门禁能够实际拦截页面直接请求，同时允许具名领域接口调用。
assert.equal(bypassesApi("uni.request({ url: '/api/orders' })"), true)
assert.equal(bypassesApi("import { request } from '../../api/client'"), true)
assert.equal(bypassesApi("import { token, errorMessage } from '../../api/client'"), false)
assert.equal(bypassesApi("const rows = await api('/orders')"), true)
assert.equal(bypassesApi("import { listOrders } from '../../api/orders.api'; await listOrders({ role: 'buyer' })"), false)
assert.equal(hasUnifiedReturn('<template><PageHeader title="表单" back /></template>'), true)
assert.equal(hasUnifiedReturn('<template><FloatingBack /></template>'), true)
assert.equal(hasUnifiedReturn('<template><button @click="back" /></template><script setup lang="ts">import { back } from \'../lib/navigation\'</script>'), true)
assert.equal(hasUnifiedReturn('<template><view /></template>'), false)
assert.equal(hasUnifiedReturn('<template><PageHeader :back="false" /></template>'), false)

assert.equal(unsupportedComponentSelector('<style>.row>text{color:red}</style>'), '.row>text')
assert.equal(unsupportedComponentSelector('<style>.row .label{color:red}@media(max-width:350px){.label{font-size:12px}}</style>'), undefined)

const sourceFiles = filesIn(sourceRoot)
const pages = sourceFiles.filter(path => path.includes('/pages/') && path.endsWith('.vue'))
let bottomNavMounts = 0

for (const path of sourceFiles) {
  const extension = extname(path)
  if (['.js', '.jsx', '.mjs', '.cjs'].includes(extension)) issue(path, '前端业务与测试必须使用 TypeScript')
  if (!['.ts', '.vue'].includes(extension)) continue
  const text = readFileSync(path, 'utf8')
  const script = businessScript(text, extension === '.vue')
  if (/@ts-(?:ignore|nocheck)\b/.test(script)) issue(path, '禁止跳过 TypeScript 检查')
  if (extension === '.vue' && /<script\b/.test(text) && !/<script\b[^>]*lang=["']ts["']/.test(text)) issue(path, 'Vue 业务脚本必须声明 lang="ts"')
  if ((path.includes('/pages/') || path.includes('/components/')) && bypassesApi(script)) issue(path, '必须通过领域 *.api.ts 的具名方法请求服务端')
  const isComponent = path.includes('/components/') || (path.includes('/pages/main/') && !path.endsWith('/pages/main/index.vue'))
  if (extension === '.vue' && isComponent) {
    const selector = unsupportedComponentSelector(text)
    if (selector) issue(path, '微信组件样式必须使用类选择器：' + selector)
  }
  const mounts = [...text.matchAll(/<BottomNav\b/g)].length
  bottomNavMounts += mounts
  if (mounts && path !== resolve(sourceRoot, 'pages/main/index.vue')) issue(path, '主底栏只能由唯一主页面挂载')
}

for (const path of pages) {
  const pagePath = relative(resolve(sourceRoot, 'pages'), path).replaceAll('\\', '/')
  const png = resolve(root, 'design/pages', pagePath.replace(/\.vue$/, '.png'))
  const specification = png.replace(/\.png$/, '.design.md')
  if (!existsSync(png)) issue(path, `缺少对应先行设计图 design/pages/${pagePath.replace(/\.vue$/, '.png')}`)
  else {
    const data = readFileSync(png)
    if (data.length < 24 || data.toString('hex', 0, 8) !== '89504e470d0a1a0a') issue(png, '设计图必须为有效 PNG')
    else {
      const width = data.readUInt32BE(16), height = data.readUInt32BE(20)
      if (width < 320 || width > 780 || height < 600 || height > 1700) issue(png, `设计图 ${width}×${height} 超出正常手机首屏约定`)
    }
  }
  if (!existsSync(specification)) issue(path, '缺少同名前缀 .design.md 设计说明')
  const text = readFileSync(path, 'utf8')
  if (!pagePath.startsWith('main/') && !/<AppShell\b[^>]*\btitle=/.test(text)) issue(path, '二级页面必须使用带标题的统一 AppShell（自动提供返回）')
  if (pagePath.startsWith('main/') && pagePath !== 'main/index.vue' && /<(?:AppShell|BottomNav|PageHeader|UniNavBar|uni-nav-bar)\b/.test(text)) issue(path, '主内容视图不能重复挂载导航框架')
}

const client = resolve(sourceRoot, 'api/client.ts')
if (!existsSync(client)) issue(client, '缺少统一 API 传输工具')
const shell = resolve(sourceRoot, 'components/AppShell.vue')
if (!existsSync(shell) || !hasUnifiedReturn(readFileSync(shell, 'utf8'))) issue(shell, '统一二级页面框架必须提供共享返回实现，展示型可使用悬浮返回')
if (bottomNavMounts !== 1) issue(resolve(sourceRoot, 'pages/main/index.vue'), `底栏应挂载一次，实际 ${bottomNavMounts} 次`)
const mainPage = resolve(sourceRoot, 'pages/main/index.vue')
if (existsSync(mainPage)) {
  const text = readFileSync(mainPage, 'utf8')
  if (/<(?:PageHeader|UniNavBar|uni-nav-bar)\b/.test(text)) issue(mainPage, '四个主页面不显示顶部导航栏，仅处理状态栏与胶囊安全距离')
  if (/\breLaunch\s*\(/.test(text)) issue(mainPage, '主导航切换不能重新启动页面')
  if ((text.match(/v-show=/g) ?? []).length !== 4) issue(mainPage, '四个主视图应保留挂载与独立状态')
}

if (issues.length) {
  console.error(`前端规范检查未通过（${issues.length} 项）：\n${issues.map(item => `- ${item}`).join('\n')}`)
  process.exitCode = 1
} else console.log(`前端规范检查通过：${pages.length} 个页面/视图，TypeScript、接口分层、唯一底栏及设计目录一致。`)
