/** 文件作用：封装订单履约、支付、消息、评价、证据与匿名接单大厅接口。 */
import { request, upload, queryString } from './client'
import type { ListQuery, Order, Message, Review, UploadedImage, PublicJob, PaymentResult } from '../types/models'
/** 订单列表筛选参数。 */
export interface OrderQuery extends ListQuery { role?: string }
/** 聊天增量与历史游标。 */
export interface MessageQuery { limit?: number; after_id?: string; before_id?: string }
/** 下单载荷。 */
export interface CreateOrderInput { service_id: string; quantity: number; requirements: string }
/** 公开需求发布载荷。 */
export interface RequestInput { game_id: string; title: string; requirements: string; amount_cents: number }
/** 读取本人买家或接单订单。 */
export function listOrders(query: OrderQuery = {}): Promise<Order[]> { return request('/orders' + queryString(query)) }
/** 读取订单与权限操作信息。 */
export function getOrder(id: string): Promise<Order> { return request('/orders/' + encodeURIComponent(id)) }
/** 按服务创建未支付订单。 */
export function createOrder(body: CreateOrderInput): Promise<Order> { return request('/orders', 'POST', body) }
/** 发布公开接单需求。 */
export function createRequest(body: RequestInput): Promise<Order> { return request('/requests', 'POST', body) }
/** 读取匿名接单大厅。 */
export function listJobs(query: ListQuery = {}): Promise<PublicJob[]> { return request('/jobs' + queryString(query)) }
/** 提交订单动作及原因说明。 */
export function performOrderAction(id: string, action: string, note = '', clubId?: string): Promise<Order> { return request('/orders/' + encodeURIComponent(id) + '/actions', 'POST', { action, note, ...(clubId ? { club_id: clubId } : {}) }) }
/** 选择俱乐部后认领公开需求，认证范围由服务端复核。 */
export function claimJob(id: string, clubId = ''): Promise<Order> { return performOrderAction(id, 'claim', '', clubId) }
/** 取得真实微信支付参数或本地演示结果。 */
export function payOrder(id: string, body: {channel?:'balance'|'wechat';request_id?:string;platform?:'android'|'ios';quoted_total_cents?:number;quoted_topup_cents?:number} = {}): Promise<PaymentResult> { return request('/orders/' + encodeURIComponent(id) + '/pay', 'POST', body) }
/** 向微信同步当前订单支付状态。 */
export function syncOrderPayment(id: string): Promise<Order> { return request('/orders/' + encodeURIComponent(id) + '/sync-payment', 'POST', {}) }
/** 获取聊天消息，权限由服务端校验。 */
export function listMessages(id: string, query: MessageQuery = {}): Promise<Message[]> { return request('/orders/' + encodeURIComponent(id) + '/messages' + queryString(query)) }
/** 发送文字或安全图片消息。 */
export function sendOrderMessage(id: string, body: { content: string; image_url?: string }): Promise<Message> { return request('/orders/' + encodeURIComponent(id) + '/messages', 'POST', body) }
/** 读取订单评价。 */
export function listOrderReviews(id: string): Promise<Review[]> { return request('/orders/' + encodeURIComponent(id) + '/reviews') }
/** 提交订单评价。 */
export function submitOrderReview(id: string, body: { rating: number; content: string }): Promise<Review> { return request('/orders/' + encodeURIComponent(id) + '/review', 'POST', body) }
/** 上传本订单证据图片。 */
export function uploadEvidence(id: string, filePath: string): Promise<UploadedImage> { return upload('/orders/' + encodeURIComponent(id) + '/evidence', filePath) }
/** 更新受保护证据图片的临时链接。 */
export function refreshEvidenceLink(id: string, evidenceID: string): Promise<UploadedImage> { return request('/orders/' + encodeURIComponent(id) + '/evidence-link' + queryString({ evidence_id: evidenceID })) }
