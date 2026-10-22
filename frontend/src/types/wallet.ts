/** 文件作用：声明按资金来源隔离的钱包、微信充值、提现审核和押金原路退还契约。 */
import type { PaymentParams } from './models'
/** 钱包不同用途的余额均使用整数分。 */
export interface Wallet { available_cents:number; recharge_cents:number; spendable_cents:number; pending_cents:number; withdrawal_frozen_cents:number; deposit_cents:number; deposit_refunding_cents:number; currency:'CNY'; settlement_hold_days:number; dev_mode:boolean }
/** 不可变资金变动，金额正负由对应资金分区变动决定。 */
export interface WalletEntry { id:string; kind:string; reference_id:string; amount_cents:number; note:string; created_at:string; recharge_delta:number; available_delta:number; pending_delta:number; withdrawal_frozen_delta:number; deposit_delta:number; deposit_refunding_delta:number }
/** 微信充值或自愿押金付款。 */
export interface WalletTopup { id:string; request_id?:string; purpose:'balance'|'deposit'; amount_cents:number; status:'pending_payment'|'paid'|'refunded'; refunded_cents:number; created_at:string }
/** 服务端支付准备结果，不能从原生支付回调直接推断到账。 */
export interface WalletTopupPayment { payment:WalletTopup; mode:'dev'|'wechat'; pay_params?:PaymentParams }
/** 微信待收款确认凭据，仅当前用户短暂读取。 */
export interface TransferConfirmation { mch_id:string; app_id:string; package_info:string }
/** 提现申请的审核、冻结和微信实际终态。 */
export interface WalletWithdrawal { id:string; request_id?:string; user_id:string; amount_cents:number; status:'pending_review'|'processing'|'wait_user_confirm'|'success'|'failed'|'rejected'; failure_reason?:string; created_at:string; updated_at:string; confirmation?:TransferConfirmation }
/** 每笔押金按原付款渠道分别退还。 */
export interface WalletDepositRefund { id:string; request_id?:string; amount_cents:number; status:'processing'|'success'; created_at:string; parts:Array<{topup_id:string;amount_cents:number;status:'queued'|'processing'|'success'}> }
/** 管理员填写获批商家转账场景，秘密沿用支付配置。 */
export interface WalletSettings { withdrawal_enabled:boolean; transfer_scene_id:string; transfer_scene_report_infos:Array<{info_type:string;info_content:string}>; transfer_notify_url:string; transfer_remark:string; user_recv_perception:string }
/** 保存配置不等同于商户场景通过微信审批。 */
export interface WalletSettingsResult { settings:WalletSettings; transfer_ready:boolean; configuration_notice:string }
