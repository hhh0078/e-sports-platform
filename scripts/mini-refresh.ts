/** 文件作用：通过微信官方SDK验收主视图刷新事件、二级原生下拉、真实服务端分页及失败重试；不提交资金或业务变更。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { promisify } from 'node:util'
import { execFile } from 'node:child_process'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { ScrollViewElement, TextareaElement, InputElement } from '../frontend/node_modules/miniprogram-automator/out/Element'

/** 官方SDK仅查询窗口和公开节点，禁止访问Vue实例、令牌或页面内部data。 */
declare const wx: { getWindowInfo(): { windowWidth: number; windowHeight: number } }
/** 节点查找只沿已编译的公共组件边界。 */
type Scope = Page | Element
const root = process.cwd(), require = createRequire(path.join(root, 'frontend/package.json'))
const project = path.join(root, 'frontend/dist/dev/mp-weixin'), api = 'http://127.0.0.1:18091/api'
const output = path.join(root, 'output/pull-refresh/mini-' + (process.env.UI_EXPECT_WIDTH || '390'))
const automator = require('miniprogram-automator') as { connect(options: { wsEndpoint: string }): Promise<MiniProgram> }
const runFile = promisify(execFile), cli = '/Applications/wechatwebdevtools.app/Contents/MacOS/wechatide'
const checks: string[] = [], exceptions: string[] = [], warnings: string[] = [], measurements: unknown[] = []
let mini: MiniProgram | undefined, stage = '初始化', failure: unknown, mockActive = false
let viewport: { windowWidth: number; windowHeight: number } | undefined

/** 取得唯一受本流程管理的官方连接。 */
function sdk(): MiniProgram { assert.ok(mini); return mini }
/** 脱敏保存错误，不写入会话或用户私密资料。 */
function redact(error: unknown): string { return String(error instanceof Error ? error.message : error).replace(/(token|secret|openid|authorization)["'\s:=]+[^\s,}"']+/gi, '$1=[REDACTED]').slice(0, 1300) }
/** 等待实际条件，不通过重复点击来重试任何业务提交。 */
async function until<T>(read: () => Promise<T | false | null | undefined>, label: string, timeout = 18000): Promise<T> {
  const start = Date.now()
  while (Date.now() - start < timeout) { const value = await read(); if (value) return value; await new Promise(resolve => setTimeout(resolve, 150)) }
  throw new Error('等待超时：' + label)
}
/** 打开本地实际路由，不注入组件状态。 */
async function open(url: string): Promise<Page> { stage = url; console.log('CHECK ' + stage); const page = await sdk().reLaunch(url); assert.ok(page); return page }
/** 兼容官方SDK公开组件标签的短名称与编译路径名称。 */
async function component(scope: Scope, name: string): Promise<Element | null> { const aliases: Record<string,string> = { 'bottom-nav':'BottomNav','app-shell':'AppShell','manage-list':'ManageList','list-load-state':'ListLoadState','funds-form':'FundsForm','finance-popup':'FinancePopup','empty-state':'EmptyState' }; return await scope.$(name) || await scope.$('components\\/' + aliases[name]) }
/** 仅沿页面公开组件递归寻找真实节点。 */
async function find(scope: Scope, selector: string, depth = 0): Promise<Element | null> {
  const direct = await scope.$(selector); if (direct) return direct
  if (depth >= 3) return null
  for (const name of ['app-shell','manage-list','list-load-state','funds-form','finance-popup','empty-state','bottom-nav']) { const child = await component(scope,name); if (child) { const found = await find(child, selector, depth + 1); if (found) return found } }
  return null
}
/** 找到明确文本的真实按钮，不调用页面私有方法。 */
async function button(page: Page, label: string): Promise<Element> { return until(async () => { for (const item of await page.$$('button') as Element[]) if ((await item.text()).replace(/[\uE000-\uF8FF]/g, '').trim() === label) return item; return null }, label) }
/** 保存微信工具原始截图及当前检查点。 */
async function pass(label: string, name?: string): Promise<void> {
  checks.push(label); console.log('PASS ' + label)
  if (name) await runFile(cli, ['-c','Codex','simulator_screenshot','--project',project,'--path',path.join(output,name + '.png'),'--optimize','false'], { timeout: 25000 })
}
/** 通过真实开发体验入口登录，不向小程序注入token。 */
async function login(nickname: string): Promise<void> {
  const page = await open('/pages/account/login')
  await until(async () => { const item = await page.$('.login-primary'); return item && await item.property('disabled') !== true ? item : null }, '登录准备')
  await (await until(() => page.$('.consent-line label'), '协议勾选')).tap()
  await (await button(page, '开发体验')).tap()
  await (await until(async () => { for (const item of await page.$$('.account-card') as Element[]) if ((await item.text()).includes(nickname)) return item; return null }, '体验身份')).tap()
  await until(async () => { if ((await sdk().currentPage())?.path === 'pages/main/index') return true; const skip = await page.$('.finish-later'); if (skip) await skip.tap(); const error = await page.$('.login-error'); if (error) throw new Error(await error.text()); return false }, '登录回跳')
}
/** 通过真实退出恢复访客，不留管理员客户端会话。 */
async function logout(): Promise<void> {
  const page = await open('/pages/account/profile')
  await sdk().pageScrollTo(100000)
  const current = await until(async () => { const exit = await page.$('.logout-button'); return exit ? { exit } : await page.$('.guest-note') ? { exit: null } : null }, '退出入口')
  if (current.exit) { await current.exit.tap(); await until(async () => (await sdk().currentPage())?.path === 'pages/main/index', '访客主页') }
}
/** 触发官方二级页原生下拉动画，完成后检查正文和草稿仍存在。 */
async function nativeRefresh(page: Page, selector: string): Promise<void> { await sdk().pageScrollTo(0); await sdk().callWxMethod('startPullDownRefresh'); await page.waitFor(650); await until(() => find(page, selector), '下拉后正文 ' + selector) }
/** 四个常驻主容器只启用当前手势，程序事件与物理手势证据分别记录。 */
async function mainViews(): Promise<void> {
  const page = await open('/pages/main/index?tab=player'), nav = await until(() => component(page,'bottom-nav'), '唯一主底栏')
  const buttons = await nav.$$('.nav-item') as Element[]; assert.equal(buttons.length, 4)
  for (let index = 0; index < 4; index++) {
    await buttons[index].tap(); await page.waitFor(300)
    const scrolls = await page.$$('.main-scroll') as ScrollViewElement[]; assert.equal(scrolls.length, 4)
    for (let n = 0; n < 4; n++) { const enabled = await scrolls[n].property('refresherEnabled'); assert.equal(enabled === true || enabled === 'true', n === index, '只能激活当前容器') }
    const scroll = scrolls[index]
    await scroll.scrollTo(0, 0); await scroll.trigger('refresherrefresh'); await scroll.trigger('refresherrefresh')
    await until(async () => { const value = await scroll.property('refresherTriggered'); return value === false || value === 'false' }, '主容器停止刷新动画')
    await scroll.scrollTo(0, 100000); await page.waitFor(250)
    const top = Number(await scroll.property('scrollTop'))
    await buttons[(index + 1) % 4].tap(); await buttons[index].tap(); await page.waitFor(200)
    const after = Number(await scroll.property('scrollTop')); assert.ok(Math.abs(after - top) <= 1, '主标签往返位置丢失')
    measurements.push({ tab: index, scroll_before: top, scroll_after: after, width: Number((await scroll.size()).width) })
    await pass('主导航' + index + '官方刷新事件结束且切页保留位置', 'main-' + index)
  }
}
/** 独立核对本地用户目录实际总量，结束立即撤销核对会话，不向小程序注入token。 */
async function expectedUsers(): Promise<Set<string>> {
  const response = await fetch(api + '/auth/dev', { method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({user_id:'admin'}) })
  assert.equal(response.status,200)
  const session = await response.json() as { data:{token:string} }
  const headers = {Authorization:'Bearer '+session.data.token}, ids = new Set<string>()
  try {
    let offset=0
    while (offset<=1000000) { const result=await fetch(api+'/admin/users?limit=100&offset='+offset,{headers});assert.equal(result.status,200);const page=await result.json() as {data:Array<{id:string}>};for(const row of page.data)ids.add(row.id);offset+=page.data.length;if(page.data.length<100)return ids }
    throw new Error('测试用户目录超过分页安全上限')
  } finally { await fetch(api+'/auth/logout',{method:'POST',headers}) }
}
/** 真实用户列表触底，受控下一页失败不清已有行，恢复后同一按钮重试。 */
async function usersPagination(): Promise<void> {
  const expected=await expectedUsers()
  const page = await open('/pages/manage/users')
  await until(async () => (await page.$$('.user-row')).length === 20, '用户第一页20条')
  await sdk().mockWxMethod('request', () => ({ statusCode: 503, data: { error: '本地SDK受控分页失败' }, header: {} })); mockActive = true
  await sdk().pageScrollTo(100000)
  const retry = await until(() => find(page, '.list-load-retry'), '失败页脚')
  assert.equal((await page.$$('.user-row')).length, 20, '下一页失败不能丢首屏记录')
  await pass('真实列表第二页失败保留20条且显示轻量重试', 'users-page-error')
  await sdk().restoreWxMethod('request'); mockActive = false; await retry.tap()
  await until(async () => (await page.$$('.user-row')).length >= 40, '同偏移重试追加40条')
  for (let n = 0; n < Math.ceil(expected.size/20)+1; n++) { await sdk().pageScrollTo(0); await sdk().pageScrollTo(100000); await page.waitFor(250); if ((await page.$$('.user-row')).length >= expected.size) break }
  const rows = await page.$$('.user-row') as Element[]; assert.equal(rows.length, expected.size)
  const ids = await Promise.all(rows.map(async row => { const meta = await row.$('.user-meta'); assert.ok(meta); return (await meta.text()).replace(/^ID:\s*/,'') })); assert.equal(new Set(ids).size, rows.length);assert.deepEqual([...ids].sort(),[...expected].sort())
  await pass('用户列表触底完整加载'+rows.length+'条、无重复并与真实API一致', 'users-all-pages')
  await nativeRefresh(page, '.user-row'); await until(async () => (await page.$$('.user-row')).length === 20, '原生下拉重置第一页')
  await pass('管理列表原生下拉重新读取并结束动画', 'users-refreshed')
}
/** 服务详情的只读下拉不重置下单说明草稿。 */
async function serviceDraft(): Promise<void> {
  const page = await open('/pages/services/detail?id=delta-service-8')
  const input = await until(() => page.$('textarea'), '服务下单说明') as TextareaElement
  const draft = '刷新验收草稿，不提交订单'; await input.input(draft)
  await nativeRefresh(page, 'textarea'); assert.equal(await input.value(), draft)
  await pass('服务详情下拉保留下单说明草稿', 'service-draft-retained')
}
/** 钱包金额弹层打开时下拉不清金额、不关闭弹层，不提交任何付款。 */
async function walletDraft(): Promise<void> {
  const page = await open('/pages/account/wallet')
  await until(() => page.$('.deposit-main'), '押金区域')
  await (await until(() => page.$('.deposit-main .text-btn'), '自愿押金入口')).tap()
  const input = await until(() => find(page, '.funds-amount-input'), '金额输入') as InputElement
  await input.input('123.45'); await sdk().callWxMethod('startPullDownRefresh'); await page.waitFor(400)
  assert.equal(await input.value(), '123.45'); assert.ok(await find(page, '.finance-popup-sheet'))
  const close = await until(() => find(page, '.finance-popup-close'), '关闭资金弹层'); await close.tap()
  await pass('钱包弹层下拉受阻且金额草稿保留，未创建付款', 'wallet-draft-retained')
}
/** 验收当前机型的真实页面和公开分页。 */
async function run(): Promise<void> {
  assert.ok(fs.readFileSync(path.join(project, 'api/client.js'), 'utf8').includes(api), '仅允许本地开发工程')
  fs.mkdirSync(output, { recursive: true })
  mini = await automator.connect({ wsEndpoint: 'ws://127.0.0.1:9423' })
  mini.on('exception', (error: unknown) => exceptions.push(redact(error)))
  mini.on('console', (entry: unknown) => { const value = JSON.stringify(entry); if (/Some selectors are not allowed|不支持颜色|unsupported color/i.test(value)) warnings.push(value.slice(0, 300)) })
  viewport = await mini.evaluate(() => wx.getWindowInfo()) as typeof viewport
  assert.equal(viewport?.windowWidth, Number(process.env.UI_EXPECT_WIDTH || 390))
  await logout(); await mainViews(); await login('开机电竞管理员'); await usersPagination(); await serviceDraft(); await walletDraft()
  for (const [route, selector] of [['/pages/manage/index','.module-grid'],['/pages/account/messages/index','.message-toolbar'],['/pages/account/subscriptions','.subscription-row'],['/pages/community/clubs/index','.club-search']] as const) { const page = await open(route); await until(() => find(page, selector), route); await nativeRefresh(page, selector); await pass(route + '只读下拉后正常展示') }
  assert.deepEqual(exceptions, []); assert.deepEqual(warnings, [])
}

run().catch(async error => { failure = error; console.error(redact(error)); if (mini) try { await pass('失败现场保留', 'failure') } catch { /* 不掩盖原错误。 */ } process.exitCode = 1 }).finally(async () => {
  if (mockActive) try { await mini?.restoreWxMethod('request'); mockActive = false } catch (error) { failure ||= error; process.exitCode = 1 }
  if (mini) { try { await logout() } catch (error) { failure ||= error; process.exitCode = 1 } await mini.disconnect() }
  fs.mkdirSync(output, { recursive: true }); fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify({ file_purpose: '本地微信SDK刷新分页验收，不代表真机手势性能或真实资金', passed: !failure, stage, viewport, checks, measurements, runtime_exceptions: exceptions, compatibility_warnings: warnings, ...(failure ? { error: redact(failure) } : {}), boundaries: ['主容器刷新使用官方SDK事件，二级刷新使用官方startPullDownRefresh；物理手势另行记录。','列表来自本地真实API，仅下一页503为受控替身；未提交订单、支付、退款或后台配置。','手机实际手势流畅度和网络需单独真机验收。'] }, null, 2) + '\n')
})
