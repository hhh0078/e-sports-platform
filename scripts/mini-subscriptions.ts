/** 文件作用：通过微信官方 SDK 验收六类订阅入口、身份隔离、受控原生授权和真实浮层；仅运行指定本地工程，模拟授权不构成真实微信收件证明。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { AuthSession, Service } from '../frontend/src/types/models'
import type { SubscriptionAuthorizationStatus, SubscriptionDelivery, SubscriptionOverview, SubscriptionScene, SubscriptionSettings, SubscriptionSettingsBody } from '../frontend/src/types/subscriptions'

/** 用于验收的真实窗口与节点几何，不读取页面私有状态。 */
interface Rectangle { left: number; top: number; width: number; height: number }
/** 原生授权替身只记录公开场景所使用的假模板，不能写入令牌或业务数据。 */
interface NativeCall { outcome: string; templates: string[] }
/** 官方 SDK 可以查询的公开页面或组件作用域。 */
type Scope = Page | Element
/** 本地体验身份与期望的订阅场景。 */
interface TestIdentity { id: string; nickname: string; scenes: SubscriptionScene[] }

const root = process.cwd(), require = createRequire(path.join(root, 'frontend/package.json'))
const api = process.env.UI_TEST_API || 'http://127.0.0.1:18091/api'
const project = path.resolve(process.env.UI_TEST_PROJECT || 'frontend/dist/dev/mp-weixin')
const endpoint = process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9423'
const output = path.resolve(process.env.UI_TEST_OUTPUT || `output/wechat-subscriptions/mini-${process.env.UI_EXPECT_WIDTH || '390'}`)
const cli = '/Applications/wechatwebdevtools.app/Contents/MacOS/wechatide', runFile = promisify(execFile)
const automator = require('miniprogram-automator') as { connect(options: { wsEndpoint: string }): Promise<MiniProgram> }
const SDK = (require('miniprogram-automator/out/MiniProgram') as { default: typeof MiniProgram }).default
const originalVersionCheck = SDK.prototype.checkVersion
const scenes: SubscriptionScene[] = ['provider_order', 'provider_refund', 'buyer_accepted', 'club_member', 'club_dispute', 'admin_club']
const labels: Record<SubscriptionScene, string> = { provider_order: '服务新订单', provider_refund: '退款申请', buyer_accepted: '打手接单', club_member: '入会申请', club_dispute: '俱乐部订单争议', admin_club: '俱乐部待认证' }
const identities: TestIdentity[] = [
  { id: 'player', nickname: '云间旅人', scenes: scenes.slice(0, 3) },
  { id: 'worker', nickname: '北辰', scenes: scenes.slice(0, 3) },
  { id: 'clubowner', nickname: '山海主理人', scenes: scenes.slice(0, 5) },
  { id: 'admin', nickname: '开机电竞管理员', scenes: [...scenes.slice(0, 3), 'admin_club'] },
]
const checks: string[] = [], exceptions: string[] = [], warnings: string[] = [], screenshots: string[] = [], nativeCalls: NativeCall[] = [], versionChecks: string[] = []
const measurements: Array<{ label: string; selector: string; rectangle: Rectangle }> = []
const mocks = new Set<string>(), sessions = new Map<string, AuthSession>()
let mini: MiniProgram | undefined, viewport: { windowWidth: number; windowHeight: number } | undefined
let originalSettings: SubscriptionSettingsBody | undefined, stage = '初始化', failure: unknown, cleanupFailure: string | undefined
let originalDeliveries = new Set<string>(), deliveryDelta: Array<Pick<SubscriptionDelivery, 'scene' | 'status' | 'error_code'>> = []

/** 受控页面表达式仅取得公开窗口，不访问 Vue、页面 data、令牌或用户私有资料。 */
declare const wx: { getWindowInfo(): { windowWidth: number; windowHeight: number } }
/** 官方 SDK 公开绑定仅记录本轮原生替身被真实按钮调用。 */
declare function subscriptionSmokeNative(call: NativeCall): void

/** 取得已建立的唯一官方 SDK 连接。 */
function sdk(): MiniProgram { assert.ok(mini, 'SDK 尚未连接'); return mini }
/** 报告只保留脱敏错误摘要，不保存后端会话或原生私密资料。 */
function redact(error: unknown): string { return String(error instanceof Error ? error.message : error).replace(/(token|secret|openid|authorization)["'\s:=]+[^\s,}"']+/gi, '$1=[REDACTED]').slice(0, 1400) }
/** 按真实页面条件等待，不重试业务提交。 */
async function until<T>(read: () => Promise<T | null | undefined | false>, label: string, timeout = 18000): Promise<T> {
  const start = Date.now()
  while (Date.now() - start < timeout) {
    try { const value = await read(); if (value) return value } catch (error) { if (!/rawPath|getPageMetaByWebviewId/.test(String(error))) throw error }
    await new Promise(resolve => setTimeout(resolve, 150))
  }
  throw new Error('等待超时：' + label)
}
/** 官方工具仅在版本元数据缺失导致 split 异常时读取真实基础库版本，禁止直接跳过校验。 */
function compatibilityCheck(): void {
  SDK.prototype.checkVersion = async function checkedVersion(this: MiniProgram): Promise<void> {
    try { await originalVersionCheck.call(this) } catch (error) {
      if (!(error instanceof TypeError) || !/split/.test(error.message)) throw error
      const info = await this.callWxMethod('getSystemInfoSync') as { SDKVersion?: string }
      assert.match(String(info.SDKVersion), /^\d+\.\d+\.\d+/)
      const [major, minor, patch] = String(info.SDKVersion).split('.').map(Number)
      assert.ok(major > 2 || major === 2 && (minor > 7 || minor === 7 && patch >= 3), '真实基础库低于官方自动化最低版本')
      versionChecks.push('工具元数据缺失时已核验真实基础库 ' + info.SDKVersion)
    }
  }
}
/** 独立本地会话只用于核对和恢复配置，不向小程序写入任何 token。 */
async function call<T>(route: string, token = '', method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(api + route, { method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) })
  const envelope = await response.json() as { data?: T; error?: string }
  assert.ok(response.ok, route + ' HTTP ' + response.status + ' ' + (envelope.error || ''))
  return envelope.data as T
}
/** 未授权身份访问管理员接口必须明确拒绝，读取失败不视为权限通过。 */
async function forbidden(route: string, token: string): Promise<void> {
  const response = await fetch(api + route, { headers: { Authorization: 'Bearer ' + token } })
  assert.equal(response.status, 403, route + ' 未拒绝普通身份')
}
/** 将只读附加目录剔除，恢复接口只接收约定的配置主体。 */
function settingsBody(settings: SubscriptionSettings): SubscriptionSettingsBody { return { enabled: settings.enabled, scenes: settings.scenes.map(item => ({ scene: item.scene, enabled: item.enabled, template_id: item.template_id, fields: { ...item.fields } })) } }
/** 通过原生导航打开当前工程页面，工具元数据迟到时仅等待读取、不重复发导航。 */
async function open(url: string, restart = false): Promise<Page> {
  stage = '页面 ' + url; console.log('CHECK ' + stage)
  try { const page = restart ? await sdk().reLaunch(url) : await sdk().navigateTo(url); assert.ok(page); assert.equal(page.path, url.split('?')[0].slice(1)); return page }
  catch (error) { if (!/rawPath|getPageMetaByWebviewId/.test(String(error))) throw error; return until(async () => { const page = await sdk().currentPage(); return page?.path === url.split('?')[0].slice(1) ? page : null }, '导航元数据') }
}
/** 查找编译后的公开组件，兼容短标签和官方自动化输出的路径标签。 */
async function component(scope: Scope, names: string[]): Promise<Element | null> { for (const name of names) { const found = await scope.$(name); if (found) return found } return null }
/** 在公开页面与组件边界查找节点，不调用内部方法或 setData。 */
async function find(page: Page, selector: string): Promise<Element | null> {
  const direct = await page.$(selector); if (direct) return direct
  for (const name of ['subscribe-reminder', 'components\\/SubscribeReminder', 'finance-popup', 'components\\/FinancePopup', 'empty-state', 'components\\/EmptyState']) {
    const child = await page.$(name); if (!child) continue
    const own = await child.$(selector); if (own) return own
    const popup = await component(child, ['finance-popup', 'components\\/FinancePopup'])
    if (popup) { const nested = await popup.$(selector); if (nested) return nested }
  }
  return null
}
/** 查找真实按钮文本，移除图标字体后比较，不能注入业务事件。 */
async function button(page: Page, label: string): Promise<Element> { return until(async () => { for (const item of await page.$$('button') as Element[]) if ((await item.text()).replace(/[\uE000-\uF8FF]/g, '').trim() === label) return item; return null }, '按钮 ' + label) }
/** 查询真实节点尺寸，隐藏节点不能当作已展示通过。 */
async function rectangle(item: Element): Promise<Rectangle> { const [offset, size] = await Promise.all([item.offset(), item.size()]); return { left: Number(offset.left), top: Number(offset.top), width: Number(size.width), height: Number(size.height) } }
/** 校验可见控件横向不溢出，并留下当前窗口的真实测量值。 */
async function within(page: Page, selector: string, label: string): Promise<void> {
  const items = await page.$$(selector) as Element[]
  assert.ok(items.length > 0, label + '缺少实际节点')
  for (const item of items) { const area = await rectangle(item); assert.ok(area.width > 0 && area.left >= -1 && area.left + area.width <= Number(viewport?.windowWidth) + 1, label + '横向溢出'); measurements.push({ label, selector, rectangle: area }) }
}
/** 等待按钮真实解除禁用，覆盖原生回包和后端授权上报异步链。 */
async function enabled(page: Page, selector: string): Promise<Element> { return until(async () => { const item = await find(page, selector); if (!item) return null; const disabled = await item.property('disabled'); return disabled !== true && disabled !== 'true' ? item : null }, selector + '可操作') }
/** 使用微信开发者工具官方原始截图，不修改、拼接或压缩像素。 */
async function record(label: string, name?: string): Promise<void> {
  checks.push(label); console.log('PASS ' + label)
  if (name) { await new Promise(resolve => setTimeout(resolve, 400)); const destination = path.join(output, name + '.png'); await runFile(cli, ['-c', 'Codex', 'simulator_screenshot', '--project', project, '--path', destination, '--optimize', 'false'], { timeout: 25000 }); screenshots.push(path.basename(destination)) }
}
/** 通过协议勾选与真实开发体验身份按钮登录，不注入会话。 */
async function login(identity: TestIdentity): Promise<void> {
  const page = await open('/pages/account/login', true)
  await (await until(() => page.$('.consent-line label'), '协议主动勾选')).tap()
  await (await button(page, '开发体验')).tap()
  await (await until(async () => { for (const account of await page.$$('.account-card') as Element[]) if ((await account.text()).includes(identity.nickname)) return account; return null }, identity.nickname + '身份按钮')).tap()
  await until(async () => { if ((await sdk().currentPage())?.path === 'pages/main/index') return true; const later = await page.$('.finish-later'); if (later) await later.tap(); return false }, '登录回跳')
}
/** 最终从可见资料页真正退出，恢复访客并撤销本轮客户端体验会话。 */
async function logout(): Promise<void> {
  if (!mini) return
  const page = await open('/pages/account/profile', true)
  await sdk().pageScrollTo(100000)
  const status = await until(async () => { const exit = await page.$('.logout-button'); if (exit) return { exit }; return await page.$('.guest-note') ? { exit: null } : null }, '真实退出入口或访客资料空态')
  if (status.exit) { await status.exit.tap(); await until(async () => (await sdk().currentPage())?.path === 'pages/main/index', '退出后访客主页') }
  const guest = await open('/pages/account/subscriptions', true)
  await until(async () => { const empty = await component(guest, ['empty-state', 'components\\/EmptyState']); return empty && (await empty.text()).includes('登录后设置微信提醒') }, '访客提醒空态')
}
/** 仅替代官方订阅接口的本次返回；记录真实按钮调用，既不触发实际授权也不发送微信通知。 */
async function nativeOutcome(outcome: SubscriptionAuthorizationStatus): Promise<void> {
  await sdk().mockWxMethod('requestSubscribeMessage', (options: { tmplIds: string[]; success?: (result: Record<string, string>) => void; fail?: (error: { errMsg: string }) => void; complete?: (result: unknown) => void }, status: string) => {
    subscriptionSmokeNative({ outcome: status, templates: options.tmplIds })
    if (status === 'fail') { const result = { errMsg: 'requestSubscribeMessage:fail 本地SDK受控失败' }; options.fail?.(result); options.complete?.(result); return }
    const result: Record<string, string> = { errMsg: 'requestSubscribeMessage:ok' }
    for (const id of options.tmplIds) result[id] = status
    options.success?.(result); options.complete?.(result)
  }, outcome)
  mocks.add('requestSubscribeMessage')
}
/** 从共享入口展开真正浮层，并检查面板和关闭入口均处于屏幕内。 */
async function reminderPopup(page: Page, name: string): Promise<void> {
  await (await until(() => find(page, '.subscribe-entry'), '共享订阅入口')).tap()
  await enabled(page, '.subscribe-confirm')
  const sheet = await until(async () => { const found = await find(page, '.finance-popup-sheet'); return found && (await rectangle(found)).height > 0 ? found : null }, '真实订阅浮层')
  const area = await rectangle(sheet)
  assert.ok(area.width > 0 && area.height > 0 && area.left >= -1 && area.top >= -1 && area.left + area.width <= Number(viewport?.windowWidth) + 1 && area.top + area.height <= Number(viewport?.windowHeight) + 1, '订阅浮层超出实际窗口')
  measurements.push({ label: '真实订阅浮层', selector: '.finance-popup-sheet', rectangle: area })
  await record('共享订阅说明为真实浮层，窄屏和关闭入口正常', name)
}
/** 核对页面只展示当前角色可操作的场景，后台模板配置不混入个人页。 */
async function rolePage(identity: TestIdentity): Promise<void> {
  await login(identity)
  const page = await open('/pages/account/subscriptions')
  const rows = await until(async () => { const found = await page.$$('.subscription-row') as Element[]; return found.length === identity.scenes.length ? found : null }, identity.id + '角色场景数')
  const overview = await call<SubscriptionOverview>('/subscriptions', sessions.get(identity.id)?.token)
  assert.deepEqual(overview.scenes.map(item => item.scene).sort(), [...identity.scenes].sort())
  for (const scene of identity.scenes) assert.ok((await Promise.all(rows.map(item => item.text()))).some(text => text.includes(labels[scene])), '缺少角色场景 ' + scene)
  assert.equal((await page.$$('input')).length, 0, '个人设置不得展示模板输入')
  await within(page, '.subscription-row', identity.id + '场景行')
  await record(identity.id + '仅展示本人身份可用的 ' + identity.scenes.length + ' 类场景', identity.id + '-settings')
  if (identity.id !== 'admin') {
    for (const route of ['/admin/subscriptions/settings', '/admin/subscriptions/deliveries']) await forbidden(route, sessions.get(identity.id)?.token || '')
    const denied = await open('/pages/manage/subscriptions')
    await until(async () => { const empty = await component(denied, ['empty-state', 'components\\/EmptyState']); return empty && (await empty.text()).includes('仅平台管理员') }, '非平台管理员拒绝配置')
    assert.equal((await denied.$$('input')).length, 0)
    await record(identity.id + '不能访问模板配置或发送记录')
  }
}
/** 记录后端发送状态增量，不把客户端授权成功当成实际收件或发送成功。 */
async function snapshotDeliveries(): Promise<SubscriptionDelivery[]> { const result = await call<{ items: SubscriptionDelivery[] }>('/admin/subscriptions/deliveries?limit=100', sessions.get('admin')?.token); return result.items }
/** 执行当前设备宽度的完整六场景验收；机型和自动化端口由调用者统一安排。 */
async function run(): Promise<void> {
  assert.equal(api, 'http://127.0.0.1:18091/api', '写入式验收只允许明确本地API')
  assert.equal(project, path.join(root, 'frontend/dist/dev/mp-weixin'), '验收只允许当前dist/dev工程')
  assert.equal(endpoint, 'ws://127.0.0.1:9423', '验收只允许已由主流程安排的9423连接')
  const compiled = fs.readFileSync(path.join(project, 'api/client.js'), 'utf8')
  assert.ok(compiled.includes(api) && !compiled.includes('https://game.ai.58it.cn/api'), '编译包API必须明确指向本地，禁止生产写入')
  assert.equal((await call<{ dev_mode: boolean }>('/bootstrap')).dev_mode, true)
  for (const identity of identities) sessions.set(identity.id, await call<AuthSession>('/auth/dev', '', 'POST', { user_id: identity.id }))
  originalSettings = settingsBody(await call<SubscriptionSettings>('/admin/subscriptions/settings', sessions.get('admin')?.token))
  originalDeliveries = new Set((await snapshotDeliveries()).map(item => item.id))
  const marker = Date.now().toString(36)
  const testSettings: SubscriptionSettingsBody = { enabled: true, scenes: scenes.map(scene => ({ scene, enabled: true, template_id: 'local_sdk_only_' + marker + '_' + scene, fields: { thing1: 'title', time2: 'time' } })) }
  await call('/admin/subscriptions/settings', sessions.get('admin')?.token, 'PATCH', testSettings)
  fs.mkdirSync(output, { recursive: true }); compatibilityCheck()
  mini = await automator.connect({ wsEndpoint: endpoint })
  mini.on('exception', (entry: unknown) => exceptions.push(redact(entry)))
  mini.on('console', (entry: unknown) => { if (/不支持颜色|unsupported color/i.test(JSON.stringify(entry))) warnings.push('颜色兼容告警') })
  await mini.exposeFunction('subscriptionSmokeNative', (entry: NativeCall) => nativeCalls.push(entry))
  viewport = await mini.evaluate(() => wx.getWindowInfo()) as typeof viewport
  assert.ok(viewport && [320, 390].includes(viewport.windowWidth), '仅验收320或390宽度，脚本不自行改机型')
  if (process.env.UI_EXPECT_WIDTH) assert.equal(viewport.windowWidth, Number(process.env.UI_EXPECT_WIDTH))
  await login(identities[0]); await logout(); await record('访客可正常展示微信提醒空态，无后台配置', 'guest')
  for (const identity of identities) await rolePage(identity)
  let page = await open('/pages/manage/subscriptions')
  await until(() => page.$('.subscriptions-master'), '管理员独立配置')
  assert.equal((await page.$$('.subscriptions-scene')).length, 6)
  await within(page, '.subscriptions-master', '管理员主开关')
  await within(page, '.subscriptions-scene-heading', '管理员六类配置')
  await record('平台管理员独立页面显示六类模板配置', 'admin-settings')
  await (await button(page, '发送记录')).tap()
  await until(() => page.$('.subscriptions-delivery-tools'), '独立发送日志页签')
  await record('管理员可查看独立发送记录，空记录正常显示', 'admin-deliveries')
  await sdk().mockWxMethod('request', (options: { fail?: (error: { errMsg: string }) => void; complete?: (error: { errMsg: string }) => void }) => { const result = { errMsg: 'request:fail 本地冷启动错误验收' }; options.fail?.(result); options.complete?.(result) })
  mocks.add('request')
  page = await open('/pages/manage/subscriptions', true)
  await until(() => page.$('.subscriptions-admin-error'), '管理员冷加载错误提示')
  assert.ok((await (await until(() => page.$('.subscriptions-admin-error'), '可见错误容器')).text()).includes('重新加载'))
  await record('管理员冷加载失败提供可见错误与重试入口', 'admin-cold-error')
  await sdk().restoreWxMethod('request'); mocks.delete('request')
  await (await button(page, '重新加载')).tap(); await until(() => page.$('.subscriptions-master'), '真实重试配置成功')
  await record('解除受控网络失败后，真实按钮重试可恢复配置')
  await login(identities[0]); page = await open('/pages/account/subscriptions')
  await enabled(page, '.subscription-authorize')
  for (const outcome of ['accept', 'reject', 'fail'] as const) {
    await nativeOutcome(outcome); const before = nativeCalls.length
    await (await enabled(page, '.subscription-authorize')).tap()
    await until(async () => nativeCalls.length === before + 1, '真实点击调用原生替身 ' + outcome)
    await enabled(page, '.subscription-authorize')
    const error = await page.$('.subscription-error'); assert.equal(error, null, '原生结果不应阻断本页或产生请求错误')
    const native = nativeCalls.at(-1); assert.ok(native && native.templates.length <= 3 && new Set(native.templates).size === native.templates.length && native.templates.every(id => id.startsWith('local_sdk_only_' + marker + '_')), '只能申请本轮假模板且不能重复计数')
    await record('个人页真实点击处理原生 ' + outcome + '，模板去重且不超过三个')
  }
  const services = await call<Service[]>('/services?game_id=delta&limit=100')
  const service = services.find(item => item.status === 'active' && item.provider_id !== 'player')
  assert.ok(service, '本地验收缺少可浏览服务')
  page = await open('/pages/services/detail?id=' + encodeURIComponent(service.id))
  await reminderPopup(page, 'service-reminder')
  await (await until(() => find(page, '.subscribe-later'), '稍后再说')).tap()
  await until(async () => { const sheet = await find(page, '.finance-popup-sheet'); return !sheet || (await rectangle(sheet)).height === 0 }, '浮层真实关闭')
  assert.equal((await sdk().currentPage())?.path, 'pages/services/detail', '关闭订阅不能离开或阻断详情')
  await record('服务详情订阅浮层可关闭，取消不影响正常浏览')
  await reminderPopup(page, 'service-reminder-reject'); await nativeOutcome('reject'); const before = nativeCalls.length
  await (await enabled(page, '.subscribe-confirm')).tap()
  await until(async () => nativeCalls.length === before + 1, '共享按钮调起原生拒绝替身')
  await until(async () => { const sheet = await find(page, '.finance-popup-sheet'); return !sheet || (await rectangle(sheet)).height === 0 }, '拒绝后共享浮层关闭')
  assert.equal((await sdk().currentPage())?.path, 'pages/services/detail')
  await record('共享组件真实按钮处理拒绝，详情仍可使用')
  await sdk().restoreWxMethod('requestSubscribeMessage'); mocks.delete('requestSubscribeMessage')
  deliveryDelta = (await snapshotDeliveries()).filter(item => !originalDeliveries.has(item.id)).map(({ scene, status, error_code }) => ({ scene, status, error_code }))
  assert.ok(deliveryDelta.every(item => item.status !== 'sent' && item.status !== 'sending'), '授权验收不允许触发真实微信发送')
  await logout(); await record('测试结束通过真实退出入口恢复访客，不残留管理员会话', 'final-guest')
  assert.deepEqual(exceptions, []); assert.deepEqual(warnings, [])
}
/** 无论验收成功失败都先撤销原生替身、恢复本地配置并退出，恢复失败必须报告而非伪装通过。 */
async function cleanup(): Promise<void> {
  const errors: string[] = []
  for (const method of [...mocks]) try { await mini?.restoreWxMethod(method); mocks.delete(method) } catch (error) { errors.push('恢复原生方法 ' + method + '：' + redact(error)) }
  if (originalSettings) try { await call('/admin/subscriptions/settings', sessions.get('admin')?.token, 'PATCH', originalSettings); assert.deepEqual(settingsBody(await call<SubscriptionSettings>('/admin/subscriptions/settings', sessions.get('admin')?.token)), originalSettings) } catch (error) { errors.push('恢复本地订阅配置：' + redact(error)) }
  if (mini) try { await logout() } catch (error) { errors.push('真实退出访客失败：' + redact(error)) }
  for (const session of sessions.values()) try { await call('/auth/logout', session.token, 'POST', {}) } catch (error) { errors.push('撤销独立核对会话：' + redact(error)) }
  SDK.prototype.checkVersion = originalVersionCheck
  try { await mini?.disconnect() } catch (error) { errors.push('断开官方 SDK：' + redact(error)) }
  if (errors.length) cleanupFailure = errors.join('；')
}
/** 保存脱敏报告，真实收件、外部签名回调和真机能力单独标注验收边界。 */
function report(): void {
  fs.mkdirSync(output, { recursive: true })
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify({ passed: !failure && !cleanupFailure, stage, viewport, checks, native_mock_calls: nativeCalls, screenshots, measurements, version_checks: versionChecks, runtime_exceptions: exceptions, canvas_color_warnings: warnings, delivery_delta: deliveryDelta, configuration_restored: !!originalSettings && !cleanupFailure, ...(failure ? { error: redact(failure) } : {}), ...(cleanupFailure ? { cleanup_error: cleanupFailure } : {}), boundaries: ['原生订阅accept/reject/fail由官方SDK受控替身返回，不构成真实微信授权或用户收件证明。', '只操作127.0.0.1:18091开发服务与dist/dev工程；本地API须使用假AppID和假secret且没有对应官方身份。', '未创建订单、支付、俱乐部或触发发送/重试；原有业务交易与资金数据不修改。', '320和390由主流程分别设置机型运行，本脚本不改设备或自动化端口。', '真实模板授权、微信终端展示、签名回调和真机生命周期需另行验收。'] }, null, 2) + '\n')
}
run().catch(error => { failure = error; console.error(redact(error)); process.exitCode = 1 }).finally(async () => { await cleanup(); if (cleanupFailure) { console.error(cleanupFailure); process.exitCode = 1 } report() })
