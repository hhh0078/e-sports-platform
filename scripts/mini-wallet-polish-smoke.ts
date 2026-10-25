/** 文件作用：仅用微信官方 SDK 和本地开发身份验收资金真实浮层、紧凑钱包和最低押金补缴，禁止支付或转账。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { InputElement, ScrollViewElement } from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { AuthSession, Club, ClubDetail, Service, User } from '../frontend/src/types/models'
import type { Wallet, WalletTopup, WalletWithdrawal, WalletDepositRefund, ClubDeposits, FinancialSettings } from '../frontend/src/types/wallet'

const root = process.cwd(), api = process.env.UI_TEST_API || 'http://127.0.0.1:18091/api'
const output = path.resolve(process.env.UI_TEST_OUTPUT || 'output/wallet-polish/mini-390')
const project = path.resolve(process.env.UI_MINIPROGRAM_PROJECT || 'frontend/dist/local/mp-weixin')
const require = createRequire(path.join(root, 'frontend/package.json'))
const automator = require('miniprogram-automator') as { connect(options: { wsEndpoint: string }): Promise<MiniProgram> }
const checks: string[] = [], exceptions: string[] = [], colorWarnings: string[] = [], forbidden: string[] = [], screenshots: string[] = [], remaining: string[] = [], mocks: string[] = []
const navigationChecks: Array<{ step: string; expected_route: string; actual_route: string }> = []
let mini: MiniProgram | undefined, localVerified = false, viewport: { windowWidth: number; windowHeight: number } | undefined
let original: { token: string; value: FinancialSettings } | undefined, failure: unknown, restored = false
/** 受控逻辑层表达式仅读取官方窗口，不写页面资料、缓存或会话。 */
declare const wx: { getWindowInfo(): { windowWidth: number; windowHeight: number } }
/** 原生资金能力仅记录到 SDK 绑定，绝不实际拉起支付或转账。 */
declare function walletPolishForbidden(method: string): void
/** 取得当前唯一官方 SDK 连接。 */
function sdk(): MiniProgram { assert.ok(mini); return mini }
/** 清除错误证据中可能出现的认证字段，避免报告保存会话或微信身份。 */
function redacted(value: unknown): string { return String(value).replace(/(token|secret|openid|authorization)["'\s:=]+[^\s,}"']+/gi, '$1=[REDACTED]').slice(0, 1200) }
/** 等待可观察界面状态，失败保留原步骤而不修改页面内部数据。 */
async function until<T>(read: () => Promise<T | false | null | undefined>, label: string, timeout = 12000): Promise<T> {
  const start = Date.now()
  while (Date.now() - start < timeout) { const value = await read(); if (value) return value; await new Promise(resolve => setTimeout(resolve, 150)) }
  throw new Error('等待超时：' + label)
}
/** 只请求固定本地 API，并在读取 bootstrap 确认开发模式之后才允许身份或设置操作。 */
async function call<T>(route: string, token = '', method = 'GET', body?: unknown): Promise<T> {
  assert.match(api, /^http:\/\/(127\.0\.0\.1|localhost):18091\/api$/)
  assert.ok(localVerified || route === '/bootstrap' && method === 'GET', '尚未验证本地开发模式')
  const response = await fetch(api + route, { method, redirect: 'error', headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) })
  const envelope = await response.json() as { data: T; error?: string }
  assert.ok(response.ok, route + ' ' + response.status + ' ' + redacted(envelope.error || ''))
  return envelope.data
}
/** 沿公开自定义组件边界查找，不调用组件业务函数。 */
async function child(scope: Page | Element, names: string[]): Promise<Element | null> { for (const name of names) { const item = await scope.$(name); if (item) return item } return null }
/** 在已知模板及插槽作用域中查询可见节点。 */
async function find(scopes: Array<Page | Element>, selector: string): Promise<Element | null> { for (const scope of scopes) { const item = await scope.$(selector); if (item) return item } return null }
/** 只导航一次；校验官方回包及转场后的真实路由，错误路径立即失败，不补发导航。 */
async function open(url: string): Promise<Page> {
  console.log('OPEN ' + url)
  const expected = url.split('?')[0].slice(1), page = await sdk().reLaunch(url); assert.ok(page)
  navigationChecks.push({ step: 'reLaunch返回', expected_route: expected, actual_route: page.path }); assert.equal(page.path, expected)
  await page.waitFor(400)
  const current = await sdk().currentPage(); navigationChecks.push({ step: '转场稳定', expected_route: expected, actual_route: current?.path || '' }); assert.equal(current?.path, expected, '转场后出现非预期路由，禁止重试掩盖')
  return page
}
/** 将真实节点的几何信息转换为数值。 */
async function rect(item: Element): Promise<{ left: number; top: number; width: number; height: number }> { const [offset, size] = await Promise.all([item.offset(), item.size()]); return { left: Number(offset.left), top: Number(offset.top), width: Number(size.width), height: Number(size.height) } }
/** 核对实际节点横向边界，隐藏或销毁的节点不作为通过依据。 */
async function within(scopes: Array<Page | Element>, selector: string): Promise<void> {
  const item = await until(() => find(scopes, selector), '边界节点 ' + selector), area = await rect(item)
  assert.ok(area.width > 0 && area.height > 0, selector + '未可见')
  assert.ok(area.left >= -1 && area.left + area.width <= Number(viewport?.windowWidth) + 1, selector + '横向溢出：' + JSON.stringify(area))
}
/** 滚动普通页面至真实操作位置，不使用页面业务方法。 */
async function show(item: Element): Promise<void> { const page = await sdk().currentPage(); assert.ok(page); const area = await rect(item); await sdk().pageScrollTo(Math.max(0, Number(await page.scrollTop()) + area.top - 160)); await page.waitFor(150) }
/** 保存实际 SDK 截图；工具不支持截图时明确记录限制，由根代理补原生截图。 */
async function record(label: string, image?: string): Promise<void> {
  checks.push(label); console.log('PASS ' + label)
  if (image) { await new Promise(resolve => setTimeout(resolve, 400)); const file = path.join(output, image + '.png'); try { await sdk().screenshot({ path: file }); screenshots.push(file) } catch (error) { remaining.push('SDK 截图未成功 ' + image + '：' + redacted(error)) } }
}
/** 通过真实登录页主动勾选协议并选择从服务端获得的体验昵称，不注入任何会话。 */
async function login(session: AuthSession): Promise<void> {
  const page = await open('/pages/account/login')
  await until(async () => { const primary = await page.$('.login-primary'), dev = await page.$('.dev-toggle'); return primary && dev && ![true, 'true'].includes(await primary.property('disabled')) && ![true, 'true'].includes(await dev.property('disabled')) ? true : null }, '登录协议及体验身份已完成读取')
  await until(async () => (await page.$$('.agreement-link') as Element[]).length === 2, '两份登录协议')
  await (await until(() => page.$('.consent-checkbox-label'), '主动勾选登录协议')).tap()
  await (await until(() => page.$('.dev-toggle'), '开发体验入口')).tap()
  const account = await until(async () => { for (const item of await page.$$('.account-card') as Element[]) { const name = await item.$('.account-name'); if (name && (await name.text()).trim() === session.user.nickname) return item } return null }, '服务端对应体验身份')
  const scroll = await until(() => page.$('.accounts-scroll'), '体验账号滚动区') as ScrollViewElement, bounds = await rect(scroll), target = await rect(account)
  if (target.top < bounds.top || target.top + target.height > bounds.top + bounds.height) { await scroll.scrollTo(0, Math.max(0, Number(await scroll.property('scrollTop')) + target.top - bounds.top - 8)); await page.waitFor(250) }
  const visible = await rect(account); assert.ok(visible.width > 0 && visible.height > 0 && visible.top >= bounds.top - 1 && visible.top + visible.height <= bounds.top + bounds.height + 1, '体验账号卡必须实际位于弹层滚动区内')
  await account.tap()
  let skipped = false
  const main = await until(async () => {
    const current = await sdk().currentPage(); if (!current) return null
    assert.ok(['pages/account/login', 'pages/main/index'].includes(current.path), '登录回跳出现非预期路由：' + current.path)
    if (current.path === 'pages/main/index') return current
    const error = await page.$('.login-error'); assert.equal(error, null, error ? await error.text() : '')
    const later = !skipped ? await page.$('.finish-later') : null
    if (later) { const area = await rect(later); if (area.width > 0 && area.height > 0) { skipped = true; await later.tap() } }
    return null
  }, '单次跳过可选资料并真实回到主页面')
  const view = await until(async () => {
    const current = await sdk().currentPage(); if (!current) return null; assert.equal(current.path, 'pages/main/index', '等待登录身份时出现非预期路由')
    const scrolls = await main.$$('.main-scroll') as ScrollViewElement[]; if (scrolls.length !== 4 || (await scrolls[3].attribute('class')).includes('main-scroll-inactive')) return null
    const value = await child(main, ['me-view', 'pages\\/main\\/me\\/me']), name = value ? await value.$('.profile-name') : null
    return name && (await name.text()).trim() === session.user.nickname ? value : null
  }, '我的视图激活并展示本次登录昵称')
  await main.waitFor(400)
  const settled = await sdk().currentPage(); navigationChecks.push({ step: '登录身份及转场稳定', expected_route: 'pages/main/index', actual_route: settled?.path || '' }); assert.equal(settled?.path, 'pages/main/index', '登录后迟到点击改变路由')
  assert.equal((await (await until(() => view.$('.profile-name'), '稳定后的登录昵称')).text()).trim(), session.user.nickname, '稳定后账号身份变化')
}
/** 使用真实退出入口进入访客视图，检查钱包零态和商户配置隔离。 */
async function guest(): Promise<void> {
  const page = await open('/pages/account/profile'), logout = await page.$('.logout-button')
  if (logout) { await show(logout); await logout.tap(); await until(async () => (await sdk().currentPage())?.path === 'pages/main/index', '退出会话回跳') }
  const { view } = await me()
  assert.match(await (await until(() => view.$('.profile-name'), '访客名称')).text(), /登录后查看/)
  assert.equal(await (await until(() => view.$('.wallet-main-value'), '访客余额')).text(), '¥0.00')
  assert.doesNotMatch(await view.text(), /支付配置|商户配置|平台管理中心/)
  await record('访客完整显示个人中心零数据，不展示商户配置', 'guest-me')
}
/** 切入主页面我的视图并等待真实身份资料加载。 */
async function me(): Promise<{ page: Page; view: Element }> { const page = await open('/pages/main/index?tab=me'), view = await until(() => child(page, ['me-view', 'pages\\/main\\/me\\/me']), '个人中心视图'); await until(() => view.$('.me-wallet'), '紧凑钱包卡'); return { page, view } }
/** 核验账户可消费合计、可提现说明和三类在途资金，普通用户不展示商户配置。 */
async function compact(session: AuthSession, slug: string): Promise<void> {
  const expected = await call<Wallet>('/wallet', session.token), { view } = await me()
  assert.equal(expected.spendable_cents, expected.available_cents + expected.recharge_cents, '账户合计与收入/充值分区不符')
  const main = await until(() => view.$('.wallet-main-value'), '账户余额')
  await until(async () => (await main.text()) === '¥' + (expected.spendable_cents / 100).toFixed(2), '个人中心真实账户合计')
  assert.equal(await (await until(() => view.$('.wallet-caption'), '账户合计标题')).text(), '账户余额')
  assert.equal(await (await until(() => view.$('.wallet-balance-note'), '可提现限制说明')).text(), '其中可提现 ¥' + (expected.available_cents / 100).toFixed(2) + '，充值仅用于消费')
  const card = await until(() => view.$('.me-wallet'), '钱包卡'), area = await rect(card), parts = await view.$$('.wallet-item') as Element[]
  assert.ok(area.height <= 170, '个人中心钱包卡过高：' + area.height); assert.equal(parts.length, 3)
  for (const [index, key] of (['ongoing_cents', 'settling_cents', 'withdrawal_frozen_cents'] as const).entries()) { const amount = expected[key]; assert.ok(typeof amount === 'number'); assert.ok((await parts[index].text()).includes('¥' + (amount / 100).toFixed(2))) }
  await within([view], '.me-wallet'); await within([view], '.wallet-available'); await within([view], '.wallet-balance-note')
  assert.equal(await view.$('.wallet-recharge'), null, '账户合计下不再重复显示充值金额行')
  if (session.user.role !== 'admin') assert.doesNotMatch(await view.text(), /支付配置|商户配置|平台管理中心/)
  await record(slug + '个人中心钱包≤170px，合计账户余额、可提现说明和三类在途资金清晰', 'me-' + slug)
}
/** 取得资金表单、共享 FinancePopup 和官方 UniPopup 的真实组件实例。 */
async function popup(page: Page, form = true, ruleScope?: Element): Promise<{ scopes: Array<Page | Element>; finance: Element; native: Element; sheet: Element }> {
  const host = ruleScope || (form ? await until(() => child(page, ['funds-form', 'components\\/FundsForm']), '资金表单组件') : page)
  const finance = await until(() => child(host, ['finance-popup', 'components\\/FinancePopup']), '共享资金弹框组件')
  const native = await until(() => child(finance, ['uni-popup', 'node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup']), '官方 UniPopup')
  const scopes: Array<Page | Element> = [host, finance, native, page], sheet = await until(() => find(scopes, '.finance-popup-sheet'), '真实资金弹框内容')
  await until(async () => (await rect(sheet)).width > 0, '弹框打开动画完成')
  const sheetArea = await rect(sheet)
  assert.ok(sheetArea.height <= Number(viewport?.windowHeight) * 0.82 + 1, '资金弹框超过82vh：' + sheetArea.height)
  assert.ok(sheetArea.top >= -1 && sheetArea.top + sheetArea.height <= Number(viewport?.windowHeight) + 1, '资金弹框越出窗口')
  const official = await until(() => native.$('.uni-popup'), '官方固定定位根节点')
  assert.equal(await official.style('position'), 'fixed', '弹框必须脱离普通文档流')
  let transitions = await native.$$('uni-transition') as Element[]
  if (!transitions.length) transitions = await native.$$('node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-transition\\/uni-transition') as Element[]
  assert.ok(transitions.length >= 2, '官方遮罩和内容过渡必须同时存在')
  const mask = await until(() => transitions[0].$('view'), '官方遮罩实际视图'), maskArea = await rect(mask)
  assert.equal(await mask.style('position'), 'fixed'); assert.ok(Math.abs(maskArea.width - Number(viewport?.windowWidth)) <= 1, '遮罩未覆盖窗口宽度')
  await within(scopes, '.finance-popup-sheet'); await within(scopes, '.finance-popup-close')
  const primary = await find(scopes, form ? '.funds-form-actions .btn' : '.withdrawal-rules-close')
  if (primary) assert.equal(Math.round((await rect(primary)).height), 44, '资金弹框主按钮应为44px')
  return { scopes, finance, native, sheet }
}
/** 关闭真实浮层并等待官方关闭动画结束，不修改父页面 operation 状态。 */
async function closePopup(value: Awaited<ReturnType<typeof popup>>): Promise<void> { await (await until(() => find(value.scopes, '.finance-popup-close'), '关闭资金浮层')).tap(); await until(async () => !await value.native.$('.uni-popup'), '官方资金浮层已移除') }
/** 打开规则并验证官方遮罩、不推挤正文和长内容可滚动关闭。 */
async function rules(page: Page, anchorSelector: string, club = false): Promise<void> {
  const anchor = await until(() => page.$(anchorSelector), '正文布局锚点'), before = await rect(anchor)
  const rule = await until(() => child(page, ['withdrawal-rules', 'components\\/WithdrawalRules']), '共享提现规则')
  await (await until(() => rule.$('.withdrawal-rules-link'), '提现规则入口')).tap()
  const value = await popup(page, false, rule), text = await value.sheet.text()
  assert.match(text, /0元起提现/); assert.match(text, /每天可提现一次/); assert.match(text, club ? /本俱乐部已结算佣金/ : /充值余额仅用于消费/)
  const after = await rect(anchor); assert.equal(Math.round(after.height), Math.round(before.height)); assert.equal(Math.round(after.top), Math.round(before.top), '规则弹框推挤正文')
  await record((club ? '俱乐部' : '个人') + '提现规则为官方固定浮层，不推挤正文', club ? 'club-rules' : 'personal-rules')
  const scroll = await find(value.scopes, '.finance-popup-scroll') as ScrollViewElement | null; assert.ok(scroll)
  const close = await until(() => find(value.scopes, '.withdrawal-rules-close'), '规则我知道了'), scrollBefore = await rect(scroll), closeBefore = await rect(close)
  await scroll.scrollTo(0, 100000); await page.waitFor(250)
  const scrollAfter = await rect(scroll), closeAfter = await rect(close)
  assert.ok(closeAfter.top >= scrollAfter.top - 1 && closeAfter.top + closeAfter.height <= scrollAfter.top + scrollAfter.height + 1, '规则底部按钮未滚入scroll-view可见区域')
  assert.ok(closeAfter.top + closeAfter.height <= Number(viewport?.windowHeight) + 1, '规则底部按钮仍在窗口外')
  if (closeBefore.top + closeBefore.height > scrollBefore.top + scrollBefore.height + 1) assert.ok(closeAfter.top < closeBefore.top - 1, '长规则没有发生真实内部滚动')
  await record((club ? '俱乐部' : '个人') + '规则底部按钮已真实滚入可见区域后关闭', club ? 'club-rules-bottom' : 'personal-rules-bottom')
  await close.tap(); await until(async () => !await value.native.$('.uni-popup'), '规则确认关闭')
}
/** 读取资金记录的脱敏业务编号，比较取消或低额提交前后没有新资金单。 */
async function records(session: AuthSession): Promise<string[]> {
  const [payments, withdrawals, refunds] = await Promise.all([call<WalletTopup[]>('/wallet/topups', session.token), call<WalletWithdrawal[]>('/wallet/withdrawals', session.token), call<WalletDepositRefund[]>('/wallet/deposit-refunds', session.token)])
  return [...payments.map(v => 'p:' + v.id), ...withdrawals.map(v => 'w:' + v.id), ...refunds.map(v => 'r:' + v.id)].sort()
}
/** 按真实金额加载个人钱包，并分别打开充值、提现、押金与退款表单后关闭。 */
async function personal(session: AuthSession, shortfall: number, slug: string): Promise<void> {
  const expected = await call<Wallet>('/wallet', session.token), page = await open('/pages/account/wallet')
  await until(() => page.$('.dev-note'), '本地钱包已加载'); assert.equal(await page.$('.error-box'), null)
  assert.equal(await (await until(() => page.$('.balance-caption'), '账户合计标题')).text(), '账户余额')
  assert.equal(await (await until(() => page.$('.balance-value'), '账户合计金额')).text(), '¥' + (expected.spendable_cents / 100).toFixed(2))
  assert.equal(await (await until(() => page.$('.balance-availability'), '钱包可提现说明')).text(), '其中可提现 ¥' + (expected.available_cents / 100).toFixed(2) + '，充值仅用于消费')
  assert.equal(expected.deposit_shortfall_cents, shortfall)
  await rules(page, '.balance-section')
  const before = await records(session)
  for (const [selector, label] of [['.wallet-actions .btn-secondary', '充值'], ['.wallet-actions .btn', '提现'], ['.deposit-main .text-btn', '押金'], ['.refund-entry .text-btn', '退押金']] as const) {
    let entry = await page.$(selector)
    if (label === '提现') { entry = null; for (const item of await page.$$('.wallet-actions .btn') as Element[]) if ((await item.text()).trim() === '提现') entry = item }
    if (!entry) { assert.equal(label, '退押金'); assert.equal(expected.deposit_cents, 0); remaining.push(slug + '原押金为0，退还入口不可用；未制造资金'); continue }
    await show(entry); const anchor = await rect(await until(() => page.$('.ledger-section'), '交易明细锚点')); await entry.tap(); const value = await popup(page)
    assert.match(await value.sheet.text(), new RegExp(label === '押金' ? '平台押金' : label === '退押金' ? '退还押金' : label))
    const after = await rect(await until(() => page.$('.ledger-section'), '弹框后明细锚点')); assert.equal(Math.round(after.top), Math.round(anchor.top), label + '弹框推挤正文')
    await within(value.scopes, '.funds-amount-input'); await within(value.scopes, '.funds-form-actions')
    if (label === '押金') {
      const input = await until(() => find(value.scopes, '.funds-amount-input'), '补缴金额') as InputElement
      assert.equal(await input.property('value'), (shortfall / 100).toFixed(2))
      await input.input(((shortfall - 1) / 100).toFixed(2)); await (await until(() => find(value.scopes, '.funds-form-actions .btn'), '提交不足差额')).tap()
      const error = await until(() => find(value.scopes, '.funds-form-error'), '低额补缴错误'); assert.match(await error.text(), /最低押金|差额|补缴/)
      assert.equal(await input.property('value'), ((shortfall - 1) / 100).toFixed(2), '拒绝后丢失金额草稿')
      await record(slug + '默认补缴' + (shortfall / 100).toFixed(2) + '元，低一分拒绝且保留草稿', 'personal-deposit')
    }
    await record(slug + label + '表单使用固定浮层且不推挤页面'); await closePopup(value)
  }
  assert.deepEqual(await records(session), before, '打开/取消或不足额提交产生了资金单')
}
/** 验证组织平台押金补缴默认差额、低额拒绝、退款弹框与规则。 */
async function clubFunds(owner: AuthSession, club: Club): Promise<void> {
  const expected = await call<ClubDeposits>('/clubs/' + club.id + '/deposits', owner.token), page = await open('/pages/community/clubs/funds?id=' + encodeURIComponent(club.id))
  await until(() => page.$('.funds-shortfall'), '组织资金已加载且显示补缴差额'); assert.equal(await page.$('.error-box'), null); assert.equal(expected.platform.shortfall_cents, 234)
  await rules(page, '.funds-balance', true)
  const before = await records(owner), entry = await until(() => page.$('.funds-actions .btn'), '补缴平台押金入口'); await show(entry); await entry.tap()
  const value = await popup(page), input = await until(() => find(value.scopes, '.funds-amount-input'), '组织补缴金额') as InputElement
  assert.equal(await input.property('value'), '2.34'); await input.input('2.33'); await (await until(() => find(value.scopes, '.funds-form-actions .btn'), '组织低额提交')).tap()
  if (owner.user.phone_verified) {
    assert.match(await (await until(() => find(value.scopes, '.funds-form-error'), '组织最低额错误')).text(), /最低押金|差额|补缴/)
    await record('俱乐部默认补缴2.34元，2.33元拒绝且未生成付款单', 'club-deposit')
  } else {
    await cancelPhoneGate(page, value, input, owner, '2.33', '俱乐部补缴')
    remaining.push('组织负责人未验证手机号，低额提交在手机号门槛前取消；最低额后端拒绝已由独立真实PG集成测试覆盖。')
    await record('俱乐部默认补缴2.34元，未验证手机号取消后保留2.33元且无付款单', 'club-deposit')
  }
  await closePopup(value)
  const refund = await until(() => page.$('.funds-actions .text-btn'), '组织退押金入口')
  if (expected.platform.amount_cents > 0) { await refund.tap(); const returned = await popup(page); assert.match(await returned.sheet.text(), /退还押金/); await closePopup(returned); await record('俱乐部原路退押金为真实浮层，仅打开后取消') } else assert.ok([true, 'true'].includes(await refund.property('disabled')), '无押金退款须禁用')
  assert.deepEqual(await records(owner), before, '组织取消或低额提交产生资金单')
}
/** 对可操作的成员押金页核验共享浮层；现有规则不要求押金时如实记录不可操作状态。 */
async function memberDeposit(worker: AuthSession, clubs: Club[]): Promise<void> {
  const requested = process.env.UI_TEST_MEMBER_CLUB_ID, chosen = requested ? clubs.find(v => v.id === requested) : clubs.find(v => v.deposit_cents > 0)
  const club = chosen || clubs[0]; assert.ok(club, '没有可浏览的本地俱乐部')
  const expected = await call<ClubDeposits>('/clubs/' + club.id + '/deposits', worker.token), page = await open('/pages/community/clubs/deposit?id=' + encodeURIComponent(club.id))
  await until(() => page.$('.member-amount'), '成员押金余额'); assert.equal(await page.$('.error-box'), null); await within([page], '.member-balance')
  const before = await records(worker)
  for (const [selector, available, label] of [['.member-pay', expected.membership.required_cents > expected.membership.amount_cents, '缴纳'], ['.member-refund', expected.membership.amount_cents > 0, '退还']] as const) {
    const entry = await until(() => page.$(selector), '成员押金' + label)
    if (!available) { assert.ok([true, 'true'].includes(await entry.property('disabled'))); remaining.push('成员押金' + label + '：现有余额/规则下入口禁用，未制造资金'); continue }
    await show(entry); await entry.tap(); const value = await popup(page); assert.match(await value.sheet.text(), /押金/); await within(value.scopes, '.funds-amount-input'); await record('成员押金' + label + '复用官方浮层，仅打开后取消', 'member-' + label); await closePopup(value)
  }
  assert.deepEqual(await records(worker), before)
}
/** 公开服务与俱乐部均使用新门槛撤标，原金额和差额由本人资金接口返回。 */
async function publicBadges(serviceID: string, clubID: string): Promise<void> {
  const service = await call<Service>('/services/' + serviceID); assert.equal(service.provider_deposit_cents, 0)
  const page = await open('/pages/services/detail?id=' + encodeURIComponent(serviceID)); await until(() => page.$('.detail-title'), '公开服务已加载')
  const shell = await child(page, ['app-shell', 'components\\/AppShell']), scopes = [page, ...(shell ? [shell] : [])], deposits = await find(scopes, '.detail-deposits')
  if (deposits) assert.doesNotMatch(await deposits.text(), /平台押金/)
  const detail = await call<ClubDetail>('/clubs/' + clubID); assert.equal(detail.club.platform_deposit_cents, 0); assert.equal(detail.club.platform_deposit_shortfall_cents, 234)
  await record('不足额服务和俱乐部公开撤标，本人原押金继续保留', 'service-insufficient')
}
/** 普通身份尝试进入配置路由时，真实页面不渲染商户/交易规则输入字段。 */
async function denied(): Promise<void> {
  for (const route of ['/pages/manage/finance/settings', '/pages/manage/wallet', '/pages/manage/settings']) {
    const page = await open(route), empty = await until(async () => await page.$('.access-note') || await child(page, ['empty-state', 'components\\/EmptyState']), '普通用户配置拒绝')
    assert.match(await empty.text(), /管理员/); assert.equal((await page.$$('input') as Element[]).length, 0)
  }
  await record('普通玩家不能查看商户支付、转账或最低押金配置')
}
/** 取消覆盖在资金弹框上层的手机号验证，核验草稿与未验证身份不变，不点任何真实或模拟授权。 */
async function cancelPhoneGate(page: Page, value: Awaited<ReturnType<typeof popup>>, input: InputElement, session: AuthSession, amount: string, label: string): Promise<void> {
  const shell = await until(() => child(page, ['app-shell', 'components\\/AppShell']), '页面公共壳')
  const phone = await until(() => child(shell, ['phone-verification', 'components\\/PhoneVerification']), '手机号验证组件')
  const native = await until(() => child(phone, ['uni-popup', 'node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup']), '手机号官方弹层')
  const scopes: Array<Page | Element> = [phone, native, shell, page], sheet = await until(() => find(scopes, '.phone-sheet'), '手机号浮层可见')
  assert.ok((await rect(sheet)).height > 0); await within(scopes, '.phone-sheet')
  const phoneLayer = await until(() => find(scopes, '.phone-layer'), '手机号根层'), financeLayer = await until(() => find(value.scopes, '.finance-popup-layer'), '资金根层')
  assert.ok(Number(await phoneLayer.style('z-index')) > Number(await financeLayer.style('z-index')), '手机号验证被忙碌资金表单遮住')
  await record(label + '手机号官方浮层高于忙碌资金表单且可见', 'phone-over-funds-' + session.user.id)
  await (await until(() => find(scopes, '.phone-heading .icon-btn'), '取消手机号验证')).tap()
  await until(async () => !await native.$('.uni-popup'), '手机号浮层取消移除')
  const submit = await until(() => find(value.scopes, '.funds-form-actions .btn'), '手机号取消后原资金按钮')
  await until(async () => ![true, 'true'].includes(await submit.property('disabled')), '资金表单解除忙碌')
  assert.equal(await input.property('value'), amount)
  assert.equal((await call<User>('/me', session.token)).phone_verified, false, '取消时不应绑定真实或模拟手机号')
  await record(label + '取消手机号验证保留原金额且仍未验证')
}
/** 从未验证体验身份的个人充值表单触发手机号浮层，取消后核对没有任何新增资金单。 */
async function phoneCancel(): Promise<void> {
  const accounts = await call<User[]>('/dev/accounts'), user = accounts.find(value => value.id === 'outsider' && !value.phone_verified) || accounts.find(value => !value.phone_verified)
  if (!user) { remaining.push('固定5个体验账号均已验证手机号，未伪造第6个登录身份；手机号浮层取消路径需独立夹具。'); return }
  const session = await call<AuthSession>('/auth/dev', '', 'POST', { user_id: user.id }); await login(session)
  const before = await records(session), page = await open('/pages/account/wallet'); await until(() => page.$('.dev-note'), '未验证用户钱包已加载')
  await (await until(() => page.$('.wallet-actions .btn-secondary'), '未验证用户充值入口')).tap()
  const value = await popup(page), input = await until(() => find(value.scopes, '.funds-amount-input'), '未验证用户金额') as InputElement
  await input.input('1.00'); await (await until(() => find(value.scopes, '.funds-form-actions .btn'), '提交触发手机号验证')).tap()
  await cancelPhoneGate(page, value, input, session, '1.00', '个人充值'); assert.deepEqual(await records(session), before)
  await closePopup(value); await record('取消个人充值手机号验证没有新增资金单')
}
/** 拦截测试不应调用的原生资金能力，出现任何调用即验收失败。 */
async function nativeGuards(): Promise<void> {
  await sdk().exposeFunction('walletPolishForbidden', (method: unknown) => forbidden.push(String(method)))
  for (const method of ['requestPayment', 'requestMerchantTransfer']) { await sdk().mockWxMethod(method, (options: { fail?: (result: { errMsg: string }) => void; complete?: (result: { errMsg: string }) => void }, name: string) => { walletPolishForbidden(name); const result = { errMsg: name + ':fail UI smoke blocked' }; options.fail?.(result); options.complete?.(result) }, method); mocks.push(method) }
}
/** 输出脱敏断言、截图及真实微信能力未测试的明确边界。 */
function report(): void { fs.mkdirSync(output, { recursive: true }); fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify({ passed: !failure && restored, project, api, package_mode: '本地API构建复制至实际微信工程；不是生产接口或已上传包', viewport, checks, screenshots, navigation_checks: navigationChecks, runtime_exceptions: exceptions, canvas_color_warnings: colorWarnings, forbidden_native_calls: forbidden, original_financial_settings_restored: restored, remaining: [...remaining, '本地 SDK 未验证真实微信支付、退款、提现到账；本次未创建资金交易。'], ...(failure ? { error: redacted(failure instanceof Error ? failure.message : failure) } : {}) }, null, 2) + '\n') }
/** 执行四身份与两类押金规则验收，所有临时财务设置在 finally 恢复原值。 */
async function run(): Promise<void> {
  fs.mkdirSync(output, { recursive: true }); assert.equal((await call<{ dev_mode: boolean }>('/bootstrap')).dev_mode, true); localVerified = true
  assert.match(fs.readFileSync(path.join(project, 'api/client.js'), 'utf8'), /127\.0\.0\.1:18091\/api/, '必须使用本地联调构建')
  const accounts: Record<string, AuthSession> = {}; for (const id of ['player', 'worker', 'clubowner', 'admin']) accounts[id] = await call<AuthSession>('/auth/dev', '', 'POST', { user_id: id })
  original = { token: accounts.admin.token, value: await call<FinancialSettings>('/admin/financial-settings', accounts.admin.token) }
  const clubs = await call<Club[]>('/clubs?mine=1', accounts.clubowner.token), club = clubs.find(v => v.owner_id === accounts.clubowner.user.id && v.status === 'active'); assert.ok(club)
  const [workerWallet, clubDeposit] = await Promise.all([call<Wallet>('/wallet', accounts.worker.token), call<ClubDeposits>('/clubs/' + club.id + '/deposits', accounts.clubowner.token)])
  await call('/admin/financial-settings', accounts.admin.token, 'PATCH', { ...original.value, personal_platform_deposit_cents: workerWallet.deposit_cents + 123, club_creation_deposit_cents: clubDeposit.platform.amount_cents + 234 })
  mini = await automator.connect({ wsEndpoint: process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9424' })
  mini.on('exception', (error: unknown) => exceptions.push(redacted(error)))
  mini.on('console', (event: unknown) => { const text = JSON.stringify(event); if (/不支持颜色|unsupported color|invalid color/i.test(text)) colorWarnings.push(redacted(text)) })
  viewport = await mini.evaluate(() => wx.getWindowInfo()) as { windowWidth: number; windowHeight: number }; if (process.env.UI_EXPECT_WIDTH) assert.equal(viewport.windowWidth, Number(process.env.UI_EXPECT_WIDTH)); assert.ok([320, 390].includes(viewport.windowWidth))
  await nativeGuards(); await login(accounts.worker); await guest()
  await phoneCancel()
  for (const id of ['player', 'worker', 'clubowner', 'admin']) { await login(accounts[id]); await compact(accounts[id], id) }
  await login(accounts.worker); await personal(accounts.worker, 123, '打手'); await memberDeposit(accounts.worker, clubs)
  await login(accounts.clubowner); await clubFunds(accounts.clubowner, club)
  await login(accounts.player); await denied(); await publicBadges('delta-service-1', club.id)
  await login(accounts.admin); const settings = await open('/pages/manage/finance/settings'); await until(async () => (await settings.$$('.finance-input') as Element[]).length === 4, '管理员四项交易规则'); await within([settings], '.finance-settings-save'); await record('管理员可见两类最低押金与两档平台抽成配置', 'admin-rules')
  assert.deepEqual(exceptions, []); assert.deepEqual(colorWarnings, []); assert.deepEqual(forbidden, [])
}
run().catch(async error => { failure = error; console.error(redacted(error)); if (mini) try { await sdk().screenshot({ path: path.join(output, 'failure.png') }) } catch { /* 截图失败仍保留原始失败断言。 */ } }).finally(async () => {
  if (original) try { await call('/admin/financial-settings', original.token, 'PATCH', original.value); const after = await call<FinancialSettings>('/admin/financial-settings', original.token); assert.deepEqual(after, original.value); restored = true } catch (error) { failure ||= error; console.error('恢复本地原财务配置失败：' + redacted(error)) }
  for (const method of mocks) try { await mini?.restoreWxMethod(method) } catch (error) { failure ||= error }
  await mini?.disconnect(); report(); if (failure) process.exitCode = 1; else console.log('通过 ' + checks.length + ' 项，原财务设置已恢复')
})
