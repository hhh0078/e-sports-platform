/** 文件作用：定义订单费用协商、补款与原路退差的公开类型，不包含微信私密付款凭证。 */
import type { Order, PaymentParams } from './models'
/** 调价意图以正整数分表示增加或减少的差额。 */
export type AdjustmentKind = 'increase' | 'decrease'
/** 付款和退款的未知中间状态不等同资金成功。 */
export type AdjustmentStatus = 'proposed' | 'awaiting_payment' | 'paying' | 'refunding' | 'completed' | 'rejected' | 'withdrawn' | 'expired' | 'refunded'
/** 参与者可审阅的不可抹除协商记录。 */
export interface OrderAdjustment { id:string; order_id:string; kind:AdjustmentKind; amount_cents:number; reason:string; proposer_id:string; proposer_name:string; status:AdjustmentStatus; buyer_agreed:boolean; provider_agreed:boolean; expires_at:string; created_at:string; updated_at:string; completed_at?:string; failure_reason?:string; request_id?:string }
/** 创建方案所需的稳定业务幂等标识。 */
export interface CreateAdjustmentInput { kind:AdjustmentKind; amount_cents:number; reason:string; request_id:string }
/** 补款返回临时支付参数，仅本次原账号使用，不写本地持久存储。 */
export interface AdjustmentPayment { adjustment:OrderAdjustment; order:Order; mode:'wechat'|'balance'|'dev'; pay_params?:PaymentParams }
