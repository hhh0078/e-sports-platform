/** 文件作用：封装交易规则、组织钱包、三类押金及提现来源核验，页面不直接拼接资金请求。 */
import { request, queryString } from './client'
import type { FinancialSettings, ClubDeposits, ClubWallet, WalletWithdrawal, WalletEntry, WithdrawalAudit } from '../types/wallet'
/** 读取创建及下单前可公开告知的交易规则，不包含商户配置。 */
export function getFinancialRules(): Promise<FinancialSettings> { return request('/financial-rules') }
/** 平台管理员读取两个平台费率和俱乐部创建押金。 */
export function getFinancialSettings(): Promise<FinancialSettings> { return request('/admin/financial-settings') }
/** 平台管理员保存整数基点费率及整数分押金。 */
export function saveFinancialSettings(body: FinancialSettings): Promise<FinancialSettings> { return request('/admin/financial-settings', 'PATCH', body) }
/** 读取本人在指定俱乐部的押金以及组织平台押金状态。 */
export function getClubDeposits(id: string): Promise<ClubDeposits> { return request('/clubs/' + encodeURIComponent(id) + '/deposits') }
/** 负责人读取独立俱乐部佣金钱包。 */
export function getClubWallet(id: string): Promise<ClubWallet> { return request('/clubs/' + encodeURIComponent(id) + '/wallet') }
/** 分页读取俱乐部佣金不可变账目。 */
export function listClubWalletEntries(id: string, query: { limit?: number; offset?: number } = {}): Promise<WalletEntry[]> { return request('/clubs/' + encodeURIComponent(id) + '/entries' + queryString(query)) }
/** 读取俱乐部提现历史，普通成员不能读取。 */
export function listClubWithdrawals(id: string): Promise<WalletWithdrawal[]> { return request('/clubs/' + encodeURIComponent(id) + '/withdrawals') }
/** 冻结俱乐部佣金来源并申请向负责人实名微信转账。 */
export function createClubWithdrawal(id: string, body: { amount_cents: number; request_id: string; user_name?: string }): Promise<WalletWithdrawal> { return request('/clubs/' + encodeURIComponent(id) + '/withdrawals', 'POST', body) }
/** 平台管理员查看付款来源、订单及本次提现占用的逐笔核验结果。 */
export function getWithdrawalAudit(id: string): Promise<WithdrawalAudit> { return request('/admin/wallet/withdrawals/' + encodeURIComponent(id) + '/audit') }
