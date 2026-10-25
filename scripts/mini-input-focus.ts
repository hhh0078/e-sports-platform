/** 文件作用：仅在独立本地微信工程用官方SDK点击输入框，保存聚焦前后原始截图及几何证据；不保存配置、发布服务或创建资金单。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { InputElement, ScrollViewElement, TextareaElement } from '../frontend/node_modules/miniprogram-automator/out/Element'

/** evaluate仅使用微信公开窗口信息，不读取Vue、页面data或会话。 */
declare const wx: { getWindowInfo(): { windowWidth: number; windowHeight: number } }
/** 官方节点查询与真实文本输入的公开边界。 */
type Scope = Page | Element
type Editable = InputElement | TextareaElement
/** 原生节点的可见外框，不能用它替代内部占位文字像素验收。 */
interface Rectangle { left: number; top: number; width: number; height: number }
/** 每次截图对应完整窗口、滚动位置和同页标签／输入框。 */
interface Measurement { scene: string; phase: string; route: string; scroll_top: number; container_scroll_top?: number; window: { windowWidth: number; windowHeight: number }; nodes: Record<string, Rectangle>; screenshot?: string }
/** 一个输入场景只操作已发现的原生节点，不调用业务方法。 */
interface FocusCase { name: string; input: Editable; text: string; anchors: Record<string, Element>; adjacent?: Element; scroll?: ScrollViewElement; inPopup?: boolean }

const root = process.cwd(), api = 'http://127.0.0.1:18091/api'
const project = path.resolve(process.env.UI_TEST_PROJECT || 'frontend/dist/focus-test/mp-weixin'), endpoint = process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9424'
const expectedWidth = Number(process.env.UI_EXPECT_WIDTH || 390), output = path.join(root, 'output/input-focus/mini-' + expectedWidth)
const require = createRequire(path.join(root, 'frontend/package.json'))
const automator = require('miniprogram-automator') as { connect(options: { wsEndpoint: string }): Promise<MiniProgram> }
const checks: string[] = [], screenshots: string[] = [], screenshotErrors: string[] = [], exceptions: string[] = [], measurements: Measurement[] = []
let mini: MiniProgram | undefined, failure: unknown, stage = '本地工程预检', loggedOut = false

/** 取得唯一官方SDK连接，不自动重连到其它工程。 */
function sdk(): MiniProgram { assert.ok(mini); return mini }
/** 隐去异常中可能出现的认证资料，报告不保存用户输入值。 */
function redacted(value: unknown): string { return String(value instanceof Error ? value.message : value).replace(/(token|secret|openid|authorization|api_v3_key)["'\s:=]+[^\s,}"']+/gi, '$1=[REDACTED]').slice(0, 1200) }
/** 等待明确的公开界面条件，不重试保存或资金提交。 */
async function until<T>(read: () => Promise<T | false | null | undefined>, label: string, timeout = 18000): Promise<T> {
  const start = Date.now()
  while (Date.now() - start < timeout) { const value = await read(); if (value) return value; await new Promise(resolve => setTimeout(resolve, 150)) }
  throw new Error('等待超时：' + label)
}
/** 获取唯一固定本地API的只读数据，拒绝重定向和非开发服务。 */
async function localRead<T>(route: '/bootstrap' | '/dev/accounts'): Promise<T> {
  const response = await fetch(api + route, { redirect: 'error' }); assert.equal(response.status, 200, '本地只读接口 ' + route)
  const result = await response.json() as { data: T }; return result.data
}
/** 导航到真实编译路由，不注入页面内部状态。 */
async function open(route: string): Promise<Page> { stage = route; console.log('CHECK ' + route); const page = await sdk().reLaunch(route); assert.ok(page); assert.equal(page.path, route.split('?')[0].slice(1)); return page }
/** 兼容现有uni-app短名称和编译路径，自顶向下跨已知公共组件查找。 */
async function child(scope: Scope, names: string[]): Promise<Element | null> { for (const name of names) { const item = await scope.$(name); if (item) return item } return null }
/** 仅遍历本轮需要的已编译组件边界，避免读取私有data或实例。 */
async function find(scope: Scope, selector: string, depth = 0): Promise<Element | null> {
  const direct = await scope.$(selector); if (direct) return direct
  if (depth >= 4) return null
  for (const names of [['app-shell', 'components\\/AppShell'], ['funds-form', 'components\\/FundsForm'], ['finance-popup', 'components\\/FinancePopup'], ['uni-popup', 'node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup'], ['game-selector', 'components\\/GameSelector']]) {
    const component = await child(scope, names); if (component) { const found = await find(component, selector, depth + 1); if (found) return found }
  }
  return null
}
/** 按当前可见业务文案找按钮，剔除官方图标私用字符。 */
async function button(scope: Scope, label: string): Promise<Element> { return until(async () => { for (const item of await scope.$$('button') as Element[]) if ((await item.text()).replace(/[\uE000-\uF8FF]/g, '').trim() === label) return item; return null }, '按钮 ' + label) }
/** 读取真实节点矩形，不将CSS声明当作渲染结果。 */
async function rect(item: Element): Promise<Rectangle> { const [offset, size] = await Promise.all([item.offset(), item.size()]); return { left: Number(offset.left), top: Number(offset.top), width: Number(size.width), height: Number(size.height) } }
/** 通过页面或常驻主容器的公开滚动能力，把输入置于键盘上方的操作区域。 */
async function show(page: Page, item: Element, scroll?: ScrollViewElement): Promise<void> {
  const area = await rect(item), current = scroll ? Number(await scroll.property('scrollTop')) : Number(await page.scrollTop())
  const next = Math.max(0, current + area.top - 150)
  if (scroll) await scroll.scrollTo(0, next); else await sdk().pageScrollTo(next)
  await page.waitFor(250)
}
/** 使用微信hideKeyboard完成失焦，不伪造页面blur事件。 */
async function blur(page: Page): Promise<void> { await sdk().callWxMethod('hideKeyboard'); await page.waitFor(300) }
/** 保存当前公开几何和原始SDK截图，不读取或写入任何实际秘密。 */
async function capture(page: Page, scenario: FocusCase, phase: string): Promise<Measurement> {
  stage = scenario.name + ' / ' + phase
  const nodes: Record<string, Rectangle> = {}
  for (const [label, item] of Object.entries(scenario.anchors)) { nodes[label] = await rect(item); assert.ok(nodes[label].width > 0 && nodes[label].height > 0, '节点未显示 ' + label) }
  const window = await sdk().evaluate(() => wx.getWindowInfo()) as Measurement['window']
  assert.equal(window.windowWidth, expectedWidth, '必须实际切换微信模拟器宽度')
  const value: Measurement = { scene: scenario.name, phase, route: page.path, scroll_top: Number(await page.scrollTop()), ...(scenario.scroll ? { container_scroll_top: Number(await scenario.scroll.property('scrollTop')) } : {}), window, nodes }
  const file = path.join(output, scenario.name + '-' + phase + '.png')
  try { await sdk().screenshot({ path: file }); assert.ok(fs.statSync(file).size > 100, 'SDK截图为空'); value.screenshot = file; screenshots.push(file) } catch (error) { screenshotErrors.push(stage + '：' + redacted(error)) }
  measurements.push(value); console.log('MEASURE ' + stage); return value
}
/** 只断言外框及同页节点相对关系稳定，页面整体键盘避让位移单独保留原始数据。 */
function sameGeometry(before: Measurement, after: Measurement): void {
  const first = Object.keys(before.nodes)[0]
  for (const key of Object.keys(before.nodes)) {
    const a = before.nodes[key], b = after.nodes[key]
    for (const dimension of ['width', 'height'] as const) assert.ok(Math.abs(a[dimension] - b[dimension]) <= 1, after.scene + ' / ' + after.phase + ' 外框' + dimension + '变化 ' + key)
    assert.ok(Math.abs((a.top - before.nodes[first].top) - (b.top - after.nodes[first].top)) <= 1, after.scene + ' / ' + after.phase + ' 同页标签相对位置变化 ' + key)
  }
}
/** 原生tap才作为聚焦动作；input方法仅填写测试草稿，并在填写后再次tap保证真实点击路径。 */
async function exercise(page: Page, scenario: FocusCase): Promise<void> {
  await scenario.input.input(''); await blur(page)
  if (!scenario.inPopup) await show(page, scenario.input, scenario.scroll)
  const before = await capture(page, scenario, 'before-empty')
  await scenario.input.tap(); await page.waitFor(350); sameGeometry(before, await capture(page, scenario, 'tap-empty'))
  await scenario.input.input(scenario.text); assert.equal(await scenario.input.value(), scenario.text)
  // 有内容时搜索清除按钮会正常占宽；只比较同一内容状态的聚焦几何，不能把该变化误判成文字上跳。
  await blur(page); const filled = await capture(page, scenario, 'filled-before-tap')
  await scenario.input.tap(); await page.waitFor(350); sameGeometry(filled, await capture(page, scenario, 'tap-filled'))
  if (scenario.adjacent) { await scenario.adjacent.tap(); await page.waitFor(350); sameGeometry(filled, await capture(page, scenario, 'tap-adjacent')) }
  await blur(page); sameGeometry(filled, await capture(page, scenario, 'after-blur'))
  checks.push(scenario.name + '：真实tap、草稿输入、相邻切换与hideKeyboard，外框及相对关系稳定；内部字形需目视截图')
}
/** 按实际标签找到普通表单字段，避免依赖输入框在整页的脆弱序号。 */
async function field(page: Page, label: string): Promise<{ row: Element; label: Element; input: Editable }> {
  return until(async () => {
    for (const row of await page.$$('.field') as Element[]) {
      const caption = await row.$('.field-label')
      if (caption && (await caption.text()).trim().startsWith(label)) { const input = await row.$('input') || await row.$('textarea'); if (input) return { row, label: caption, input: input as Editable } }
    }
    return null
  }, '字段 ' + label)
}
/** 通过协议勾选与开发体验卡片登录真实本地管理员，绝不注入token。 */
async function login(nickname: string): Promise<void> {
  const page = await open('/pages/account/login')
  await until(async () => { const item = await page.$('.login-primary'); return item && await item.property('disabled') !== true ? item : null }, '登录资料准备')
  await (await until(() => page.$('.consent-checkbox-label'), '主动勾选两协议')).tap()
  await (await until(() => page.$('.dev-toggle'), '开发体验')).tap()
  await (await until(async () => { for (const card of await page.$$('.account-card') as Element[]) { const name = await card.$('.account-name'); if (name && (await name.text()).trim() === nickname) return card } return null }, '本地管理员体验身份')).tap()
  await until(async () => { if ((await sdk().currentPage())?.path === 'pages/main/index') return true; const later = await page.$('.finish-later'); if (later) await later.tap(); const error = await page.$('.login-error'); if (error) throw new Error(await error.text()); return false }, '登录回跳')
}
/** 每次结束通过真实退出入口撤销管理员会话，不保留测试输入或身份。 */
async function logout(): Promise<void> {
  const page = await open('/pages/account/profile')
  const current = await until(async () => { const exit = await page.$('.logout-button'); return exit ? { exit } : await page.$('.guest-note') ? { exit: null } : null }, '真实退出入口或访客状态')
  if (current.exit) { await show(page, current.exit); await current.exit.tap(); await until(async () => (await sdk().currentPage())?.path === 'pages/main/index', '退出回到主页') }
  loggedOut = true
}
/** 覆盖截图所示支付配置，公开配置只改本次草稿；秘密字段必须本来为空且从不保存。 */
async function payment(): Promise<void> {
  const page = await open('/pages/manage/settings?tab=payment')
  await until(() => page.$('.payment-status'), '平台支付配置读取完成')
  assert.equal(await page.$('.error-box'), null)
  const first = await field(page, '微信支付商户号'), next = await field(page, '商户 API 证书序列号')
  const visible = await page.$$('.payment-panel input') as InputElement[]
  for (const input of visible) await input.input('')
  await exercise(page, { name: 'payment-text', input: first.input, text: 'focus-demo-123456', adjacent: next.input, anchors: { first_label: first.label, first_input: first.input, second_label: next.label, second_input: next.input } })
  const secret = await field(page, 'APIv3 密钥'), pem = await field(page, '商户私钥'), publicKey = await field(page, '微信支付平台公钥')
  assert.equal(await secret.input.value(), ''); assert.equal(await pem.input.value(), ''); assert.equal(await publicKey.input.value(), '')
  await exercise(page, { name: 'payment-password', input: secret.input, text: 'InputFocusDemoOnly000000000000000', adjacent: pem.input, anchors: { password_label: secret.label, password_input: secret.input, pem_label: pem.label, pem_input: pem.input } })
  await exercise(page, { name: 'payment-textarea', input: pem.input, text: '输入框验收草稿，不保存。\n第二行用于核对多行布局。', adjacent: publicKey.input, anchors: { pem_label: pem.label, pem_input: pem.input, public_label: publicKey.label, public_input: publicKey.input } })
}
/** 资金表单只打开、编辑、关闭；禁止点击确认或创建测试付款。 */
async function wallet(): Promise<void> {
  const page = await open('/pages/account/wallet'), entry = await until(async () => { const value = await page.$('.deposit-main .text-btn'); return value && await value.property('disabled') !== true ? value : null }, '钱包押金入口')
  await show(page, entry); await entry.tap()
  const input = await until(() => find(page, '.funds-amount-input'), '真实金额输入') as InputElement
  const note = await until(() => find(page, '.funds-form-note'), '金额说明'), box = await until(() => find(page, '.funds-amount-field'), '金额外框')
  await exercise(page, { name: 'wallet-amount', input, text: '12345.67', inPopup: true, anchors: { description: note, field: box, input } })
  await (await until(() => find(page, '.finance-popup-close'), '关闭金额弹框')).tap(); await page.waitFor(350)
}
/** 发布服务只保留页面草稿，覆盖普通标题、色值与共享游戏搜索，绝不发布。 */
async function publish(): Promise<void> {
  const page = await open('/pages/services/publish'), title = await field(page, '服务标题'), price = await field(page, '价格（元）')
  await exercise(page, { name: 'service-title', input: title.input, text: '输入框布局验收，不发布', adjacent: price.input, anchors: { title_label: title.label, title: title.input, price_label: price.label, price: price.input } })
  const add = await button(page, '添加文字'); await show(page, add); await add.tap()
  const color = await until(() => page.$('.color-code'), '封面色值输入') as InputElement, editor = await until(() => page.$('.label-index'), '文案编号')
  await exercise(page, { name: 'service-color', input: color, text: '#ffffff', anchors: { editor_label: editor, color } })
  const trigger = await until(() => find(page, '.selector-trigger'), '共享游戏选择'); await show(page, trigger); await trigger.tap()
  const search = await until(() => find(page, '.catalog-input'), '弹层搜索输入') as InputElement, heading = await until(() => find(page, '.catalog-heading'), '游戏弹层标题')
  await exercise(page, { name: 'game-search', input: search, text: '三角', inPopup: true, anchors: { heading, input: search } })
  await (await until(() => find(page, '.selector-close'), '关闭游戏选择')).tap(); await page.waitFor(350)
}
/** 首页搜索位于常驻scroll-view，独立记录它的滚动位置而不误作页面滚动。 */
async function homeSearch(): Promise<void> {
  const page = await open('/pages/main/index?tab=player'), player = await until(() => child(page, ['player-view', 'pages\\/main\\/player\\/player']), '玩家业务视图')
  const tools = await until(() => player.$('.service-tools'), '首页搜索入口'); await (await button(tools, '搜索')).tap()
  const input = await until(() => player.$('.search-input'), '首页搜索框') as InputElement, row = await until(() => player.$('.search-row'), '首页搜索行'), toolbar = await until(() => player.$('.service-toolbar'), '服务列表标题')
  const scrolls = await page.$$('.main-scroll') as ScrollViewElement[]; assert.equal(scrolls.length, 4)
  await exercise(page, { name: 'home-search', input, text: '本地输入验收', scroll: scrolls[0], anchors: { toolbar, search_row: row, input } })
}
/** 运行前验证固定独立产物、开发服务及实际机型；一个运行只验收一个真实模拟器宽度。 */
async function run(): Promise<void> {
  assert.ok([320, 390].includes(expectedWidth), '仅接受320或390真实模拟器宽度')
  const compiled = fs.readFileSync(path.join(project, 'api/client.js'), 'utf8'); assert.ok(compiled.includes(api), '必须先构建独立focus-test本地工程'); assert.ok(!compiled.includes('https://game.ai.58it.cn'), '输入验收工程不得指向生产域名')
  assert.equal((await localRead<{ dev_mode: boolean }>('/bootstrap')).dev_mode, true)
  const admin = (await localRead<Array<{ id: string; nickname: string }>>('/dev/accounts')).find(user => user.id === 'admin'); assert.ok(admin)
  fs.mkdirSync(output, { recursive: true }); mini = await automator.connect({ wsEndpoint: endpoint }); mini.on('exception', (error: unknown) => exceptions.push(redacted(error)))
  const window = await mini.evaluate(() => wx.getWindowInfo()) as Measurement['window']; assert.equal(window.windowWidth, expectedWidth)
  await logout(); loggedOut = false; await login(admin.nickname)
  await payment(); await wallet(); await publish(); await homeSearch(); await logout()
  assert.deepEqual(exceptions, [], '输入操作期间出现运行时异常'); assert.deepEqual(screenshotErrors, [], '原始截图未完整保存，不能完成视觉验收')
}

run().catch(async error => { failure = error; console.error(redacted(error)); if (mini) try { const file = path.join(output, 'failure.png'); await mini.screenshot({ path: file }); screenshots.push(file) } catch { /* 保留原始失败原因，不用失败截图错误覆盖它。 */ } process.exitCode = 1 }).finally(async () => {
  if (mini) { try { await sdk().callWxMethod('hideKeyboard'); await logout() } catch (error) { failure ||= error; process.exitCode = 1 } await mini.disconnect() }
  fs.mkdirSync(output, { recursive: true }); fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify({ file_purpose: '本地微信输入聚焦外框和原始像素证据；不代表内部字形已经人工验收', passed: !failure, stage, project, api, endpoint, expected_width: expectedWidth, checks, measurements, screenshots, screenshot_errors: screenshotErrors, runtime_exceptions: exceptions, final_session_logged_out: loggedOut, visual_review_required: true, boundaries: ['Element.tap是本脚本聚焦动作；input方法只填写假草稿，不以程序输入冒充物理点击或真实手机键盘。', '外框不变不能证明占位文字像素未移动；必须逐组目视before-empty、tap-empty、tap-filled、tap-adjacent及after-blur原始截图。', '保留page.scrollTop、主容器scrollTop和窗口变化；键盘合法避让与内部文字跳动要分别判断。', '只在18091开发后端使用真实协议勾选及开发体验UI登录，未保存支付设置、发布服务、创建订单或付款；手机键盘效果需真机另验。'], ...(failure ? { error: redacted(failure) } : {}) }, null, 2) + '\n')
})
