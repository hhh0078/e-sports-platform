/** 文件作用：通过微信官方自动化 SDK 的可见界面验收四方聊天、履约、退款拒绝、平台介入及陌生账号隔离；仅允许本地测试后端。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import automator from '../frontend/node_modules/miniprogram-automator'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { InputElement, TextareaElement } from '../frontend/node_modules/miniprogram-automator/out/Element'

const projectDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const evidenceDir = path.resolve(process.env.TEST_EVIDENCE_DIR || path.join(projectDir, 'output/standards-refactor/transaction'))
const apiBase = process.env.TEST_API_BASE || 'http://127.0.0.1:18091/api'
const endpoint = process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9420'
let mini: MiniProgram | undefined
const events: string[] = []
/** 在微信逻辑层调用同一原生路由API，避免SDK等待已销毁页面的回调。 */
declare const wx: {navigateTo(options:{url:string}):void;reLaunch(options:{url:string}):void}

/** 获取已连接 SDK，防止异常退出时访问未初始化对象。 */
function connected(): MiniProgram { assert.ok(mini, '微信自动化 SDK 尚未连接'); return mini }

/** 在短时限内等待实际界面状态，超时给出可定位的步骤名称。 */
async function until<T>(read: () => Promise<T | null | false | undefined>, description: string, timeout = 12000): Promise<T> {
  const started = Date.now()
  while (Date.now() - started < timeout) { const value = await read(); if (value) return value; await new Promise<void>(resolve => setTimeout(resolve, 150)) }
  throw new Error('等待超时：' + description)
}

/** 等待当前路由准备好，必须经过真正的页面导航。 */
async function pageAt(route: string): Promise<Page> { return until(async () => { const page = await connected().currentPage(); return page?.path === route ? page : null }, route) }

/** 原生跳转后按实际页面确认；重启时先结束自动化RPC，再验证新的页面实例。 */
async function openRoute(url:string,restart=false):Promise<void> {
  const previous=await connected().currentPage()
  await connected().evaluate((target:string,reset:boolean)=>{if(reset)setTimeout(()=>wx.reLaunch({url:target}),30);else wx.navigateTo({url:target});return true},url,restart)
  await until(async()=>{const page=await connected().currentPage();return page?.path===url.split('?')[0].replace(/^\//,'')&&(!restart||page!==previous)?page:null},'原生路由 '+url.split('?')[0])
}

/** 移除图标字体和多余空白，只匹配用户可读按钮文字。 */
function visibleText(value: string): string { return value.replace(/[\uE000-\uF8FF]/g, '').replace(/\s+/g, ' ').trim() }

/** 根据界面中的实际按钮文字查找操作，不调用页面内部方法。 */
async function button(text: string): Promise<Element> {
  return until(async () => { const page = await connected().currentPage(); if (!page) return null; const elements: Element[] = await page.$$('button'); for (const element of elements) if (visibleText(await element.text()) === text) return element; return null }, '按钮 ' + text)
}

/** 确认输入节点为官方 SDK 支持的文本输入框。 */
async function field(page: Page, selector: string): Promise<InputElement | TextareaElement> {
  const element = await until(() => page.$(selector), '输入框 ' + selector)
  assert.ok('input' in element && typeof element.input === 'function', '节点不是输入框：' + selector)
  return element as InputElement | TextareaElement
}

/** 点击可见订单动作并填写确认原因，业务状态由前后端真实流转。 */
async function action(text: string, note = ''): Promise<void> {
  await (await button(text)).tap()
  const page = await connected().currentPage(); assert.ok(page)
  if (note) await (await field(page, '.modal textarea')).input(note)
  await (await button('确认' + text)).tap()
  await until(async () => !(await page.$('.modal')), '操作完成 ' + text)
}

/** 从协议复选框与本地体验身份界面登录，绝不读取或写入令牌。 */
async function login(name: string): Promise<void> {
  await openRoute('/pages/account/login',true)
  const page = await pageAt('pages/account/login')
  await page.waitFor(450)
  await (await until(() => page.$('.consent-line label'), '协议勾选标签')).tap()
  await until(async () => { const checkbox = await page.$('checkbox'); return checkbox && String(await checkbox.property('checked')) === 'true' }, '用户明确勾选两协议')
  await (await button('开发体验')).tap()
  const account = await until(async () => { const elements: Element[] = await page.$$('button'); for (const element of elements) if ((await element.text()).includes(name)) return element; return null }, '体验账号 ' + name)
  await account.tap()
  await (await button('先这样，稍后完善')).tap()
  await pageAt('pages/main/index')
  events.push('登录：' + name); console.log('PASS 登录 ' + name)
}

/** 寻找官方手机号弹层内明确标识的本地测试入口。 */
async function phoneControl(page: Page): Promise<Element | null> {
  const direct = await page.$('.phone-dev'); if (direct) return direct
  const shell = await page.$('app-shell') || await page.$('components\\/AppShell')
  if (!shell) return null
  const phone = await shell.$('phone-verification') || await shell.$('components\\/PhoneVerification')
  if (!phone) return null
  const control = await phone.$('.phone-dev'); if (control) return control
  const popup = await phone.$('uni-popup')
  return popup ? popup.$('.phone-dev') : null
}

/** 只点击本地手机号模拟按钮，明确不声称真实手机号能力已验收。 */
async function bindPhoneOrWaitForOrder(page: Page): Promise<void> {
  await until(async () => {
    if ((await connected().currentPage())?.path === 'pages/orders/detail') return true
    const control = await phoneControl(page); if (!control) return false
    await control.tap(); events.push('手机号：通过本地模拟号码验证，未调用真实认证'); console.log('PASS 本地手机号验证'); return true
  }, '订单跳转或本地手机号验证')
}

/** 打开订单并等待真实状态渲染。 */
async function order(id: string, expected: string): Promise<Page> { await openRoute('/pages/orders/detail?id=' + encodeURIComponent(id)); const page = await pageAt('pages/orders/detail'); await expectStatus(page, expected); return page }

/** 核对用户可见履约状态。 */
async function expectStatus(page: Page, expected: string): Promise<void> { await until(async () => { const element = await page.$('.status-title'); return element && (await element.text()) === expected }, '订单状态 ' + expected); events.push('状态：' + expected); console.log('PASS 状态 ' + expected) }

/** 在实际聊天输入框发送文本，等待消息在界面中留存。 */
async function chat(content: string): Promise<void> { const page = await connected().currentPage(); assert.ok(page); await (await field(page, '.composer input')).input(content); await (await button('发送')).tap(); await until(async () => { const panel = await page.$('.chat-panel'); return panel && (await panel.text()).includes(content) }, '聊天消息留存'); events.push('聊天：' + content); console.log('PASS 聊天留存') }

/** 保存微信模拟器原始截图，供业务和视觉人工复核。 */
async function screenshot(name: string, selector?: string): Promise<void> {
  if (selector) { const page = await connected().currentPage(), element = page && await page.$(selector); if (element) { const offset: { top: number | string } = await element.offset(); await connected().pageScrollTo(Math.max(0, Number(offset.top) - 100)) } }
  await connected().screenshot({ path: path.join(evidenceDir, name + '.png') })
  if (selector) await connected().pageScrollTo(0)
}

/** 验证运行目标仅为本机测试后端，不允许误对生产交易运行自动化。 */
async function verifyTarget(): Promise<void> {
  const url = new URL(apiBase)
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname), '验收仅允许连接本机测试后端')
  const response = await fetch(apiBase + '/bootstrap'); assert.equal(response.status, 200)
  const health: unknown = await response.json()
  assert.ok(typeof health === 'object' && health !== null && 'data' in health)
  const data = health.data
  assert.ok(typeof data === 'object' && data !== null && 'dev_mode' in data && data.dev_mode === true, '禁止对生产环境执行模拟支付和退款')
}

/** 保留四类参与者与无关账号的完整本地退款争议验收链路。 */
async function run(): Promise<void> {
  await verifyTarget(); fs.mkdirSync(evidenceDir, { recursive: true })
  mini = await automator.connect({ wsEndpoint: endpoint })
  const exceptions: string[] = []
  mini.on('exception', (event: unknown) => exceptions.push(typeof event === 'object' && event !== null && 'message' in event ? String(event.message) : String(event)))
  await login('云间旅人')
  await openRoute('/pages/services/detail?id=delta-service-1')
  let page = await pageAt('pages/services/detail')
  await (await field(page, 'textarea')).input('微信模拟器验收：双排教学一小时，需讲解撤离路线，交付后共同核对。')
  await (await button('确认下单')).tap(); await bindPhoneOrWaitForOrder(page)
  page = await pageAt('pages/orders/detail')
  const query: Record<string, unknown> = page.query
  assert.equal(typeof query.id, 'string'); const id = query.id as string; assert.ok(id)
  await expectStatus(page, '待支付')
  await action('支付订单'); await expectStatus(page, '等待接单')
  await chat('老板：今晚八点集合，请先确认需求。')
  await login('北辰'); page = await order(id, '等待接单')
  await action('接受订单'); await expectStatus(page, '已接单')
  await chat('打手：已确认需求，会在订单内同步进度。')
  await action('开始服务'); await expectStatus(page, '服务中')
  await action('提交交付', '已完成一小时双排教学，讲解了撤离路线与物资选择。'); await expectStatus(page, '待验收')
  await login('山海主理人'); page = await order(id, '待验收')
  await chat('俱乐部：已加入沟通，发生争议会配合核查服务记录。')
  await screenshot('mini-club-chat', '.chat-heading')
  await login('云间旅人'); page = await order(id, '待验收')
  await action('申请退款', '演示售后：认为讲解内容与事先约定不一致，请双方核对。')
  await login('北辰'); page = await order(id, '待验收')
  await action('拒绝退款', '演示答辩：已经完成约定教学，提交交付记录作为依据。')
  await login('云间旅人'); page = await order(id, '待验收')
  await action('申请平台介入', '演示仲裁：双方意见不同，请平台查看四方聊天与下单约定。')
  await login('开机电竞管理员'); page = await order(id, '待验收')
  await chat('平台：已核对双方要求与交付记录，现给出本地测试裁决。')
  await screenshot('mini-admin-dispute', '.admin-resolution')
  await (await button('支持买方 · 原路退款')).tap()
  await (await field(page, '.modal textarea')).input('本地验收：支持买方全额退款，双方聊天和处理记录继续留存。')
  await (await button('确认裁决退款')).tap(); await expectStatus(page, '已退款')
  await screenshot('mini-refunded')
  await login('自由玩家'); await openRoute('/pages/orders/detail?id=' + encodeURIComponent(id)); page = await pageAt('pages/orders/detail')
  await until(async () => { const panel = await page.$('.error-panel'); return panel && (await panel.text()).includes('无权') }, '陌生账号订单访问被拒绝')
  assert.equal(await page.$('.chat-panel'), null); await screenshot('mini-order-denied')
  assert.deepEqual(exceptions, [], '小程序出现业务运行异常')
  await openRoute('/pages/main/index?tab=player',true); await screenshot('mini-home-final')
  const report = { time: new Date().toISOString(), api_base: apiBase, order_id: id, passed: true, assertions: events, runtime_exceptions: exceptions, limits: ['本地模拟交易，不代表真实微信支付或手机号能力验收'] }
  fs.writeFileSync(path.join(evidenceDir, 'mini-transaction.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report, null, 2))
}

run().catch((error: unknown) => { console.error(error instanceof Error ? error.stack : String(error)); process.exitCode = 1 }).finally(async () => { if (mini) await mini.disconnect() })
