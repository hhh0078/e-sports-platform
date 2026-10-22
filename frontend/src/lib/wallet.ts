/** 文件作用：统一钱包金额输入、状态文案与可恢复的资金请求号，不在客户端推导资金到账。 */
/** 金额输入转整数分，拒绝指数、负数和超过两位小数。 */
export function parseWalletAmount(value:string,maxCents=10000000,minCents=100):number{
  if(!/^\d+(\.\d{1,2})?$/.test(value.trim()))throw new Error('金额最多填写两位小数')
  const cents=Math.round(Number(value)*100)
  if(!Number.isSafeInteger(cents)||cents<minCents||cents>maxCents)throw new Error('金额须为'+minCents/100+'至'+maxCents/100+'元')
  return cents
}
/** 创建幂等请求号；它只是业务去重标识，不是安全凭据。 */
export function newWalletRequestID():string{return 'w'+Date.now().toString(36)+Math.random().toString(36).slice(2,12)}
/** 显示服务端明确的状态，未知状态不猜成功。 */
export function walletStatus(value:string):string{return({pending_payment:'待支付',paid:'已支付',refunded:'已退还',pending_review:'待平台审核',processing:'处理中',wait_user_confirm:'待确认收款',success:'已完成',failed:'未成功',rejected:'已拒绝',queued:'等待退款'} as Record<string,string>)[value]||value}
/** 将记账类型转换为可读业务名称。 */
export function walletKind(value:string):string{return({topup:'余额充值',recharge:'余额充值',deposit:'缴纳平台押金',deposit_topup:'缴纳平台押金',order_payment:'余额支付',order_pending:'服务收入待结算',order_settlement:'服务收入结算',order_refund:'订单退款',order_refund_reversal:'退回待结算收入',order_balance_refund:'恢复支付余额',withdrawal_freeze:'申请提现',withdrawal_finish:'提现处理完成',withdrawal_reject:'提现退回',withdrawal_success:'提现到账',withdrawal_failed:'提现退回',withdrawal_rejected:'提现退回',deposit_refund:'退还平台押金',deposit_refund_success:'押金原路退还',deposit_refund_freeze:'申请退还押金'} as Record<string,string>)[value]||value}
