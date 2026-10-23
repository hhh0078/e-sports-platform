/** 文件作用：集中封装订单协商调价、双方确认及补款查询，页面不直接拼接网络请求。 */
import { request } from './client'
import type { OrderAdjustment,CreateAdjustmentInput,AdjustmentPayment } from '../types/adjustments'
/** 为参与者读取本单完整费用协商历史。 */
export function listOrderAdjustments(orderID:string):Promise<OrderAdjustment[]>{return request('/orders/'+encodeURIComponent(orderID)+'/adjustments')}
/** 创建独立增减价方案，不立即扣款或退款。 */
export function createOrderAdjustment(orderID:string,body:CreateAdjustmentInput):Promise<OrderAdjustment>{return request('/orders/'+encodeURIComponent(orderID)+'/adjustments','POST',body)}
/** 提交本人接受、拒绝或撤回，权限及状态由服务端复核。 */
export function respondOrderAdjustment(orderID:string,id:string,action:'accept'|'reject'|'withdraw'):Promise<OrderAdjustment>{return request('/orders/'+encodeURIComponent(orderID)+'/adjustments/'+encodeURIComponent(id)+'/respond','POST',{action})}
/** 为已接受加价取得原补款单的付款参数或核验结果。 */
export function payOrderAdjustment(orderID:string,id:string,body:{channel:'wechat'|'balance';request_id?:string}):Promise<AdjustmentPayment>{return request('/orders/'+encodeURIComponent(orderID)+'/adjustments/'+encodeURIComponent(id)+'/pay','POST',body)}
/** 只查询并核验原补款或退款单，不能以刷新换单重发资金。 */
export function syncOrderAdjustment(orderID:string,id:string):Promise<OrderAdjustment>{return request('/orders/'+encodeURIComponent(orderID)+'/adjustments/'+encodeURIComponent(id)+'/sync','POST',{})}
