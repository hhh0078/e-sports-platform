/** 文件作用：通过微信官方自动化 SDK 实际点击小程序，验收四身份聊天室与退款争议闭环；仅运行本地测试环境。 */
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const automator = require('../frontend/node_modules/miniprogram-automator')
const evidenceDir = path.resolve(__dirname, '../output/playwright')
let mini
const events = []
/** 在短时限内等待实际页面状态，失败给出明确步骤信息。 */
async function until(read, description, timeout = 12000) {
  const started = Date.now()
  while (Date.now() - started < timeout) { const value = await read(); if (value) return value; await new Promise(resolve => setTimeout(resolve, 150)) }
  throw new Error('等待超时：' + description)
}
/** 取得当前路由并等待所需页面加载。 */
async function pageAt(route) { return until(async () => { const page = await mini.currentPage(); return page?.path === route ? page : null }, route) }
/** 根据用户可见文字寻找按钮，不依赖组件内部状态或直接调用业务方法。 */
async function button(text) {
  return until(async () => { const page = await mini.currentPage(); for (const element of await page.$$('button')) { if ((await element.text()).trim() === text) return element } return null }, '按钮 ' + text)
}
/** 点击可见操作并填写确认框，验证行为由前后端真实流程完成。 */
async function action(text, note = '') {
  await (await button(text)).tap()
  const page = await mini.currentPage()
  if (note) { const field = await until(() => page.$('.modal textarea'), '操作原因'); await field.input(note) }
  await (await button('确认' + text)).tap()
  await until(async () => !(await page.$('.modal')), '操作完成 ' + text)
}
/** 从本地身份选择界面登录，不读取或写入会话令牌。 */
async function login(name) {
  await mini.reLaunch('/pages/login/index')
  const page = await pageAt('pages/login/index')
  await page.waitFor(450)
  await (await until(() => page.$('.consent-line label'), '协议勾选标签')).tap()
  await until(async () => String(await (await page.$('checkbox')).property('checked')) === 'true', '明确同意协议')
  await (await button('开发体验')).tap()
  const account = await until(async () => { for (const element of await page.$$('button')) if ((await element.text()).includes(name)) return element; return null }, '账号 ' + name)
  await account.tap()
  await (await button('先这样，稍后完善')).tap()
  await pageAt('pages/me/index')
  events.push('登录：' + name); console.log('PASS 登录 ' + name)
}
/** 只通过明确标识的本地测试手机号按钮验证，不模拟真实微信授权结果。 */
async function bindPhoneOrWaitForOrder(page) {
  await until(async () => {
    if ((await mini.currentPage())?.path === 'pages/order/index') return true
    const shell = await page.$('components\\/AppShell'), phone = shell && await shell.$('components\\/PhoneVerification')
    const control = phone && await phone.$('.phone-dev')
    if (!control) return false
    await control.tap(); events.push('手机号：通过本地模拟号码验证，未调用真实运营商认证'); console.log('PASS 本地手机号验证')
    return true
  }, '订单跳转或手机号验证')
}
/** 打开订单并等待服务端状态渲染。 */
async function order(id, expected) {
  await mini.navigateTo('/pages/order/index?id=' + id)
  const page = await pageAt('pages/order/index')
  await expectStatus(page, expected)
  return page
}
/** 核对用户可见的订单状态。 */
async function expectStatus(page, expected) {
  await until(async () => { const element = await page.$('.status-title'); return element && (await element.text()) === expected }, '订单状态 ' + expected)
  events.push('状态：' + expected); console.log('PASS 状态 ' + expected)
}
/** 在实际输入框中发送沟通文字，并检查留存结果。 */
async function chat(content) {
  const page = await mini.currentPage()
  await (await page.$('.composer input')).input(content)
  await (await button('发送')).tap()
  await until(async () => (await page.$('.chat-panel')).text().then(text => text.includes(content)), '聊天消息留存')
  events.push('聊天：' + content); console.log('PASS 聊天留存')
}
/** 保存模拟器原始截图，供人工复核 UI 与业务状态。 */
async function screenshot(name, selector) {
  if (selector) { const page = await mini.currentPage(); const element = await page.$(selector); if (element) { const offset = await element.offset(); await mini.pageScrollTo(Math.max(0, Number(offset.top) - 100)) } }
  await mini.screenshot({ path: path.join(evidenceDir, name + '.png') })
  if (selector) await mini.pageScrollTo(0)
}
/** 执行一次完整退款争议，只有本地测试模式才允许开始。 */
async function run() {
  const health = await fetch('http://127.0.0.1:8090/api/bootstrap').then(response => response.json())
  assert.equal(health.data.dev_mode, true, '禁止对正式交易运行自动测试')
  fs.mkdirSync(evidenceDir, { recursive: true })
  mini = await automator.connect({ wsEndpoint: process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9420' })
  const exceptions = []
  mini.on('exception', event => exceptions.push(String(event.message || event)))
  await login('云间旅人')
  await mini.navigateTo('/pages/service/index?id=delta-service-1')
  let page = await pageAt('pages/service/index')
  await (await until(() => page.$('textarea'), '需求输入')).input('微信模拟器验收：双排教学一小时，需讲解撤离路线，交付后共同核对。')
  await (await button('确认下单')).tap()
  await bindPhoneOrWaitForOrder(page)
  page = await pageAt('pages/order/index')
  const id = page.query.id
  assert.ok(id)
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
  await login('集结管理员'); page = await order(id, '待验收')
  await chat('平台：已核对双方要求与交付记录，现给出本地测试裁决。')
  await screenshot('mini-admin-dispute', '.admin-resolution')
  await (await button('支持买方 · 原路退款')).tap()
  await (await until(() => page.$('.modal textarea'), '平台裁决原因')).input('本地验收：支持买方全额退款，双方聊天和处理记录继续留存。')
  await (await button('确认裁决退款')).tap()
  await expectStatus(page, '已退款')
  await screenshot('mini-refunded')
  await login('自由玩家'); await mini.navigateTo('/pages/order/index?id=' + id); page = await pageAt('pages/order/index')
  await until(async () => { const panel = await page.$('.error-panel'); return panel && (await panel.text()).includes('无权') }, '陌生账号被拒绝')
  assert.equal(await page.$('.chat-panel'), null)
  await screenshot('mini-order-denied')
  assert.deepEqual(exceptions, [], '小程序出现业务运行异常')
  await mini.reLaunch('/pages/index/index'); await screenshot('mini-home-final')
  const report = { time: new Date().toISOString(), order_id: id, passed: true, assertions: events, runtime_exceptions: exceptions }
  fs.writeFileSync(path.join(evidenceDir, 'mini-e2e.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report, null, 2))
}
run().catch(error => { console.error(error.stack); process.exitCode = 1 }).finally(async () => { if (mini) await mini.disconnect() })
