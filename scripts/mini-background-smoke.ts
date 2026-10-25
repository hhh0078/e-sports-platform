/** 文件作用：仅通过微信官方 SDK 与原始模拟器截图，验收四主导航及访客二级页的背景连续性、个人中心渐变和紧凑布局；不登录、不付款、不修改配置或缓存。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { ScrollViewElement } from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { Service } from '../frontend/src/types/models'
import { normalizeLabelText } from '../frontend/src/lib/cover'

/** 微信窗口的真实几何信息，不取令牌、OpenID 或页面内部状态。 */
interface WindowInfo { windowWidth: number; windowHeight: number; statusBarHeight: number }
/** SDK 实测节点的矩形。 */
interface Rectangle { left: number; top: number; width: number; height: number }
/** 最终证据保留的颜色、位置与用途。 */
interface Measurement { label: string; selector: string; rect: Rectangle; background: string }
/** PNG 解码只读取官方截图，绝不重新绘制或修改截图。 */
interface ScreenshotPixels { width: number; height: number; data: Buffer }
/** 节点验收沿真实 Page 和公开组件边界查询。 */
type Scope = Page | Element
/** 四个主导航的真实按钮位置。 */
type MainTab = 'player' | 'worker' | 'orders' | 'me'

const root = process.cwd(), require = createRequire(path.join(root, 'frontend/package.json'))
const project = path.resolve(process.env.UI_TEST_PROJECT || 'frontend/dist/build/mp-weixin')
const outputRoot = path.resolve(process.env.UI_TEST_OUTPUT_ROOT || 'output/background-continuity')
const endpoint = process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9423'
const cli = process.env.UI_WECHATIDE_CLI || '/Applications/wechatwebdevtools.app/Contents/MacOS/wechatide'
const runFile = promisify(execFile)
const responsive = process.env.UI_TEST_RESPONSIVE === '1'
const scrollOnly = process.env.UI_TEST_SCROLL_ONLY === '1'
const publicAPI = process.env.UI_TEST_API || 'https://game.ai.58it.cn/api'
assert.ok(['https://game.ai.58it.cn/api','http://127.0.0.1:18091/api'].includes(publicAPI),'仅允许正式公开GET或指定本地验收API')
const automator = require('miniprogram-automator') as { connect(options: { wsEndpoint: string }): Promise<MiniProgram> }
const SDK = (require('miniprogram-automator/out/MiniProgram') as { default: typeof MiniProgram }).default
const png = require('pngjs') as { PNG: { sync: { read(bytes: Buffer): ScreenshotPixels } } }
const originalCheckVersion = SDK.prototype.checkVersion
const checks: string[] = [], exceptions: string[] = [], colorWarnings: string[] = [], screenshots: string[] = []
const measurements: Measurement[] = [], versionChecks: Array<Record<string, unknown>> = []
const scrollMeasurements: Array<{ label: string; viewport_height: number; content_height: number; scroll_top: number; scroll_width: number }> = []
const scrollRetentionMeasurements: Array<{ label: string; kind: 'tab_roundtrip' | 'detail_return'; tab: MainTab; scrollable_range: number; before?: number; after?: number; difference?: number; status: 'passed' | 'failed' | 'not_scrollable' }> = []
const pixels: Array<{ label: string; image: string; x: number; y: number; actual: number[]; expected: number[]; difference: number }> = []
const responsiveMeasurements: Array<Record<string, unknown>> = []
let mini: MiniProgram | undefined, viewport: WindowInfo | undefined, output = outputRoot, stage = '初始化', failure: unknown
let originalGame: unknown, restored = false, preferenceRead = false, originalRoute = ''

/** 受控表达式只允许读取公开微信窗口和指定的游戏偏好，不写业务数据。 */
declare const wx: { getWindowInfo(): WindowInfo; getStorageSync(key: string): unknown }

/** 取得当前官方 SDK 连接，未连接时明确失败。 */
function sdk(): MiniProgram { assert.ok(mini, 'SDK 尚未连接'); return mini }
/** 清除异常中的潜在秘密；只写精简错误，不保存完整控制台数据。 */
function redact(value: unknown): string { return String(value).replace(/(token|secret|openid|authorization)["'\s:=]+[^\s,}"']+/gi, '$1=[REDACTED]').slice(0, 1600) }
/** 观察实际 UI 直到条件成立，不重试业务提交、不写页面状态。 */
async function until<T>(read: () => Promise<T | false | null | undefined>, label: string, timeout = 15000): Promise<T> {
  const start = Date.now()
  while (Date.now() - start < timeout) { const value = await read(); if (value) return value; await new Promise(resolve => setTimeout(resolve, 150)) }
  throw new Error('等待超时：' + label)
}
/** 仅当 SDK 原版本校验因缺元数据访问 split 失败，改用真实微信 SDKVersion 核验最低版本。 */
function compatibilityCheck(): void {
  SDK.prototype.checkVersion = async function checkedVersion(this: MiniProgram): Promise<void> {
    try { await originalCheckVersion.call(this); versionChecks.push({ method: 'SDK原校验', passed: true }); return }
    catch (error) {
      if (!(error instanceof TypeError) || !/split/.test(error.message)) throw error
      const metadata = await this.send('Tool.getInfo') as { SDKVersion?: unknown }
      if (typeof metadata.SDKVersion === 'string' && metadata.SDKVersion.length > 0) throw error
      const system = await this.callWxMethod('getSystemInfoSync') as { SDKVersion?: unknown }
      assert.equal(typeof system.SDKVersion, 'string', '真实微信 SDKVersion 缺失，禁止跳过版本校验')
      const version = String(system.SDKVersion), parts = version.split('.').map(Number)
      assert.ok(/^\d+\.\d+\.\d+(?:\.\d+)?$/.test(version) && parts.every(Number.isFinite), '无法校验真实微信 SDKVersion')
      const [major, minor, patch] = parts
      assert.ok(major > 2 || major === 2 && (minor > 7 || minor === 7 && patch >= 3), '微信 SDKVersion 必须至少为 2.7.3')
      versionChecks.push({ method: 'Tool元数据缺失时读取真实getSystemInfoSync', SDKVersion: version, minimum: '2.7.3', passed: true, original_error: redact(error.message) })
    }
  }
}
/** 跨编译后的真实组件标签查找，不调用 Vue 方法或内部 data。 */
async function child(scope: Scope, names: string[]): Promise<Element | null> { for (const name of names) { const item = await scope.$(name); if (item) return item } return null }
/** 在公开组件与页面插槽范围查找实际节点。 */
async function find(scopes: Scope[], selector: string): Promise<Element | null> { for (const scope of scopes) { const item = await scope.$(selector); if (item) return item } return null }
/** 查询真实节点矩形，不用 CSS 声明高度充当运行几何。 */
async function rect(item: Element): Promise<Rectangle> { const [offset, size] = await Promise.all([item.offset(), item.size()]); return { left: Number(offset.left), top: Number(offset.top), width: Number(size.width), height: Number(size.height) } }
/** 检查真实矩形处于另一个可见区域内，保留一像素渲染舍入容差。 */
function inside(area: Rectangle, bounds: Rectangle, label: string): void { assert.ok(area.width > 0 && area.height > 0 && area.left >= bounds.left - 1 && area.top >= bounds.top - 1 && area.left + area.width <= bounds.left + bounds.width + 1 && area.top + area.height <= bounds.top + bounds.height + 1, label + '越界：' + JSON.stringify({ area, bounds })) }
/** 两个独立内容层只允许边缘的一像素舍入，不允许互相遮盖。 */
function separate(a: Rectangle, b: Rectangle): boolean { return a.left + a.width <= b.left + 1 || b.left + b.width <= a.left + 1 || a.top + a.height <= b.top + 1 || b.top + b.height <= a.top + 1 }
/** 获取同名组件集合，兼容uni-app短标签及完整编译路径。 */
async function children(scope: Scope, names: string[]): Promise<Element[]> { for (const name of names) { const items = await scope.$$(name) as Element[]; if (items.length) return items } return [] }
/** 将已计算颜色缩窄为不透明 RGB；透明背景不能通过公共背景层验收。 */
function rgb(value: string): number[] {
  const hex = value.trim().match(/^#([\da-f]{6})$/i)
  if (hex) return [0, 2, 4].map(index => Number.parseInt(hex[1].slice(index, index + 2), 16))
  const match = value.match(/^rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:\s*[,/]\s*([\d.]+))?\s*\)$/i)
  assert.ok(match, '无效或未解析的背景色：' + value)
  assert.ok(match[4] === undefined || Number(match[4]) === 1, '公共背景层不能透明：' + value)
  return match.slice(1, 4).map(Number)
}
/** 统一记录真实节点的边界及显式背景，隐藏节点不能算通过。 */
async function measure(scopes: Scope[], selector: string, label: string, expected?: number[]): Promise<{ item: Element; area: Rectangle; color: number[] }> {
  const item = await until(() => find(scopes, selector), label + ' ' + selector), area = await rect(item), background = await item.style('background-color')
  assert.ok(area.width > 0 && area.height > 0, label + '节点不可见')
  assert.ok(area.left >= -1 && area.left + area.width <= Number(viewport?.windowWidth) + 1, label + '水平溢出：' + JSON.stringify(area))
  const color = rgb(background); if (expected) assert.deepEqual(color, expected, label + '公共层背景不一致')
  measurements.push({ label, selector, rect: area, background }); return { item, area, color }
}
/** 记录断言，截图另走官方 CLI 串行操作。 */
function passed(label: string): void { checks.push(label); console.log('PASS ' + label) }
/** 检查长内容已实际滚动到底部，记录原生滚动值而非只调用滚动方法。 */
async function scrollEvidence(scroll: ScrollViewElement, label: string): Promise<void> {
  const area = await rect(scroll), content = Number(await scroll.scrollHeight()), top = Number(await scroll.property('scrollTop')), width = Number(await scroll.scrollWidth())
  assert.ok(Number.isFinite(content) && Number.isFinite(top) && Number.isFinite(width), label + '原生滚动测量缺失')
  if (content > area.height + 2) assert.ok(top > 0, label + '长内容未发生实际滚动')
  assert.ok(width <= Number(viewport?.windowWidth) + 1, label + '滚动内容横向溢出')
  scrollMeasurements.push({ label, viewport_height: area.height, content_height: content, scroll_top: top, scroll_width: width })
}
/** 在非零浏览位置离开并返回同一常驻标签，期间不重新发滚动命令；短空态明确记录不可滚动。 */
async function mainScrollRetention(page: Page, buttons: Element[], game: string): Promise<void> {
  let checked = 0
  const tabs: MainTab[] = ['player', 'worker', 'orders', 'me']
  for (const [index, tab] of tabs.entries()) {
    stage = game + '非零滚动保留 ' + tab
    await buttons[index].tap()
    await until(async () => (await buttons[index].attribute('class')).split(/\s+/).includes('selected'), tab + '保留测试激活')
    await page.waitFor(250)
    const scrolls = await page.$$('.main-scroll') as ScrollViewElement[], scroll = scrolls[index]
    assert.ok(scroll)
    const area = await rect(scroll), contentHeight = Number(await scroll.scrollHeight()), range = Math.max(0, contentHeight - area.height)
    assert.ok(Number.isFinite(range), tab + '滚动范围无效')
    if (range <= 4) { scrollRetentionMeasurements.push({ label: game + ' ' + tab, kind: 'tab_roundtrip', tab, scrollable_range: range, status: 'not_scrollable' }); continue }
    await scroll.scrollTo(0, Math.min(range, Math.max(8, Math.min(180, area.height / 2))))
    await page.waitFor(180)
    const before = Number(await scroll.property('scrollTop'))
    assert.ok(Number.isFinite(before) && before > 2, tab + '测试必须从真实非零位置开始')
    const other = (index + 1) % tabs.length
    await buttons[other].tap()
    await until(async () => (await buttons[other].attribute('class')).split(/\s+/).includes('selected'), tab + '离开到相邻标签')
    await buttons[index].tap()
    await until(async () => (await buttons[index].attribute('class')).split(/\s+/).includes('selected'), tab + '返回原标签')
    assert.equal(await sdk().currentPage(), page, '切标签往返不能重建主页面')
    await page.waitFor(350)
    const restored = (await page.$$('.main-scroll') as ScrollViewElement[])[index], after = Number(await restored.property('scrollTop')), difference = Math.abs(after - before)
    scrollRetentionMeasurements.push({ label: game + ' ' + tab, kind: 'tab_roundtrip', tab, scrollable_range: range, before, after, difference, status: Number.isFinite(after) && difference <= 1 ? 'passed' : 'failed' })
    assert.ok(Number.isFinite(after) && difference <= 1, tab + '切标签往返丢失非零位置：' + JSON.stringify({ before, after }))
    await screenshot(game + '-' + tab + '-scroll-retained')
    checked++
  }
  assert.ok(checked > 0, game + '没有可滚动主视图，不能将零位置当作保留测试通过')
  passed(game + '非零滚动切标签往返保留：' + checked + '个长视图；短空态单独记录（仅程序几何）')
}
/** 通过官方 reLaunch 初始化路由，主导航连续切换时不使用此方法。 */
async function open(url: string): Promise<Page> { stage = '打开 ' + url; console.log(stage); const page = await sdk().reLaunch(url); assert.ok(page); assert.equal(page.path, url.split('?')[0].replace(/^\//, '')); return page }
/** 从官方 CLI JSON 输出中寻找已落地 PNG 路径，兼容工具另存文件名。 */
function screenshotPath(value: unknown): string | undefined {
  if (typeof value === 'string') { const candidate = path.resolve(value); return /\.png$/i.test(candidate) && fs.existsSync(candidate) ? candidate : undefined }
  if (Array.isArray(value)) { for (const item of value) { const found = screenshotPath(item); if (found) return found } }
  else if (value && typeof value === 'object') for (const item of Object.values(value)) { const found = screenshotPath(item); if (found) return found }
  return undefined
}
/** 保存官方模拟器原始截图，仅读取文件核验 PNG，不重新编码、裁剪或绘制。 */
async function screenshot(label: string): Promise<{ file: string; image: ScreenshotPixels }> {
  await new Promise(resolve => setTimeout(resolve, 350))
  const target = path.join(output, label + '.png'), result = await runFile(cli, ['-c', 'Codex', 'simulator_screenshot', '--project', project, '--path', target, '--optimize', 'false'], { timeout: 25000, maxBuffer: 1024 * 1024 })
  let file = fs.existsSync(target) ? target : undefined
  if (!file) { const text = result.stdout.trim(), first = text.indexOf('{'), last = text.lastIndexOf('}'); if (first >= 0 && last > first) file = screenshotPath(JSON.parse(text.slice(first, last + 1)) as unknown) }
  assert.ok(file, '官方模拟器未返回已落地 PNG 文件')
  const bytes = fs.readFileSync(file); assert.equal(bytes.subarray(0, 8).toString('hex'), '89504e470d0a1a0a')
  const image = png.PNG.sync.read(bytes); assert.ok(image.width > 0 && image.height > 0)
  assert.ok(Math.abs(image.width / Number(viewport?.windowWidth) - image.height / Number(viewport?.windowHeight)) < 0.05, '截图非当前设备内容比例，不可盲目映射采样坐标')
  screenshots.push(file); return { file, image }
}
/** 按真实截图与窗口比例采样，容差仅允许渲染舍入，不修改图片。 */
function sample(source: { file: string; image: ScreenshotPixels }, label: string, cssX: number, cssY: number, expected: number[], tolerance = 3): void {
  assert.ok(viewport); const { image } = source, x = Math.floor(cssX * image.width / viewport.windowWidth), y = Math.floor(cssY * image.height / viewport.windowHeight)
  assert.ok(x >= 0 && y >= 0 && x < image.width && y < image.height, '采样点越出原始截图')
  const index = (y * image.width + x) * 4, actual = Array.from(image.data.subarray(index, index + 3)), difference = Math.max(...actual.map((channel, i) => Math.abs(channel - expected[i])))
  pixels.push({ label, image: source.file, x: cssX, y: cssY, actual, expected, difference })
  assert.ok(difference <= tolerance, label + '截图背景不连续：' + JSON.stringify({ actual, expected, difference, cssY }))
}
/** 实际点击全部游戏目录，检查受控滚动与网格边界，取消不能改变游戏偏好。 */
async function responsiveCatalog(page: Page, picker: Element, nav: Element, game: string): Promise<void> {
  assert.ok(viewport); stage = game + '响应式游戏目录与底栏图标'
  const icons = await children(nav, ['icon', 'components\\/Icon']); assert.equal(icons.length, 4, '底栏必须有四个共享图标')
  const expected = 23 * viewport.windowWidth / 390
  for (const [index, icon] of icons.entries()) {
    const box = await until(() => icon.$('.app-icon'), '底栏图标外框'), boxArea = await rect(box)
    assert.ok(Math.abs(boxArea.width - expected) <= 1 && Math.abs(boxArea.height - expected) <= 1, '底栏图标未按rpx适配')
    const mask = await icon.$('.gamepad-image')
    if (index === 0) { assert.ok(mask, '玩家图标应使用统一遮罩'); const area = await rect(mask); assert.ok(Math.abs(area.width - boxArea.width) <= 1 && Math.abs(area.height - boxArea.height) <= 1, '玩家遮罩与图标外框不一致') }
    else { const official = await until(() => child(icon, ['uni-icons', 'node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-icons\\/uni-icons']), '官方图标组件'), glyph = await until(() => official.$('.uni-icons'), '官方图标字形'), font = Number.parseFloat(await glyph.style('font-size')); assert.ok(Math.abs(font - expected) <= 1, '官方图标字号仍使用固定px'); responsiveMeasurements.push({ label: game + '底栏图标' + index, rect: boxArea, font_size: font, expected }) }
  }
  await (await until(() => picker.$('.catalog-trigger'), '全部游戏入口')).tap()
  const selector = await until(() => child(picker, ['game-selector', 'components\\/GameSelector']), '共享游戏选择器'), native = await until(() => child(selector, ['uni-popup', 'node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup']), '游戏目录官方弹层')
  const scopes: Scope[] = [selector, native, picker, page], sheet = await until(() => find(scopes, '.catalog-sheet'), '游戏目录内容'), scroll = await until(() => find(scopes, '.catalog-scroll'), '游戏目录滚动区域') as ScrollViewElement
  await until(async () => { const area = await rect(sheet); return area.height > 0 && area.top >= -1 && area.top + area.height <= Number(viewport?.windowHeight) + 1 }, '游戏目录展开动画')
  const sheetArea = await rect(sheet), scrollArea = await rect(scroll), grid = await until(() => find(scopes, '.catalog-grid'), '游戏网格'), gridArea = await rect(grid)
  inside(sheetArea, { left: 0, top: 0, width: viewport.windowWidth, height: viewport.windowHeight }, '游戏目录弹层'); inside(scrollArea, sheetArea, '游戏目录滚动框')
  assert.ok(gridArea.left >= scrollArea.left - 1 && gridArea.left + gridArea.width <= scrollArea.left + scrollArea.width + 1, '游戏网格横向溢出')
  const options = await selector.$$('.catalog-game') as Element[]; assert.ok(options.length >= 3, '游戏目录未加载完整网格')
  for (const option of options) { const area = await rect(option); assert.ok(area.width > 0 && area.left >= gridArea.left - 1 && area.left + area.width <= gridArea.left + gridArea.width + 1, '游戏选项挤窄或越出网格') }
  await screenshot(game + '-responsive-catalog'); await scroll.scrollTo(0, 100000); await page.waitFor(160); await scrollEvidence(scroll, game + '游戏目录内部滚动')
  await (await until(() => find(scopes, '.selector-close'), '关闭游戏目录')).tap(); await until(async () => !await native.$('.uni-popup'), '游戏目录已关闭')
  assert.deepEqual(await sdk().evaluate(() => wx.getStorageSync('jijie_game')), originalGame, '关闭目录不得修改游戏偏好')
  responsiveMeasurements.push({ label: game + '游戏选择器', sheet: sheetArea, scroll: scrollArea, grid: gridArea, game_count: options.length })
  passed(game + '游戏弹层和网格无横溢、真实内部滚动；四图标外框与字形按23×屏宽/390适配')
}
/** 访客真实打开提现规则，检查根层固定浮层、内容滚动、触控下限和正文位置。 */
async function responsiveRules(page: Page, shell: Element, game: string): Promise<void> {
  assert.ok(viewport); stage = game + '响应式钱包规则'; await sdk().pageScrollTo(0); await page.waitFor(180)
  const anchor = await until(() => find([page, shell], '.balance-section'), '钱包正文锚点'), before = await rect(anchor)
  const rule = await until(() => child(page, ['withdrawal-rules', 'components\\/WithdrawalRules']), '提现规则组件')
  await (await until(() => rule.$('.withdrawal-rules-link'), '提现规则入口')).tap()
  const finance = await until(() => child(rule, ['finance-popup', 'components\\/FinancePopup']), '共享资金浮层'), native = await until(() => child(finance, ['uni-popup', 'node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup']), '官方资金浮层')
  const scopes: Scope[] = [rule, finance, native, page], sheet = await until(() => find(scopes, '.finance-popup-sheet'), '真实资金浮层内容')
  await until(async () => { const area = await rect(sheet); return area.height > 0 && area.top >= -1 && area.top + area.height <= Number(viewport?.windowHeight) + 1 }, '资金浮层展开动画')
  const sheetArea = await rect(sheet), windowArea = { left: 0, top: 0, width: viewport.windowWidth, height: viewport.windowHeight }
  inside(sheetArea, windowArea, '资金浮层'); assert.ok(sheetArea.height <= viewport.windowHeight * .82 + 1, '资金浮层超过82vh')
  const fixed = await until(() => native.$('.uni-popup'), '官方固定浮层根节点'); assert.equal(await fixed.style('position'), 'fixed', '资金浮层不能嵌入文档流')
  const close = await until(() => find(scopes, '.finance-popup-close'), '资金浮层关闭按钮'); inside(await rect(close), sheetArea, '关闭按钮')
  const after = await rect(anchor); assert.ok(Math.abs(after.top - before.top) <= 1 && Math.abs(after.height - before.height) <= 1, '资金浮层推移正文')
  assert.match(await sheet.text(), /0元起提现/); assert.match(await sheet.text(), /每天可提现一次/)
  await screenshot(game + '-responsive-wallet-rules-top')
  const scroll = await until(() => find(scopes, '.finance-popup-scroll'), '资金浮层内部滚动') as ScrollViewElement, primary = await until(() => find(scopes, '.withdrawal-rules-close'), '规则确认按钮')
  await scroll.scrollTo(0, 100000); await page.waitFor(180); await scrollEvidence(scroll, game + '钱包规则内部滚动')
  const primaryArea = await rect(primary); inside(primaryArea, await rect(scroll), '滚动后确认按钮'); inside(primaryArea, windowArea, '窗口内确认按钮'); assert.ok(primaryArea.height >= 43.99, '主提交触控高度小于44px')
  await screenshot(game + '-responsive-wallet-rules-bottom'); await close.tap(); await until(async () => !await native.$('.uni-popup'), '资金关闭按钮生效')
  const closed = await rect(anchor); assert.ok(Math.abs(closed.top - before.top) <= 1 && Math.abs(closed.height - before.height) <= 1, '关闭浮层后正文位置改变')
  responsiveMeasurements.push({ label: game + '钱包规则', sheet: sheetArea, primary: primaryArea, anchor_before: before, anchor_after: closed })
  passed(game + '访客提现规则为窗口内真实浮层、内部可滚、关闭可用且主操作触控≥44px')
}
/** 从明确的同环境API读取已有公开服务，不附身份、不创建数据；每个主题仅取一个样本。 */
async function publicService(game: string): Promise<Service> {
  const response = await fetch(publicAPI + '/services?game_id=' + encodeURIComponent(game) + '&sort=recommended&limit=1', { method: 'GET', redirect: 'error', signal: AbortSignal.timeout(15000), headers: { Accept: 'application/json' } })
  assert.equal(response.status, 200, '对应环境公开服务读取失败')
  const result = await response.json() as { data?: Service[] }; assert.ok(Array.isArray(result.data) && result.data.length > 0, game + '没有可验证的真实公开服务')
  const value = result.data[0]; assert.ok(value.id && value.title && Array.isArray(value.labels) && value.game_id === game, '公开服务样本结构错误'); return value
}
/** 按真实组件几何核对已有文案完整、系统标题标签与价格正常；不声称覆盖人工六文案矩阵。 */
async function responsiveCover(scopes: Scope[], service: Service, detail: boolean): Promise<void> {
  const host = await until(async () => { for (const scope of scopes) { const value = await child(scope, ['cover-art', 'components\\/CoverArt']); if (value) return value } return null }, '共享封面组件')
  const cover = await until(() => host.$('.cover-art'), '真实封面'), bounds = await rect(cover), labels = await host.$$('.cover-label') as Element[]
  const expected = service.labels.slice(0, 6).map(label => normalizeLabelText(label.text)).filter(Boolean); assert.equal(labels.length, expected.length, '真实服务文案被隐藏')
  const areas: Rectangle[] = [], system: Rectangle[] = []
  for (const [index, label] of labels.entries()) { const area = await rect(label); inside(area, bounds, '自定义文案'); assert.equal((await label.text()).trim(), expected[index], '自定义文案不完整'); assert.notEqual(await label.style('visibility'), 'hidden'); assert.ok(Number.parseFloat(await label.style('font-size')) >= (detail ? 13.9 : 9.9), '用户文案被缩成不可读字号'); areas.push(area) }
  for (let first = 0; first < areas.length; first++) for (let second = first + 1; second < areas.length; second++) assert.ok(separate(areas[first], areas[second]), '自定义文案相互重叠')
  const all: Scope[] = [...scopes, host], title = await until(() => find(all, detail ? '.detail-title' : '.service-title'), '服务标题'), tags = await until(() => find(all, detail ? '.detail-labels' : '.service-tags'), '服务分类标签')
  assert.equal((await title.text()).trim(), service.title.trim(), '服务标题丢失')
  for (const item of [title, tags]) { const area = await rect(item); inside(area, bounds, '系统标题或标签'); system.push(area) }
  const deposits = await find(all, detail ? '.detail-deposits' : '.service-deposits'); if (deposits) { const area = await rect(deposits); inside(area, bounds, '押金角标'); system.push(area) }
  for (const label of areas) for (const area of system) assert.ok(separate(label, area), '自定义文案与系统内容重叠')
  for (let first = 0; first < system.length; first++) for (let second = first + 1; second < system.length; second++) assert.ok(separate(system[first], system[second]), '标题、标签或押金相互重叠')
  const price = await until(() => find(all, detail ? '.price-row' : '.service-price'), '服务价格'), priceArea = await rect(price)
  assert.ok(priceArea.top >= bounds.top + bounds.height - 1, '价格覆盖封面'); assert.ok(priceArea.left >= -1 && priceArea.left + priceArea.width <= Number(viewport?.windowWidth) + 1, '服务价格横向溢出')
  assert.ok(system[0].top + system[0].height <= system[1].top + 1, '服务标题应位于标签上方')
  responsiveMeasurements.push({ label: service.game_id + (detail ? '详情' : '列表') + '真实样本', service_id: service.id, source: publicAPI + '公开GET', cover: bounds, user_labels: areas, system, price: priceArea, existing_label_count: expected.length })
}
/** 通过真实列表卡片点击详情，并使用已有公开内容核验，不登录也不提交下单。 */
async function responsiveService(game: string): Promise<void> {
  stage = game + '访客真实服务样本'; const service = await publicService(game), page = await open('/pages/main/index?tab=player&game_id=' + encodeURIComponent(game))
  const player = await until(() => child(page, ['player-view', 'pages\\/main\\/player\\/player']), '玩家列表视图')
  const card = await until(async () => { for (const item of await children(player, ['service-card', 'components\\/ServiceCard'])) { const title = await item.$('.service-title'); if (title && (await title.text()).trim() === service.title.trim()) return item } return null }, '与公开GET匹配的真实服务卡片')
  await responsiveCover([card], service, false)
  const title = await until(() => card.$('.service-title'), '可点击服务标题'), scrolls = await page.$$('.main-scroll') as ScrollViewElement[], scroll = scrolls[0]
  assert.ok(scroll); const titleArea = await rect(title), scrollArea = await rect(scroll), scrollTop = Number(await scroll.property('scrollTop'))
  await scroll.scrollTo(0, Math.max(0, scrollTop + titleArea.top - scrollArea.top - 56)); await page.waitFor(180); await screenshot(game + '-responsive-service-card')
  const beforeDetail = Number(await scroll.property('scrollTop')), scrollRange = Math.max(0, Number(await scroll.scrollHeight()) - scrollArea.height)
  assert.ok(Number.isFinite(beforeDetail) && beforeDetail > 2, '详情返回测试必须从真实非零列表位置开始')
  await title.tap()
  const detail = await until(async () => { const value = await sdk().currentPage(); return value?.path === 'pages/services/detail' ? value : null }, '真实服务详情路由')
  assert.equal(detail.query.id, service.id, '列表点击进入错误服务')
  const shell = await until(() => child(detail, ['app-shell', 'components\\/AppShell']), '详情公共壳'); await until(() => find([detail, shell], '.detail-title'), '真实详情加载')
  await responsiveCover([detail, shell], service, true); await screenshot(game + '-responsive-service-detail-top')
  const price = await until(() => find([detail, shell], '.price-row'), '详情价格'), priceArea = await rect(price)
  if (priceArea.top + priceArea.height > Number(viewport?.windowHeight) - 70) { await sdk().pageScrollTo(Math.max(0, priceArea.top - 100)); await detail.waitFor(160); await screenshot(game + '-responsive-service-detail-price') }
  passed(game + '一个已有服务列表与详情：独立文案完整无重叠，标题/分类/价格边界正常（仅访客样本）')
  stage = game + '详情返回非零列表位置'
  await sdk().navigateBack()
  const returned = await until(async () => { const current = await sdk().currentPage(); return current?.path === 'pages/main/index' ? current : null }, '从详情返回主页面')
  assert.equal(returned, page, '详情返回不能重建列表Page')
  await returned.waitFor(350)
  const restored = (await returned.$$('.main-scroll') as ScrollViewElement[])[0], afterDetail = Number(await restored.property('scrollTop')), difference = Math.abs(afterDetail - beforeDetail)
  scrollRetentionMeasurements.push({ label: game + '服务详情返回', kind: 'detail_return', tab: 'player', scrollable_range: scrollRange, before: beforeDetail, after: afterDetail, difference, status: Number.isFinite(afterDetail) && difference <= 1 ? 'passed' : 'failed' })
  assert.ok(Number.isFinite(afterDetail) && difference <= 1, '详情返回丢失非零列表位置：' + JSON.stringify({ before: beforeDetail, after: afterDetail }))
  await screenshot(game + '-service-return-scroll-retained')
  passed(game + '服务详情返回保留非零列表位置（仅程序几何）')
}
/** 主页面连续真实点击四底栏，逐一检查保留同一 Page、显式背景及高度。 */
async function mainTheme(game: string): Promise<void> {
  const page = await open('/pages/main/index?tab=player&game_id=' + encodeURIComponent(game))
  const player = await until(() => child(page, ['player-view', 'pages\\/main\\/player\\/player']), '玩家视图'), picker = await until(() => child(player, ['game-picker', 'components\\/GamePicker']), '真实游戏目录')
  await until(async () => { const chosen = await picker.$('.game-choice.chosen'); return chosen && await chosen.attribute('id') === 'game-option-' + game }, '路由指定主题已实际选中')
  const nav = await until(() => child(page, ['bottom-nav', 'components\\/BottomNav']), '唯一底栏组件')
  if (responsive) await responsiveCatalog(page, picker, nav, game)
  const buttons = await nav.$$('.nav-item') as Element[]; assert.equal(buttons.length, 4)
  const tabs: MainTab[] = ['player', 'worker', 'orders', 'me']
  let base: number[] | undefined
  for (const [index, tab] of tabs.entries()) {
    stage = game + '真实切换 ' + tab; await buttons[index].tap()
    await until(async () => (await buttons[index].attribute('class')).split(/\s+/).includes('selected'), tab + '底栏选中')
    assert.equal(await sdk().currentPage(), page, '底栏切换不能销毁并重建主页面')
    const shell = await measure([page], '.main-shell', game + ' ' + tab + '整页背景', base); base = shell.color
    const region = await measure([page], '.main-viewport', game + ' ' + tab + '内容视口', base), bottom = await measure([nav, page], '.bottom-nav', game + ' ' + tab + '底栏', base)
    const scrolls = await page.$$('.main-scroll') as ScrollViewElement[]; assert.equal(scrolls.length, 4)
    const scroll = scrolls[index], scrollArea = await rect(scroll); assert.ok(scrollArea.width > 0 && scrollArea.height > 0)
    assert.deepEqual(rgb(await scroll.style('background-color')), base, '滚动层背景必须与整页相同')
    assert.ok(Math.abs(region.area.top + region.area.height - bottom.area.top) <= 1, '视口与底栏间有空隙或重叠')
    const statusInset = tab === 'worker' || tab === 'orders' ? Number(viewport?.statusBarHeight) : 0
    assert.ok(Math.abs(scrollArea.top - region.area.top - statusInset) <= 1, '滚动层顶部与固定安全区不一致')
    assert.ok(Math.abs(scrollArea.height - (region.area.height - statusInset)) <= 1, '滚动层未填满扣除自身固定安全区后的视口')
    assert.ok(Math.abs(scrollArea.top + scrollArea.height - bottom.area.top) <= 1, '滚动层底部与底栏不对齐')
    assert.ok(Math.abs(shell.area.height - Number(viewport?.windowHeight)) <= 1, '整页高度不等于真实窗口')
    await scroll.scrollTo(0, 0); await page.waitFor(120)
    if (tab === 'me') await personal(page, scroll, base, game, bottom.area)
    else { await screenshot(game + '-' + tab + '-top'); await scroll.scrollTo(0, 100000); await page.waitFor(180); assert.deepEqual(rgb(await scroll.style('background-color')), base); await screenshot(game + '-' + tab + '-bottom') }
    await scrollEvidence(scroll, game + ' ' + tab)
    passed(game + ' ' + tab + '保持主页面，公共层底色和高度连续，滚动后不横溢')
  }
  await mainScrollRetention(page, buttons, game)
  assert.deepEqual(await sdk().evaluate(() => wx.getStorageSync('jijie_game')), originalGame, '只浏览主题不能写入游戏偏好')
}
/** 核验个人中心封面真实渐变、上移身份与紧凑订单间距，账户合计下保留可提现和充值仅消费说明。 */
async function personal(page: Page, scroll: ScrollViewElement, base: number[], game: string, bottom: Rectangle): Promise<void> {
  const view = await until(() => child(page, ['me-view', 'pages\\/main\\/me\\/me']), '个人中心真实组件')
  const hero = await until(() => view.$('.profile-hero'), '个人封面'), shade = await until(() => view.$('.profile-hero-shade'), '个人封面灰色渐变'), identity = await until(() => view.$('.identity-row'), '头像昵称')
  const orders = await until(() => view.$('.orders-section'), '订单入口'), heroRect = await rect(hero), identityRect = await rect(identity), orderRect = await rect(orders)
  const expectedHeroHeight = Number(viewport?.windowWidth) <= 350 ? 140 : 168
  assert.ok(Math.abs(heroRect.height - expectedHeroHeight) <= 1, '个人封面未按屏宽收紧：' + heroRect.height)
  assert.ok(identityRect.top - heroRect.top < Number(viewport?.statusBarHeight) + 44 - 4, '头像昵称未上移')
  assert.ok(identityRect.top + identityRect.height <= heroRect.top + heroRect.height + 1, '身份内容越出封面')
  assert.ok(orderRect.top - (heroRect.top + heroRect.height) <= 12, '封面与我的订单间距过大')
  const gradient = await shade.style('background-image'); assert.match(gradient, /linear-gradient/i)
  assert.match(gradient, /100%\s*\)\s*$/, '封面渐变终点必须明确为100%')
  const stops = Array.from(gradient.matchAll(/([\d.]+)%/g), match => Number(match[1])); assert.ok(stops.length >= 2 && stops.every(stop => stop <= 100), '渐变不得使用超出100%的色标')
  const name = await until(() => view.$('.profile-name'), '访客名称'); assert.match(await name.text(), /登录后查看/)
  const wallet = await until(() => view.$('.me-wallet'), '紧凑钱包卡'), walletRect = await rect(wallet), walletText = await wallet.text()
  assert.ok(walletRect.height <= 170, '个人中心钱包卡超过170px'); assert.match(walletText, /账户余额/); assert.match(walletText, /其中可提现 ¥0\.00，充值仅用于消费/)
  assert.equal(await (await until(() => view.$('.wallet-main-value'), '访客账户合计')).text(), '¥0.00'); assert.equal(await view.$('.wallet-recharge'), null, '不恢复重复充值金额行')
  for (const word of ['进行中', '结算中', '提现中']) assert.ok(walletText.includes(word), '钱包丢失' + word + '分区')
  const image = await screenshot(game + '-me-top')
  for (const [label, y] of [['渐变底部', heroRect.top + heroRect.height - 1], ['订单上沿', orderRect.top + 4], ['钱包旁边', walletRect.top + 24], ['主滚动底部', bottom.top - 5], ['底栏侧边', bottom.top + 16]] as const) sample(image, game + label, 4, y, base, label === '渐变底部' ? 12 : 3)
  await scroll.scrollTo(0, 100000); await page.waitFor(180); assert.deepEqual(rgb(await scroll.style('background-color')), base)
  const lower = await screenshot(game + '-me-bottom'); for (const y of [100, Number(viewport?.windowHeight) / 2, bottom.top - 5]) sample(lower, game + '滚动后页面边缘', 4, y, base)
  passed(game + '个人封面末端100%过渡，头像上移，订单间距≤12px，钱包资金分区保留')
}
/** 检查访客二级页公共壳与正文底色一致，长页面滚动后依旧连续且无主底栏。 */
async function secondary(game: string): Promise<void> {
  for (const [slug, route, ready] of [['login', '/pages/account/login', '.login-primary'], ['agreement', '/pages/account/agreements?type=user_agreement', '.document-body'], ['profile', '/pages/account/profile', '.profile-fields'], ['wallet', '/pages/account/wallet', '.balance-section']] as const) {
    const page = await open(route), shell = await until(() => child(page, ['app-shell', 'components\\/AppShell']), slug + '公共壳')
    await until(() => find([page, shell], ready), slug + '正文已加载')
    const scopes: Scope[] = [shell, page], frame = await measure(scopes, '.app-frame', game + ' ' + slug + '公共壳'), body = await measure(scopes, '.app-body', game + ' ' + slug + '正文', frame.color)
    assert.ok(frame.area.height >= Number(viewport?.windowHeight) - 1, '二级页背景没有铺满窗口')
    assert.equal(await find(scopes, '.bottom-nav'), null, '二级页不应重复挂主底栏')
    assert.equal(await child(page, ['bottom-nav', 'components\\/BottomNav']), null)
    await screenshot(game + '-' + slug + '-top'); await sdk().pageScrollTo(100000); await page.waitFor(180)
    const actualScroll = Number(await page.scrollTop()), after = await measure(scopes, '.app-body', game + ' ' + slug + '滚动后正文', frame.color)
    if (body.area.top + body.area.height > Number(viewport?.windowHeight) + 1) assert.ok(actualScroll > 0, slug + '长页面未发生真实滚动')
    assert.ok(after.area.left + after.area.width <= Number(viewport?.windowWidth) + 1, '二级页滚动后水平溢出')
    const image = await screenshot(game + '-' + slug + '-bottom'); sample(image, game + ' ' + slug + '滚动后背景', 4, Number(viewport?.windowHeight) - 20, frame.color)
    passed(game + '访客' + slug + '二级页壳与正文显式同色，滚动后连续且无主底栏')
    if (responsive && slug === 'wallet') await responsiveRules(page, shell, game)
  }
}
/** 将验收结果和真实能力边界写入独立证据目录，失败也保留当前步骤。 */
function report(): void {
  fs.mkdirSync(output, { recursive: true })
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify({ passed: !failure && restored, stage, project, viewport, checks, measurements, responsive_mode: responsive, scroll_only: scrollOnly, public_api: publicAPI, responsive_measurements: responsiveMeasurements, scroll_measurements: scrollMeasurements, scroll_retention_measurements: scrollRetentionMeasurements, pixel_samples: pixels, screenshots, version_checks: versionChecks, runtime_exceptions: exceptions, color_warnings: colorWarnings, original_game_preference_restored: restored, original_route: originalRoute, boundaries: ['仅访客官方微信模拟器布局验收；未注入页面业务状态、令牌、登录、支付、转账、后台或缓存写入。', '响应式附加项仅UI_TEST_RESPONSIVE=1开启；已有服务仅两主题各一个公开GET样本，不代表六文案/全部字号/四身份或交易验收，可用UI_TEST_PUBLIC_SERVICES=0略过样本。', '非零滚动保留使用官方SDK程序滚动、真实切标签及navigateBack；不代表手势惯性、帧率或真机流畅度。', '截图由官方 simulator_screenshot --optimize false 产生，仅解码采样，未修改原图。', '不替代真实设备、微信支付、退款、提现或实际资金到账验收。'], ...(failure ? { error: redact(failure instanceof Error ? failure.stack || failure.message : failure) } : {}) }, null, 2) + '\n')
}
/** 运行两个主题及主次页面检查，finally 恢复原主题与 SDK 校验函数并断开连接。 */
async function run(): Promise<void> {
  assert.match(endpoint, /^ws:\/\/127\.0\.0\.1:9423$/, '只连接既有9423工程，禁止另开模拟器')
  compatibilityCheck(); mini = await automator.connect({ wsEndpoint: endpoint })
  mini.on('exception', (error: unknown) => exceptions.push(redact(error)))
  mini.on('console', (event: unknown) => { if (/不支持颜色|unsupported color|invalid color/i.test(JSON.stringify(event))) colorWarnings.push('微信颜色告警') })
  viewport = await sdk().evaluate(() => wx.getWindowInfo()) as WindowInfo
  assert.ok([320, 390].includes(viewport.windowWidth), '只验收实际320或390宽窗口')
  if (process.env.UI_EXPECT_WIDTH) assert.equal(viewport.windowWidth, Number(process.env.UI_EXPECT_WIDTH))
  output = path.join(outputRoot, 'mini-' + viewport.windowWidth); fs.mkdirSync(output, { recursive: true })
  assert.equal(await sdk().evaluate(() => Boolean(wx.getStorageSync('jijie_token'))), false, '必须由访客开始；脚本不擅自登录或退出他人会话')
  originalGame = await sdk().evaluate(() => wx.getStorageSync('jijie_game')); preferenceRead = true
  originalRoute = (await sdk().currentPage())?.path || ''
  for (const game of ['delta', 'honor']) {
    if (scrollOnly) {
      const page=await open('/pages/main/index?tab=player&game_id='+encodeURIComponent(game)),player=await until(()=>child(page,['player-view','pages\\/main\\/player\\/player']),'滚动样本玩家视图')
      await until(async()=>(await children(player,['service-card','components\\/ServiceCard'])).length>0,'列表真实数据加载')
      const nav=await until(()=>child(page,['bottom-nav','components\\/BottomNav']),'唯一主底栏'),buttons=await nav.$$('.nav-item') as Element[];assert.equal(buttons.length,4)
      await mainScrollRetention(page,buttons,game);await responsiveService(game)
    } else {await mainTheme(game);await secondary(game);if(responsive&&process.env.UI_TEST_PUBLIC_SERVICES!=='0')await responsiveService(game)}
  }
  assert.deepEqual(exceptions, [], '微信运行时异常'); assert.deepEqual(colorWarnings, [], '微信颜色告警')
}
run().catch(async error => { failure = error; console.error(redact(error)); if (mini) try { await screenshot('failure') } catch { /* 原始失败优先保留，不伪造截图成功。 */ } }).finally(async () => {
  if (mini && preferenceRead) try { const game = typeof originalGame === 'string' && originalGame ? originalGame : 'delta'; await open('/pages/main/index?tab=me&game_id=' + encodeURIComponent(game)); assert.deepEqual(await sdk().evaluate(() => wx.getStorageSync('jijie_game')), originalGame, '原游戏缓存被修改'); restored = true } catch (error) { failure ||= error }
  SDK.prototype.checkVersion = originalCheckVersion
  try { await mini?.disconnect() } catch (error) { failure ||= error }
  report(); if (failure) process.exitCode = 1; else console.log('通过 ' + checks.length + ' 项，原游戏偏好未写入且已恢复浏览主题')
})
