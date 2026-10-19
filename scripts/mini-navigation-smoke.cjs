/**
 * 文件作用：用微信官方自动化 SDK 实际操作打手、订单、我的三页，核验真实数据、角色隔离和可撤销资料操作。
 * 执行：node scripts/mini-navigation-smoke.cjs；前提：本地 8090 接口、9420 微信自动化端口和最新导入构建。
 * 仅允许 DEV_MODE；不创建交易、不保存资料，接单开关在结束及失败时均尝试通过原界面恢复。
 */
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const automator = require('../frontend/node_modules/miniprogram-automator')
const API = 'http://127.0.0.1:8090/api'
const directory = path.resolve(__dirname, '../output/navigation-design')
const report = { passed: false, time: new Date().toISOString(), command: 'node scripts/mini-navigation-smoke.cjs', steps: [], coverage: {}, exceptions: [], console_errors: [] }
const statusNames = { active: '已上架', pending: '审核中', paused: '已暂停', rejected: '未通过' }
let mini, accounts = [], workerOriginal, currentAccount
/** 等待真实界面或只读接口出现预期结果，超时保留明确检查点。 */
async function until(read, description, timeout = 15000) {
  const started = Date.now()
  while (Date.now() - started < timeout) {
    const value = await read()
    if (value) return value
    await new Promise(resolve => setTimeout(resolve, 150))
  }
  throw new Error('等待超时：' + description)
}
/** 统一按钮中的空白，忽略排版换行但不改变可见文案。 */
function textValue(value) { return String(value || '').replace(/\s+/g, ' ').trim() }
/** 将整数分换成与页面相同的金额显示。 */
function money(cents) { return (Number(cents || 0) / 100).toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1') }
/** 通过实际编译组件名称穿过 uni-app 自定义组件边界。 */
async function component(scope, name) { return scope.$('components\\/' + name) }
/** 查找页面声明的节点，必要时检查 SectionHero 插槽所在组件。 */
async function element(page, selector) {
  const direct = await page.$(selector)
  if (direct) return direct
  const hero = await component(page, 'SectionHero')
  return hero ? hero.$(selector) : null
}
/** 按用户可见文字获取按钮，不修改组件数据或调用业务方法。 */
async function button(scope, label) {
  return until(async () => {
    for (const item of await scope.$$('button')) if (textValue(await item.text()) === label) return item
    return null
  }, '按钮 ' + label)
}
/** 等待真实路由完成，并返回官方 SDK 页面实例。 */
async function pageAt(route) {
  const page = await until(async () => { const current = await mini.currentPage(); return current?.path === route ? current : null }, route)
  // 原生转场完成前路由已经可读，留出动画时间，避免自动化点到正在卸载的节点。
  await page.waitFor(400)
  return page
}
/** 将普通页面操作控件滚动到视口中，不触碰页面内部状态。 */
async function reveal(item) { const offset = await item.offset(); await mini.pageScrollTo(Math.max(0, Number(offset.top) - 150)); await (await mini.currentPage()).waitFor(350) }
/** 读取当前 UI 登录会话的接口结果，令牌仅用于本地 GET 且不写入日志或报告。 */
async function readApi(endpoint, authenticated = true) {
  assert.ok(endpoint.startsWith('/') && !endpoint.startsWith('//'), '仅允许本地 API 相对路径')
  const token = authenticated ? await mini.callWxMethod('getStorageSync', 'jijie_token') : ''
  if (authenticated) assert.ok(typeof token === 'string' && token.length > 0, 'UI 登录后应存在会话')
  const response = await fetch(API + endpoint, { method: 'GET', headers: token ? { Authorization: 'Bearer ' + token } : {}, signal: AbortSignal.timeout(10000) })
  const body = await response.json()
  assert.equal(response.status, 200, '只读接口失败：' + endpoint + ' ' + (body.error || ''))
  return body.data
}
/** 读取已有列表的所有分页，不创建记录，也不假定固定记录数量。 */
async function readAll(endpoint) {
  const items = []
  while (items.length <= 1000000) {
    const batch = await readApi(endpoint + (endpoint.includes('?') ? '&' : '?') + 'limit=100&offset=' + items.length)
    assert.ok(Array.isArray(batch), '列表接口应返回数组：' + endpoint)
    items.push(...batch)
    if (batch.length < 100) return items
  }
  throw new Error('已有数据超过本地验收分页范围：' + endpoint)
}
/** 记录已通过的真实检查点。 */
function passed(description) { report.steps.push(description); console.log('PASS ' + description) }
/** 保存模拟器原始截图，供复核设计和原生提示。 */
async function screenshot(name, top = true) {
  if (top) await mini.pageScrollTo(0)
  await mini.screenshot({ path: path.join(directory, name + '.png') })
}
/** 从本地账号选择页实际登录指定测试身份，不直接调用登录接口。 */
async function login(id) {
  const index = accounts.findIndex(account => account.id === id)
  assert.ok(index >= 0, '本地体验账号缺失：' + id)
  await mini.reLaunch('/pages/login/index')
  const page = await pageAt('pages/login/index')
  await (await until(() => page.$('.consent-line label'), '协议勾选标签')).tap()
  await until(async () => String(await (await page.$('checkbox')).property('checked')) === 'true', '明确同意协议')
  await (await button(page, '开发体验')).tap()
  const cards = await until(async () => { const items = await page.$$('.account-card'); return items.length === accounts.length ? items : null }, '体验身份列表')
  assert.ok((await cards[index].text()).includes(accounts[index].nickname), '登录界面账号顺序应与公开列表一致')
  await reveal(cards[index])
  await cards[index].tap()
  await (await button(page, '先这样，稍后完善')).tap()
  await pageAt('pages/me/index')
  await until(async () => (await readApi('/me')).id === id, '身份切换 ' + id)
  currentAccount = id
  passed('实际登录：' + accounts[index].nickname)
}
/** 点击四项主导航中的固定按钮，验证用户可用的导航路径。 */
async function navigate(label, route) {
  const page = await mini.currentPage(), shell = await component(page, 'AppShell')
  assert.ok(shell, '页面应包含 AppShell')
  let target
  for (const item of await shell.$$('.nav-item')) if (textValue(await item.text()) === label) target = item
  assert.ok(target, '主导航缺少：' + label)
  await target.tap()
  return pageAt(route)
}
/** 比较订单列表的真实可见字段和 API 次序，识别跨角色残留。 */
async function visibleOrders(page) {
  const rows = await page.$$('.order-card')
  return Promise.all(rows.map(async row => ({
    title: textValue(await (await row.$('.order-title')).text()),
    price: textValue(await (await row.$('.amount')).text()),
    counterparty: textValue(await (await row.$('.counterparty')).text()),
    action: textValue(await (await row.$('.enter')).text())
  })))
}
/** 取得当前身份下订单应展示的真实操作文案。 */
function orderAction(order, userId) {
  if (order.status === 'pending_payment') return order.buyer_id === userId ? '继续支付' : '等待买家支付'
  if (['requested', 'rejected', 'escalated', 'processing'].includes(order.refund_status)) return '查看售后'
  return ['open', 'accepted', 'in_progress', 'delivered', 'disputed'].includes(order.status) ? '订单沟通' : '查看订单'
}
/** 核验当前订单视角、阶段、服务端过滤结果和可见操作。 */
async function checkOrders(page, role, status, roleLabel, statusLabel) {
  const user = await readApi('/me')
  const expected = await readApi('/orders?role=' + role + '&status=' + status + '&limit=20&offset=0')
  const expectedRows = expected.map(order => ({
    title: textValue(order.title), price: '¥' + money(order.amount_cents),
    counterparty: order.buyer_id === user.id ? '队友：' + (order.provider_name || '等待接单') : '老板：' + order.buyer_name,
    action: orderAction(order, user.id)
  }))
  await until(async () => {
    if (await page.$('.loading-line')) return false
    const selectedRole = await page.$('.role-tabs .selected'), selectedFilter = await page.$('.filter.selected')
    return selectedRole && selectedFilter && textValue(await selectedRole.text()) === roleLabel && textValue(await selectedFilter.text()) === statusLabel
  }, '订单选中项 ' + role + '/' + status)
  await until(async () => JSON.stringify(await visibleOrders(page)) === JSON.stringify(expectedRows), '订单界面与 API 一致 ' + role + '/' + status)
  assert.equal(await page.$('.error-panel'), null, '订单页面不应有错误')
  if (role === 'provider') assert.ok(expected.every(order => order.provider_id === user.id), '接单视角只能出现本人作为打手的订单')
  if (role === 'buyer') assert.ok(expected.every(order => order.buyer_id === user.id), '下单视角只能出现本人作为买方的订单')
  if (status === 'working') assert.ok(expected.every(order => ['open', 'accepted', 'in_progress'].includes(order.status)))
  if (status === 'completed') assert.ok(expected.every(order => order.status === 'completed'))
  report.coverage[role + '_' + status] = expected.length
  return expected
}
/** 从筛选栏实际点击阶段，保持当前身份不变。 */
async function filterOrders(page, label) {
  const scroll = await page.$('.filters'), buttons = await page.$$('.filter')
  const width = Number((await scroll.size()).width)
  for (let index = 0; index < buttons.length; index++) if (textValue(await buttons[index].text()) === label) {
    await scroll.scrollTo(Math.max(0, index * 77 - width / 2), 0)
    await buttons[index].tap()
    return
  }
  throw new Error('找不到订单筛选：' + label)
}
/** 比较资料关键字段，确保无效编辑和取消没有落库。 */
function assertProfileUnchanged(actual) {
  assert.equal(actual.nickname, workerOriginal.nickname, '测试不应修改昵称')
  assert.equal(actual.bio, workerOriginal.bio, '测试不应修改个人介绍')
}
/** 使用真实开关调整接单身份，等待后端与 UI 同步。 */
async function setProvider(enabled) {
  const page = await pageAt('pages/me/index')
  const user = await readApi('/me')
  assert.equal(user.id, 'worker', '只允许调整本地 worker 体验身份')
  if (user.provider_enabled === enabled) return
  const control = await until(() => page.$('.provider-switch switch'), '接单开关')
  await until(async () => !(await control.property('disabled')), '开关可用')
  await reveal(control)
  await control.tap()
  await until(async () => (await readApi('/me')).provider_enabled === enabled, '接单身份更新为 ' + enabled)
  await until(async () => {
    const updated = await page.$('.provider-switch switch')
    return updated && Boolean(await updated.property('checked')) === enabled && !(await updated.property('disabled'))
  }, '接单开关回显')
}
/** 即使中途失败也通过原界面恢复 worker 初始开关，不直接 PATCH 用户数据。 */
async function restoreWorker() {
  if (!mini || !workerOriginal) return
  if (currentAccount !== 'worker') await login('worker')
  else if ((await mini.currentPage()).path !== 'pages/me/index') await navigate('我的', 'pages/me/index')
  await setProvider(workerOriginal.provider_enabled)
  const user = await readApi('/me')
  assertProfileUnchanged(user)
  assert.equal(user.provider_enabled, workerOriginal.provider_enabled)
  report.restored_original_profile = true
}
/** 执行三页验收，全程依靠真实 UI 触发修改，API 只用于读后核验。 */
async function run() {
  fs.mkdirSync(directory, { recursive: true })
  const bootstrap = await readApi('/bootstrap', false)
  assert.equal(bootstrap.dev_mode, true, '禁止在正式环境运行导航验收')
  accounts = await readApi('/dev/accounts', false)
  mini = await automator.connect({ wsEndpoint: process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9420' })
  mini.on('exception', event => report.exceptions.push(String(event.message || event)))
  mini.on('console', event => {
    if (['error', 'assert'].includes(event.level || event.type)) report.console_errors.push(String(event.text || event.message || JSON.stringify(event)).slice(0, 4000))
  })
  await login('worker')
  workerOriginal = await readApi('/me')
  const stats = await readApi('/me/stats'), services = await readAll('/services?mine=1')
  assert.equal(stats.services_count, services.length, '服务总数应来自本人全部状态服务')
  assert.ok(services.length > 20, '现有服务不足 21 项，无法完成要求的真实加载更多验收；脚本不会创建服务')
  let page = await navigate('打手', 'pages/worker/index')
  const hero = await until(() => component(page, 'SectionHero'), '打手主题头部')
  assert.ok(await hero.$('.section-image'), '打手主题应有独立图片')
  await until(async () => (await page.$$('.my-service')).length === Math.min(20, services.length) && !(await page.$('.loading-line')), '服务首屏')
  assert.ok(textValue(await (await page.$('.worker-tabs .selected')).text()).startsWith('我的服务'), '打手页默认必须为我的服务')
  assert.equal(textValue(await (await page.$('.worker-tabs .selected text')).text()), String(stats.services_count))
  const statNodes = await page.$$('.worker-stats .stat-number')
  assert.deepEqual(await Promise.all(statNodes.map(node => node.text().then(textValue))), [String(stats.provider_working_count), String(stats.provider_completed_count), '¥' + money(stats.earnings_cents)])
  const firstRows = await page.$$('.my-service')
  let independentLabels = 0
  for (let index = 0; index < firstRows.length; index++) {
    const art = await component(firstRows[index], 'CoverArt')
    assert.ok(art, '服务缩略图应使用独立 CoverArt 组件')
    assert.ok(await art.$('.cover-image'))
    const labels = await art.$$('.cover-label')
    assert.equal(labels.length, (services[index].labels || []).filter(label => String(label.text || '').trim()).length)
    assert.equal((await art.$$('canvas')).length, 0)
    independentLabels += labels.length
  }
  assert.ok(independentLabels > 0, '现有首屏服务需要至少一条独立文案以完成图层验收')
  await screenshot('mini-worker')
  let loaded = firstRows.length, pageClicks = 0
  while (await page.$('.load-more')) {
    assert.ok(pageClicks++ <= Math.ceil(services.length / 20), '加载更多不可循环不前进')
    const more = await page.$('.load-more')
    await reveal(more)
    await more.tap()
    const target = Math.min(loaded + 20, services.length)
    await until(async () => (await page.$$('.my-service')).length === target && !(await page.$('.more-error')), '服务下一页')
    loaded = target
    await until(async () => { const control = await page.$('.load-more'); return !control || !(await control.property('disabled')) }, '分页完成')
  }
  assert.equal(loaded, services.length)
  const allRows = await page.$$('.my-service')
  for (let index = 0; index < allRows.length; index++) {
    assert.equal(textValue(await (await allRows[index].$('.service-title')).text()), textValue(services[index].title))
    assert.equal(textValue(await (await allRows[index].$('.service-status')).text()), statusNames[services[index].status] || services[index].status)
  }
  report.coverage.services = { total: services.length, first_page: firstRows.length, load_more_clicks: pageClicks, independent_labels: independentLabels }
  passed('打手默认服务、真实统计、全部状态、独立文案、20条分页直到按钮消失')
  const workingButton = await page.$('.orders-action')
  await reveal(workingButton)
  await workingButton.tap()
  page = await pageAt('pages/orders/index')
  assert.equal(page.query.role, 'provider'); assert.equal(page.query.status, 'working')
  const workingOrders = await checkOrders(page, 'provider', 'working', '我接的单', '进行中')
  assert.ok(workingOrders.length > 0, '需要一笔已有进行中接单验证关闭身份后仍能访问；脚本不创建交易')
  await screenshot('mini-orders')
  await filterOrders(page, '待支付')
  const providerPending = await checkOrders(page, 'provider', 'pending_payment', '我接的单', '待支付')
  assert.ok(providerPending.length > 0, '需要一笔已有打手待支付订单验证等待买家支付；脚本不创建交易')
  assert.ok((await visibleOrders(page)).every(row => row.action === '等待买家支付'))
  await (await button(await page.$('.role-tabs'), '我下的单')).tap()
  await checkOrders(page, 'buyer', 'pending_payment', '我下的单', '待支付')
  passed('进行中深链与服务端过滤一致；打手待支付不出现继续支付；切买方不残留打手订单')
  page = await navigate('我的', 'pages/me/index')
  await until(() => page.$('.stats-row'), '个人统计')
  const statItems = await page.$$('.stats-row > view')
  assert.equal(statItems.length, 3)
  console.log('CHECK 个人完成订单入口'); await statItems[2].tap()
  page = await pageAt('pages/orders/index')
  assert.equal(page.query.role, 'all'); assert.equal(page.query.status, 'completed')
  await checkOrders(page, 'all', 'completed', '全部身份', '已完成')
  page = await navigate('我的', 'pages/me/index')
  let clubItem
  for (const item of await page.$$('.space-grid button')) if ((await item.text()).includes('我的俱乐部')) clubItem = item
  assert.ok(clubItem)
  console.log('CHECK 个人俱乐部入口'); await reveal(clubItem); await clubItem.tap()
  page = await pageAt('pages/clubs/index')
  assert.equal(page.query.mine, '1')
  await until(async () => { const selected = await page.$('.tabs .active'); return selected && textValue(await selected.text()) === '我的俱乐部' }, '个人俱乐部筛选')
  const clubs = await readApi('/clubs?mine=1')
  await until(async () => (await page.$$('.club-card')).length === clubs.length, '本人俱乐部结果')
  assert.deepEqual(await Promise.all((await page.$$('.club-name')).map(item => item.text().then(textValue))), clubs.map(club => textValue(club.name)))
  passed('我的完成订单进入全部身份已完成；我的俱乐部携带 mine=1 并匹配本人列表')
  page = await navigate('我的', 'pages/me/index')
  let profileEntry
  for (const item of await page.$$('.menu-item')) if (textValue(await item.text()).includes('个人资料')) profileEntry = item
  assert.ok(profileEntry, '个人中心应有资料设置入口')
  await reveal(profileEntry); await profileEntry.tap()
  page = await pageAt('pages/profile/index')
  const nickname = await until(() => page.$('.profile-form input'), '昵称输入框')
  for (const [value, name] of [['', 'empty'], ['甲', 'one-character']]) {
    await nickname.input(value)
    await (await button(await page.$('.profile-form'), '保存资料')).tap()
    await screenshot('mini-profile-invalid-' + name, false)
    assert.ok(await page.$('.profile-form'), '无效昵称应保持编辑状态')
    assertProfileUnchanged(await readApi('/me'))
  }
  await (await button(await page.$('.profile-form'), '取消修改')).tap()
  await until(async () => await nickname.value() === workerOriginal.nickname, '取消后恢复已保存昵称')
  await mini.reLaunch('/pages/profile/index'); page = await pageAt('pages/profile/index')
  assert.equal(await (await until(() => page.$('.profile-form input'), '重开昵称输入框')).value(), workerOriginal.nickname, '再次打开应恢复已保存昵称')
  assertProfileUnchanged(await readApi('/me'))
  passed('空昵称与一字昵称均未保存，取消并重开恢复原资料；原生错误提示已截图')
  page = await navigate('我的', 'pages/me/index')
  if (!workerOriginal.provider_enabled) assert.ok(workerOriginal.bio?.trim(), '原始资料无介绍，无法安全启用接单身份')
  await setProvider(true)
  await setProvider(false)
  page = await navigate('打手', 'pages/worker/index')
  await until(async () => { const state = await element(page, '.worker-state'); return state && (await state.text()).includes('未开启') }, '关闭接单身份显示未开启')
  await (await page.$('.publish-action')).tap()
  page = await pageAt('pages/publish/index')
  assert.ok(await until(() => page.$('.preview'), '关闭身份仍可制作服务草稿'))
  assert.equal((await readApi('/me')).provider_enabled, false, '打开草稿不应自动开启接单身份')
  page = await navigate('打手', 'pages/worker/index')
  await (await page.$('.orders-action')).tap()
  page = await pageAt('pages/orders/index')
  const whileDisabled = await checkOrders(page, 'provider', 'working', '我接的单', '进行中')
  assert.deepEqual(whileDisabled.map(order => order.id), workingOrders.map(order => order.id), '关闭接单身份不应移除已有工作订单')
  await (await page.$('.order-card')).tap()
  const detail = await pageAt('pages/order/index')
  assert.equal(detail.query.id, workingOrders[0].id)
  await until(() => detail.$('.chat-panel'), '关闭接单身份后仍可访问已有订单聊天室')
  assert.equal(await detail.$('.error-panel'), null)
  page = await navigate('我的', 'pages/me/index')
  await setProvider(workerOriginal.provider_enabled)
  await screenshot('mini-me')
  passed('接单身份关闭后仍可查看进行中订单及聊天室，原开关已恢复')
  for (const id of ['admin', 'player', 'clubowner']) {
    await login(id)
    page = await pageAt('pages/me/index')
    await until(() => page.$('.menu-list'), '个人菜单 ' + id)
    const entry = await page.$('.admin-entry')
    if (id === 'admin') {
      assert.ok(entry, '管理员必须看到平台管理入口')
      await reveal(entry); await entry.tap()
      const admin = await pageAt('pages/admin/index')
      await until(() => admin.$('.admin-tabs'), '平台管理界面')
    } else assert.equal(entry, null, '非平台管理员不能看到管理入口：' + id)
  }
  passed('平台管理员具有管理入口；玩家和俱乐部管理员无平台管理入口')
  assert.deepEqual(report.exceptions, [], '小程序运行异常')
  assert.deepEqual(report.console_errors, [], '小程序控制台错误')
  report.passed = true
}
run().catch(async error => {
  report.error = error.stack
  console.error(error.stack)
  process.exitCode = 1
  if (mini) {
    try { report.failed_page = (await mini.currentPage()).path; await screenshot('mini-navigation-failure', false) } catch (captureError) { report.capture_error = captureError.message }
  }
}).finally(async () => {
  try { await restoreWorker() }
  catch (error) { report.passed = false; report.restore_error = error.stack; process.exitCode = 1; console.error('恢复原资料失败：' + error.message) }
  if (report.exceptions.length || report.console_errors.length) { report.passed = false; process.exitCode = 1 }
  fs.mkdirSync(directory, { recursive: true })
  fs.writeFileSync(path.join(directory, 'mini-navigation-report.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ passed: report.passed, coverage: report.coverage, steps: report.steps, restored_original_profile: report.restored_original_profile, exceptions: report.exceptions, console_errors: report.console_errors }, null, 2))
  if (mini) await mini.disconnect()
})
