/** 文件作用：统一订单页深链、状态筛选和操作文案，保持买家、打手及统计入口语义一致。 */
export const orderFilters = [{ id: 'all', label: '全部' }, { id: 'pending_payment', label: '待支付' }, { id: 'working', label: '进行中' }, { id: 'delivered', label: '待验收' }, { id: 'completed', label: '已完成' }, { id: 'refund', label: '售后' }]
/** 解析可公开导航的身份和状态，非法参数回退默认值，不将参数直接拼入请求。 */
export function parseOrderQuery(query = {}) {
  return { role: ['buyer', 'provider', 'all'].includes(query.role) ? query.role : 'buyer', filter: orderFilters.some(item => item.id === query.status) ? query.status : 'all' }
}
/** 构造登录返回路径，保证登录前所选身份及筛选条件不丢失。 */
export function orderListPath(role, filter) { const normalized = parseOrderQuery({ role, status: filter }); return '/pages/orders/index?role=' + normalized.role + '&status=' + normalized.filter }
/** 仅买家可继续支付，其它操作根据履约及售后状态展示。 */
export function orderActionLabel(order, userID) {
  if (order.status === 'pending_payment') return order.buyer_id === userID ? '继续支付' : '等待买家支付'
  if (['requested', 'rejected', 'escalated', 'processing'].includes(order.refund_status)) return '查看售后'
  if (['open', 'accepted', 'in_progress', 'delivered', 'disputed'].includes(order.status)) return '订单沟通'
  return '查看订单'
}
