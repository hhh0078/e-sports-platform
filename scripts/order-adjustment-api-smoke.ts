/** 文件作用：仅通过固定本机开发API验收订单补款、双方减价、原余额分区退款和四身份权限，默认正常全退，不写数据库或调用真实微信。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { randomUUID } from 'node:crypto'
import type { AuthSession, Club, ClubMember, Game, Order, OrderSettlement, Service } from '../frontend/src/types/models'
import type { AdjustmentPayment, CreateAdjustmentInput, OrderAdjustment } from '../frontend/src/types/adjustments'
import type { Wallet } from '../frontend/src/types/wallet'

/** 四个验收身份；会话令牌仅留在本进程内存。 */
type Role = 'player' | 'worker' | 'admin' | 'owner'
/** 无支付秘密的验收结果。 */
interface SmokeReport {
  run_id: string; created_at: string; api: string; dev_mode: boolean; real_wechat: false; title: string
  passed: boolean; keep_active: boolean; order_id?: string; order_status?: string; club_id: string
  amounts: { original: number; increase: number; buyer_decrease: number; worker_decrease: number; net: number }
  checks: string[]; cleanup_required: boolean; cleanup_complete: boolean; cleanup_steps?: string[]; error?: string
}
const base = 'http://127.0.0.1:18091/api', runID = new Date().toISOString().replace(/\D/g, '').slice(0, 14) + '-' + randomUUID().slice(0, 8)
const prefix = '调价验收' + runID, clubID = 'shanhai', serviceID = 'delta-service-1'
const keepRequested = process.env.SMOKE_KEEP_ACTIVE === 'true', directory = path.resolve(process.env.SMOKE_OUTPUT || 'output/order-adjustment-sharing/api-smoke')
const tokens: Partial<Record<Role, string>> = {}, sessions: Partial<Record<Role, AuthSession>> = {}
const amounts = { original: 100, increase: 101, buyer_decrease: 61, worker_decrease: 10, net: 130 }
const report: SmokeReport = { run_id: runID, created_at: new Date().toISOString(), api: base, dev_mode: false, real_wechat: false, title: prefix, passed: false, keep_active: false, club_id: clubID, amounts, checks: [], cleanup_required: false, cleanup_complete: false }
let fixture: Order | undefined, beforeBuyer: Wallet | undefined, beforeWorker: Wallet | undefined, beforeClub: Wallet | undefined

/** 原子保存公开夹具编号和验收结论，禁止持久化会话、OpenID、支付参数或响应原文。 */
function saveReport(): void {
  fs.mkdirSync(directory, { recursive: true })
  const target = path.join(directory, 'report.json'), temporary = target + '.tmp'
  fs.writeFileSync(temporary, JSON.stringify(report, null, 2) + '\n', { mode: 0o600 }); fs.renameSync(temporary, target)
}
/** 记录已经通过的断言，不打印鉴权请求头或私密响应。 */
function record(label: string): void { report.checks.push(label); saveReport(); console.log('PASS ' + label) }
/** 只向固定环回地址发起带超时的请求，拒绝跳转并按标准信封读取已知业务类型。 */
async function api<T>(role: Role | '', route: string, method = 'GET', body?: unknown, expected = 200): Promise<T> {
  assert.ok(route.startsWith('/') && !route.includes('://'), 'API路径必须为站内相对路径')
  const token = role ? tokens[role] : undefined
  const response = await fetch(base + route, { method, redirect: 'error', signal: AbortSignal.timeout(12000), headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) })
  assert.equal(response.status, expected, method + ' ' + route + ' 状态应为 ' + expected + '，响应正文不记录以防泄露')
  if (expected !== 200) { await response.arrayBuffer(); return undefined as T }
  const value: unknown = await response.json()
  assert.ok(value !== null && typeof value === 'object' && 'data' in value, 'API响应缺少data字段')
  return (value as { data: T }).data
}
/** 在会话仍属对应体验账号的前提下读取本次夹具，防止清理误触其他订单。 */
async function currentOrder(): Promise<Order> {
  assert.ok(fixture, '本次验收尚未创建订单')
  const order = await api<Order>('player', '/orders/' + fixture.id)
  assert.equal(order.id, fixture.id); assert.equal(order.title, prefix, '只能操作本次唯一前缀订单')
  fixture = order; report.order_status = order.status; return order
}
/** 统一调用本次订单履约动作，接口由后端检查真实身份和资金状态。 */
async function action(role: Role, name: string, note = ''): Promise<Order> {
  const order = await currentOrder()
  fixture = await api<Order>(role, '/orders/' + order.id + '/actions', 'POST', { action: name, note })
  return fixture
}
/** 按本次运行生成稳定幂等号，不与既有资金请求复用。 */
function requestID(suffix: string): string { return 'qa-adj-' + runID + '-' + suffix }
/** 生成调价载荷，所有金额为整数分。 */
function proposal(kind: 'increase' | 'decrease', amount: number, suffix: string): CreateAdjustmentInput {
  return { kind, amount_cents: amount, reason: prefix + ' 本机模拟协商费用，完成后按原支付渠道全退', request_id: requestID(suffix) }
}
/** 核对净价按原费率分账，金额总和严格守恒。 */
function checkSettlement(order: Order, original: OrderSettlement): void {
  assert.ok(order.settlement, '订单应有分账快照')
  const value = order.settlement
  assert.equal(value.platform_rate_bps, original.platform_rate_bps); assert.equal(value.club_rate_bps, original.club_rate_bps)
  assert.equal(value.platform_fee_cents, Math.floor(order.amount_cents * value.platform_rate_bps / 10000))
  assert.equal(value.club_fee_cents, Math.floor(order.amount_cents * value.club_rate_bps / 10000))
  assert.equal(value.provider_income_cents + value.club_fee_cents + value.platform_fee_cents, order.amount_cents)
}
/** 为四种身份核验订单与调价证据读取权限，访客不能读取私密订单。 */
async function checkParticipants(): Promise<void> {
  const order = await currentOrder()
  for (const role of ['player', 'worker', 'admin', 'owner'] as const) {
    assert.equal((await api<Order>(role, '/orders/' + order.id)).id, order.id)
    await api<OrderAdjustment[]>(role, '/orders/' + order.id + '/adjustments')
  }
  await api<unknown>('', '/orders/' + order.id, 'GET', undefined, 401)
  record('玩家、打手、平台管理员、俱乐部负责人可读所属订单证据，访客被拒绝')
}
/** 校验已支付余额、待结算净收入及组织佣金只发生本单应有变化。 */
async function checkBalances(net: number): Promise<void> {
  assert.ok(beforeBuyer && beforeWorker && beforeClub)
  const order = await currentOrder(), buyer = await api<Wallet>('player', '/wallet'), worker = await api<Wallet>('worker', '/wallet'), club = await api<Wallet>('owner', '/clubs/' + clubID + '/wallet')
  assert.ok(order.settlement)
  assert.equal(buyer.recharge_cents, beforeBuyer.recharge_cents - net, '充值原分区应只减少本单净价')
  assert.equal(buyer.available_cents, beforeBuyer.available_cents, '充值退款不得变成可提现余额')
  assert.equal(worker.pending_cents, beforeWorker.pending_cents + order.settlement.provider_income_cents)
  assert.equal(club.pending_cents, beforeClub.pending_cents + order.settlement.club_fee_cents)
}
/** 完成本地资金及权限验收；故意保持进行中只在全部检查通过时生效。 */
async function run(): Promise<void> {
  assert.ok(!process.env.SMOKE_API || process.env.SMOKE_API === base, '验收禁止更换固定本机API地址')
  const bootstrap = await api<{ dev_mode: boolean; games: Game[] }>('', '/bootstrap')
  assert.equal(bootstrap.dev_mode, true, '必须为显式本地开发模式'); report.dev_mode = true
  const service = await api<Service>('', '/services/' + serviceID), club = await api<{ club: Club; members: ClubMember[] }>('', '/clubs/' + clubID)
  assert.equal(service.status, 'active'); assert.equal(service.provider_id, 'worker'); assert.equal(service.club_id, clubID)
  assert.ok(bootstrap.games.some(game => game.id === service.game_id && game.enabled)); assert.equal(club.club.owner_id, 'clubowner'); assert.equal(club.club.status, 'active')
  assert.ok(club.members.some(member => member.user_id === 'worker' && member.status === 'active'))
  assert.equal(club.club.platform_deposit_required_cents || 0, 0, '本地验收不得自动缴纳俱乐部押金')
  record('固定127.0.0.1:18091且dev_mode=true，仅只读复用可用游戏及俱乐部')
  for (const [role, id] of [['player', 'player'], ['worker', 'worker'], ['admin', 'admin'], ['owner', 'clubowner']] as const) {
    const session = await api<AuthSession>('', '/auth/dev', 'POST', { user_id: id })
    assert.equal(session.user.id, id); assert.ok(typeof session.token === 'string' && session.token.length > 20, '体验会话无效')
    tokens[role] = session.token; sessions[role] = session
  }
  assert.ok(sessions.player?.user.phone_verified && sessions.worker?.user.phone_verified, '体验买家和打手须已有手机号验证，脚本不修改个人资料')
  beforeBuyer = await api<Wallet>('player', '/wallet'); beforeWorker = await api<Wallet>('worker', '/wallet'); beforeClub = await api<Wallet>('owner', '/clubs/' + clubID + '/wallet')
  assert.ok(beforeBuyer.recharge_cents >= amounts.original + amounts.increase, '已有充值余额不足2.01元，脚本不会自动充值或创建资金夹具')
  record('四身份本地登录，会话仅在内存；已有充值余额足够，不充值、不修改资料')
  fixture = await api<Order>('player', '/requests', 'POST', { game_id: service.game_id, title: prefix, requirements: prefix + ' 本机模拟验收需求，请勿实际履约，结束后全额退回原余额分区。', amount_cents: amounts.original })
  report.order_id = fixture.id; report.order_status = fixture.status; report.cleanup_required = true; saveReport()
  const initialPayment = await api<{ order: Order; mode: string }>('player', '/orders/' + fixture.id + '/pay', 'POST', { channel: 'balance', request_id: requestID('initial') })
  assert.equal(initialPayment.mode, 'balance'); assert.equal(initialPayment.order.payment_status, 'paid')
  fixture = await api<Order>('worker', '/orders/' + fixture.id + '/actions', 'POST', { action: 'claim', note: '', club_id: clubID }); assert.equal(fixture.club_id, clubID)
  await action('worker', 'start'); assert.equal(fixture.status, 'in_progress'); assert.ok(fixture.settlement)
  const originalSettlement = { ...fixture.settlement }
  record('唯一前缀公开需求已余额付款、由认证打手代表俱乐部接单并进入履约')
  await checkParticipants()
  const increaseInput = proposal('increase', amounts.increase, 'increase')
  for (const role of ['player', 'admin', 'owner'] as const) await api<unknown>(role, '/orders/' + fixture.id + '/adjustments', 'POST', increaseInput, 403)
  const increase = await api<OrderAdjustment>('worker', '/orders/' + fixture.id + '/adjustments', 'POST', increaseInput)
  assert.equal(increase.status, 'proposed'); assert.ok(increase.provider_agreed && !increase.buyer_agreed)
  for (const role of ['worker', 'admin', 'owner'] as const) await api<unknown>(role, '/orders/' + fixture.id + '/adjustments/' + increase.id + '/respond', 'POST', { action: 'accept' }, 403)
  await api<unknown>('worker', '/orders/' + fixture.id + '/adjustments/' + increase.id + '/pay', 'POST', { channel: 'balance', request_id: requestID('invalid-payer') }, 403)
  await api<unknown>('worker', '/orders/' + fixture.id + '/actions', 'POST', { action: 'deliver', note: '调价未完成不能直接提交交付' }, 409)
  record('加价只能本单打手提出，双方外管理员不得代同意，打手不得代付款，资金在途不能交付')
  const agreed = await api<OrderAdjustment>('player', '/orders/' + fixture.id + '/adjustments/' + increase.id + '/respond', 'POST', { action: 'accept' })
  assert.equal(agreed.status, 'awaiting_payment'); assert.ok(agreed.buyer_agreed && agreed.provider_agreed)
  const supplemental = await api<AdjustmentPayment>('player', '/orders/' + fixture.id + '/adjustments/' + increase.id + '/pay', 'POST', { channel: 'balance', request_id: requestID('supplement') })
  assert.equal(supplemental.mode, 'balance'); assert.equal(supplemental.adjustment.status, 'completed')
  await api<AdjustmentPayment>('player', '/orders/' + fixture.id + '/adjustments/' + increase.id + '/pay', 'POST', { channel: 'balance', request_id: requestID('supplement') })
  fixture = await currentOrder(); assert.equal(fixture.original_amount_cents, amounts.original); assert.equal(fixture.paid_total_cents, amounts.original + amounts.increase); assert.equal(fixture.amount_cents, amounts.original + amounts.increase)
  checkSettlement(fixture, originalSettlement); await checkBalances(fixture.amount_cents)
  record('买家确认后余额补款1.01元，原支付金额保留，重复请求不重复扣款，原费率分账守恒')
  for (const [role, other, amount, suffix] of [['player', 'worker', amounts.buyer_decrease, 'buyer-decrease'], ['worker', 'player', amounts.worker_decrease, 'worker-decrease']] as const) {
    const decrease = await api<OrderAdjustment>(role, '/orders/' + fixture.id + '/adjustments', 'POST', proposal('decrease', amount, suffix))
    await api<unknown>(role, '/orders/' + fixture.id + '/adjustments/' + decrease.id + '/respond', 'POST', { action: 'accept' }, 403)
    const completed: OrderAdjustment = await api<OrderAdjustment>(other, '/orders/' + fixture.id + '/adjustments/' + decrease.id + '/respond', 'POST', { action: 'accept' })
    assert.equal(completed.status, 'completed'); assert.ok(completed.buyer_agreed && completed.provider_agreed)
    fixture = await currentOrder(); checkSettlement(fixture, originalSettlement); await checkBalances(fixture.amount_cents)
    record((role === 'player' ? '买家' : '打手') + '提出减价，另一方同意后按原充值分区退款，不增加可提现余额')
  }
  assert.equal(fixture.amount_cents, amounts.net); assert.equal(fixture.price_refunded_cents, amounts.buyer_decrease + amounts.worker_decrease); assert.ok(!fixture.active_adjustment_id)
  const history = await api<OrderAdjustment[]>('player', '/orders/' + fixture.id + '/adjustments'); assert.equal(history.length, 3); assert.ok(history.every(item => item.status === 'completed'))
  await checkParticipants(); record('三次协商均保存证据，净价1.30元及三方净分账正确，原订单仍正常履约')
  report.passed = true
  if (keepRequested) {
    report.keep_active = true; report.cleanup_steps = ['仅供root使用订单ID查看本地SDK界面，不代表真实微信交易。', '买家对该order_id执行POST /orders/:id/actions {action:request_refund,note:本地验收完成全退}。', '打手对同一order_id执行POST /orders/:id/actions {action:approve_refund,note:同意退还全部剩余模拟款}。', '核对订单refunded、买家原充值分区恢复、打手及俱乐部待结算反冲。']; saveReport()
  }
}
/** 通过正常协商和售后接口结束唯一夹具，绝不删除订单证据或更改其他服务。 */
async function cleanup(): Promise<void> {
  if (!fixture) return
  let order = await currentOrder()
  if (order.active_adjustment_id) {
    const list = await api<OrderAdjustment[]>('player', '/orders/' + order.id + '/adjustments'), active = list.find(item => item.id === order.active_adjustment_id)
    assert.ok(active, '活动调价记录缺失，禁止跳过资金状态强行清理')
    if (['proposed', 'awaiting_payment'].includes(active.status)) await api<OrderAdjustment>(active.proposer_id === 'player' ? 'player' : 'worker', '/orders/' + order.id + '/adjustments/' + active.id + '/respond', 'POST', { action: 'withdraw' })
    else await api<OrderAdjustment>('player', '/orders/' + order.id + '/adjustments/' + active.id + '/sync', 'POST', {})
    order = await currentOrder(); assert.ok(!order.active_adjustment_id, '原调价资金仍在处理，保留订单ID供原单恢复')
  }
  if (order.refund_status === 'processing') { await api<Order>('player', '/orders/' + order.id + '/sync-payment', 'POST', {}); order = await currentOrder() }
  if (!['cancelled', 'refunded'].includes(order.status)) {
    if (['pending_payment', 'open'].includes(order.status)) await action('player', 'cancel', prefix + ' 验收结束')
    else { if (order.refund_status !== 'requested') await action('player', 'request_refund', prefix + ' 本地验收完成，退还全部剩余模拟款'); await action('worker', 'approve_refund', prefix + ' 同意按各原支付退回剩余款') }
  }
  order = await currentOrder(); assert.ok(['cancelled', 'refunded'].includes(order.status), '夹具必须通过正常流程关闭')
  if (beforeBuyer && beforeWorker && beforeClub) {
    const buyer = await api<Wallet>('player', '/wallet'), worker = await api<Wallet>('worker', '/wallet'), club = await api<Wallet>('owner', '/clubs/' + clubID + '/wallet')
    assert.equal(buyer.recharge_cents, beforeBuyer.recharge_cents); assert.equal(buyer.available_cents, beforeBuyer.available_cents)
    assert.equal(worker.pending_cents, beforeWorker.pending_cents); assert.equal(club.pending_cents, beforeClub.pending_cents)
  }
  report.cleanup_complete = true; report.cleanup_required = false; report.order_status = order.status; record('正常售后全退完成，原充值/收入分区恢复，打手及组织待结算归零增量')
}
/** 总入口保证失败也尝试正常清理，显式保持只对完全通过的夹具生效。 */
async function main(): Promise<void> {
  try { await run() } catch (error) { report.passed = false; report.error = error instanceof Error ? error.message : '本地验收发生未知错误' }
  finally {
    if (!(report.passed && report.keep_active)) { try { await cleanup() } catch (error) { report.passed = false; report.cleanup_required = true; report.error = (report.error ? report.error + '；' : '') + '正常清理未完成：' + (error instanceof Error ? error.message : '未知错误') } }
    for (const role of ['player', 'worker', 'admin', 'owner'] as const) if (tokens[role]) { try { await api<unknown>(role, '/auth/logout', 'POST', {}) } catch { /* 会话只在内存，退出失败也不写入报告。 */ } }
    saveReport(); console.log('本地模拟验收报告：' + path.join(directory, 'report.json')); if (!report.passed) { console.error(report.error || '验收未通过'); process.exitCode = 1 }
  }
}
void main()
