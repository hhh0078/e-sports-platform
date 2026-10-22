/** 文件作用：统一资金页面费率输入、押金范围文案与原请求缓存定位，不在客户端生成余额。 */
import type { DepositScope, WalletTopupPurpose } from '../types/wallet'
/** 百分比输入转换为整数基点，拒绝指数、负数和超过两位小数。 */
export function parseRate(value: string): number {
  const raw = value.trim()
  if (!/^\d{1,3}(\.\d{1,2})?$/.test(raw)) throw new Error('抽成比例请填写0—100，最多两位小数')
  const [whole, fraction = ''] = raw.split('.')
  const result = Number(whole) * 100 + Number(fraction.padEnd(2, '0'))
  if (result > 10000) throw new Error('抽成比例不能超过100%')
  return result
}
/** 将服务端整数基点显示为紧凑百分比。 */
export function formatRate(bps?: number): string { return ((bps || 0) / 100).toFixed(2).replace(/\.?0+$/, '') + '%' }
/** 按资金归属生成唯一意图键，防止不同俱乐部或账号复用同一请求。 */
export function fundsIntentKey(userID: string, clubID: string, operation: string, cents: number): string { return ['jijie_funds_intent', userID, clubID, operation, cents].map(encodeURIComponent).join(':') }
/** 三类押金及消费充值使用明确名称，不能误标组织付款。 */
export function paymentPurposeName(purpose: WalletTopupPurpose): string { return ({ balance: '余额充值', deposit: '个人平台押金', club_platform: '俱乐部平台押金', member_club: '成员俱乐部押金' })[purpose] }
/** 押金退款范围转换为对应付款用途。 */
export function depositPurpose(scope: DepositScope): Exclude<WalletTopupPurpose, 'balance'> { return scope === 'personal' ? 'deposit' : scope }
