/** 文件作用：封装钱包账目、充值、提现、押金和管理员审核配置接口。 */
import { request, queryString } from './client'
import type { ListQuery } from '../types/models'
import type { Wallet,WalletEntry,WalletTopup,WalletTopupPayment,WalletWithdrawal,WalletDepositRefund,WalletSettings,WalletSettingsResult,WalletTopupPurpose,DepositScope } from '../types/wallet'
/** 本人资金历史筛选，组织维度不改变服务端的本人归属校验。 */
export interface WalletHistoryQuery extends ListQuery { club_id?: string; purpose?: WalletTopupPurpose; scope?: DepositScope }
/** 读取真实资金分区余额。 */
export function getWallet():Promise<Wallet>{return request('/wallet')}
/** 分页读取不可变流水。 */
export function listWalletEntries(query:{limit?:number;offset?:number}={}):Promise<WalletEntry[]>{return request('/wallet/entries'+queryString(query))}
/** 使用稳定请求号建立充值或押金支付单。 */
export function createWalletTopup(body:{purpose:WalletTopupPurpose;club_id?:string;amount_cents:number;request_id:string}):Promise<WalletTopupPayment>{return request('/wallet/topups','POST',body)}
/** 查看最近付款单。 */
export function listWalletTopups(query:WalletHistoryQuery={}):Promise<WalletTopup[]>{return request('/wallet/topups'+queryString(query))}
/** 跨设备恢复本人未付款原单，沿用服务端已绑定单号。 */
export function payWalletTopup(id:string):Promise<WalletTopupPayment>{return request('/wallet/topups/'+encodeURIComponent(id)+'/pay','POST',{})}
/** 按原付款单查询微信实际支付结果。 */
export function syncWalletTopup(id:string):Promise<WalletTopup>{return request('/wallet/topups/'+encodeURIComponent(id)+'/sync','POST',{})}
/** 冻结可提现收入并提交提现审核，实名不持久化。 */
export function createWalletWithdrawal(body:{amount_cents:number;request_id:string;user_name?:string}):Promise<WalletWithdrawal>{return request('/wallet/withdrawals','POST',body)}
/** 查看本人提现与收款确认状态。 */
export function listWalletWithdrawals(query:ListQuery={}):Promise<WalletWithdrawal[]>{return request('/wallet/withdrawals'+queryString(query))}
/** 查询原微信转账单，未知结果不解冻余额。 */
export function syncWalletWithdrawal(id:string):Promise<WalletWithdrawal>{return request('/wallet/withdrawals/'+encodeURIComponent(id)+'/sync','POST',{})}
/** 申请原路退还押金，资格由服务端事务校验。 */
export function createDepositRefund(body:{amount_cents?:number;request_id:string;scope?:DepositScope;club_id?:string}):Promise<WalletDepositRefund>{return request('/wallet/deposit-refunds','POST',body)}
/** 查看本人押金退款进度。 */
export function listDepositRefunds(query:WalletHistoryQuery={}):Promise<WalletDepositRefund[]>{return request('/wallet/deposit-refunds'+queryString(query))}
/** 同步各原支付单的退款结果。 */
export function syncDepositRefund(id:string):Promise<WalletDepositRefund>{return request('/wallet/deposit-refunds/'+encodeURIComponent(id)+'/sync','POST',{})}
/** 管理员分页查看提现申请。 */
export function listAdminWithdrawals(query:{status?:string;limit?:number;offset?:number}={}):Promise<WalletWithdrawal[]>{return request('/admin/wallet/withdrawals'+queryString(query))}
/** 管理员审核通过并沿原单号发起转账；开发模拟仅后端开发环境接受。 */
export function executeAdminWithdrawal(id:string,body:{dev_outcome?:'success'|'failed'}={}):Promise<WalletWithdrawal>{return request('/admin/wallet/withdrawals/'+encodeURIComponent(id)+'/execute','POST',body)}
/** 仅拒绝尚未发起的申请并解冻，已出网单不允许走此接口。 */
export function rejectAdminWithdrawal(id:string,reason:string):Promise<WalletWithdrawal>{return request('/admin/wallet/withdrawals/'+encodeURIComponent(id)+'/reject','POST',{reason})}
/** 读取管理员钱包公开配置。 */
export function getWalletSettings():Promise<WalletSettingsResult>{return request('/admin/wallet-settings')}
/** 保存已获批场景配置，不传商户密钥。 */
export function saveWalletSettings(body:WalletSettings):Promise<WalletSettingsResult>{return request('/admin/wallet-settings','PATCH',body)}
