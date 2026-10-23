/** 文件作用：统一订单费用协商的状态文案、精确分额校验与前端可见权限；最终资格仍由服务端判断。 */
import type { Order } from '../types/models'
import type { AdjustmentKind,AdjustmentStatus,OrderAdjustment } from '../types/adjustments'
/** 展示真实资金中间态，不把受理当成支付或退款成功。 */
export function adjustmentStatus(status:AdjustmentStatus):string{return({proposed:'等待对方确认',awaiting_payment:'等待买家补款',paying:'补款核验中',refunding:'差额退还中',completed:'已完成',rejected:'已拒绝',withdrawn:'已撤回',expired:'已过期',refunded:'迟到补款已退还'})[status]||'状态待核对'}
/** 正差额以整数分校验，支持最低1分且拒绝科学计数和隐式四舍五入。 */
export function parseAdjustmentAmount(text:string):number{const input=text.trim();if(!/^\d+(\.\d{1,2})?$/.test(input))throw new Error('请填写大于0的差额，最多两位小数');const[whole,fraction='']=input.split('.'),cents=Number(whole)*100+Number(fraction.padEnd(2,'0'));if(!Number.isSafeInteger(cents)||cents<1||cents>1000000)throw new Error('差额应为0.01至10000元');return cents}
/** 判断尚未结束的协商与在途资金，过期是否关闭仍需服务端同步确认。 */
export function adjustmentPending(record:OrderAdjustment):boolean{return['proposed','awaiting_payment','paying','refunding'].includes(record.status)}
/** 付款/退款在途不能发起第二套资金操作，其余方案可由参与者拒绝或撤回。 */
export function adjustmentFundsPending(record:OrderAdjustment):boolean{return record.status==='paying'||record.status==='refunding'}
/** 根据服务端订单和当前身份决定是否展示新方案入口，不授予管理员代替双方同意的能力。 */
export function canProposeAdjustment(order:Order|null,userID:string,kind:AdjustmentKind,currentTime=Date.now()):boolean{
  if(!order||!userID||order.is_demo||order.payment_status!=='paid'||!order.provider_id||order.refund_final||!['','none'].includes(order.refund_status||'')||order.active_adjustment_id)return false
  const participant=userID===order.provider_id||userID===order.buyer_id
  if(!participant||(kind==='increase'&&userID!==order.provider_id))return false
  if(['open','accepted','in_progress','delivered'].includes(order.status))return true
  const completed=Date.parse(order.completed_at)
  return kind==='decrease'&&order.status==='completed'&&Number.isFinite(completed)&&completed<=currentTime&&currentTime<completed+7*86400000
}
/** 老订单缺少新汇总时保留当前金额作原价，不凭空推算不存在的补款。 */
export function adjustmentTotals(order:Order):{original:number;paid:number;refunded:number;extra:number;net:number}{const valid=(v:unknown,fallback:number)=>Number.isSafeInteger(v)&&Number(v)>=0?Number(v):fallback,original=valid(order.original_amount_cents,order.amount_cents),refunded=valid(order.price_refunded_cents,0),paid=valid(order.paid_total_cents,order.payment_status==='paid'?order.amount_cents:0);return{original,paid,refunded,extra:Math.max(0,paid-original),net:order.amount_cents}}
