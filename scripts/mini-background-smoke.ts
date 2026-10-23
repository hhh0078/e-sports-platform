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
const automator = require('miniprogram-automator') as { connect(options: { wsEndpoint: string }): Promise<MiniProgram> }
const SDK = (require('miniprogram-automator/out/MiniProgram') as { default: typeof MiniProgram }).default
const png = require('pngjs') as { PNG: { sync: { read(bytes: Buffer): ScreenshotPixels } } }
const originalCheckVersion = SDK.prototype.checkVersion
const checks: string[] = [], exceptions: string[] = [], colorWarnings: string[] = [], screenshots: string[] = []
const measurements: Measurement[] = [], versionChecks: Array<Record<string, unknown>> = []
const scrollMeasurements: Array<{ label: string; viewport_height: number; content_height: number; scroll_top: number; scroll_width: number }> = []
const pixels: Array<{ label: string; image: string; x: number; y: number; actual: number[]; expected: number[]; difference: number }> = []
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
/** 主页面连续真实点击四底栏，逐一检查保留同一 Page、显式背景及高度。 */
async function mainTheme(game: string): Promise<void> {
  const page = await open('/pages/main/index?tab=player&game_id=' + encodeURIComponent(game))
  const player = await until(() => child(page, ['player-view', 'pages\\/main\\/player\\/player']), '玩家视图'), picker = await until(() => child(player, ['game-picker', 'components\\/GamePicker']), '真实游戏目录')
  await until(async () => { const chosen = await picker.$('.game-choice.chosen'); return chosen && await chosen.attribute('id') === 'game-option-' + game }, '路由指定主题已实际选中')
  const nav = await until(() => child(page, ['bottom-nav', 'components\\/BottomNav']), '唯一底栏组件')
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
    assert.ok(Math.abs(scrollArea.height - region.area.height) <= 1, '滚动层未填满主视口')
    assert.ok(Math.abs(shell.area.height - Number(viewport?.windowHeight)) <= 1, '整页高度不等于真实窗口')
    await scroll.scrollTo(0, 0); await page.waitFor(120)
    if (tab === 'me') await personal(page, scroll, base, game, bottom.area)
    else { await screenshot(game + '-' + tab + '-top'); await scroll.scrollTo(0, 100000); await page.waitFor(180); assert.deepEqual(rgb(await scroll.style('background-color')), base); await screenshot(game + '-' + tab + '-bottom') }
    await scrollEvidence(scroll, game + ' ' + tab)
    passed(game + ' ' + tab + '保持主页面，公共层底色和高度连续，滚动后不横溢')
  }
  assert.deepEqual(await sdk().evaluate(() => wx.getStorageSync('jijie_game')), originalGame, '只浏览主题不能写入游戏偏好')
}
/** 核验个人中心封面真实渐变、上移身份与紧凑订单间距，保留充值和收入独立文案。 */
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
  assert.ok(walletRect.height <= 170, '个人中心钱包卡超过170px'); assert.match(walletText, /可提现/); assert.match(walletText, /充值余额（仅消费）/)
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
  }
}
/** 将验收结果和真实能力边界写入独立证据目录，失败也保留当前步骤。 */
function report(): void {
  fs.mkdirSync(output, { recursive: true })
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify({ passed: !failure && restored, stage, project, viewport, checks, measurements, scroll_measurements: scrollMeasurements, pixel_samples: pixels, screenshots, version_checks: versionChecks, runtime_exceptions: exceptions, color_warnings: colorWarnings, original_game_preference_restored: restored, original_route: originalRoute, boundaries: ['仅访客官方微信模拟器布局验收；未注入页面业务状态、令牌、登录、支付、转账、后台或缓存写入。', '截图由官方 simulator_screenshot --optimize false 产生，仅解码采样，未修改原图。', '不替代真实设备、微信支付、退款、提现或实际资金到账验收。'], ...(failure ? { error: redact(failure instanceof Error ? failure.stack || failure.message : failure) } : {}) }, null, 2) + '\n')
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
  for (const game of ['delta', 'honor']) { await mainTheme(game); await secondary(game) }
  assert.deepEqual(exceptions, [], '微信运行时异常'); assert.deepEqual(colorWarnings, [], '微信颜色告警')
}
run().catch(async error => { failure = error; console.error(redact(error)); if (mini) try { await screenshot('failure') } catch { /* 原始失败优先保留，不伪造截图成功。 */ } }).finally(async () => {
  if (mini && preferenceRead) try { const game = typeof originalGame === 'string' && originalGame ? originalGame : 'delta'; await open('/pages/main/index?tab=me&game_id=' + encodeURIComponent(game)); assert.deepEqual(await sdk().evaluate(() => wx.getStorageSync('jijie_game')), originalGame, '原游戏缓存被修改'); restored = true } catch (error) { failure ||= error }
  SDK.prototype.checkVersion = originalCheckVersion
  try { await mini?.disconnect() } catch (error) { failure ||= error }
  report(); if (failure) process.exitCode = 1; else console.log('通过 ' + checks.length + ' 项，原游戏偏好未写入且已恢复浏览主题')
})
