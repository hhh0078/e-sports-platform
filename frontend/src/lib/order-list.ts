/** 文件作用：统一订单深链、列表筛选和买家与打手操作提示。 */
import type { Order } from '../types/models'
/** 订单允许的筛选值。 */
export const orderFilters = [{ id: 'all', label: '全部' }, { id: 'pending_payment', label: '待支付' }, { id: 'working', label: '进行中' }, { id: 'delivered', label: '待验收' }, { id: 'completed', label: '已完成' }, { id: 'refund', label: '售后' }]
/** 订单列表身份。 */
export type OrderRole = 'buyer' | 'provider' | 'all'
/** 解析并限制可公开导航的身份与筛选。 */
export function parseOrderQuery(query: Record<string, unknown> = {}): { role: OrderRole; filter: string } { return { role: ['buyer', 'provider', 'all'].includes(String(query.role)) ? query.role as OrderRole : 'buyer', filter: orderFilters.some(item => item.id === query.status) ? String(query.status) : 'all' } }
/** 构造带筛选的主页面订单入口。 */
export function orderListPath(role: string, filter: string): string { const normalized = parseOrderQuery({ role, status: filter }); return '/pages/main/index?tab=orders&role=' + normalized.role + '&status=' + normalized.filter }
/** 根据本人买家身份和售后状态展示卡片操作。 */
export function orderActionLabel(order: Pick<Order, 'status' | 'buyer_id' | 'refund_status'>, userID: string): string { if (order.status === 'pending_payment') return order.buyer_id === userID ? '继续支付' : '等待买家支付'; if (['requested', 'rejected', 'escalated', 'processing'].includes(order.refund_status)) return '查看售后'; if (['open', 'accepted', 'in_progress', 'delivered', 'disputed'].includes(order.status)) return '订单沟通'; return '查看订单' }
