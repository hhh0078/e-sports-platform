/** 文件作用：统一金额、时间、状态显示和共享导航入口。 */
import { API_BASE, errorMessage, errorCode } from '../api/client'
import { navigate } from './navigation'
const statusNames: Record<string, string> = { pending_deposit:'待缴平台押金', pending: '审核中', active: '已上架', rejected: '未通过', paused: '已暂停', pending_payment: '待支付', open: '等待接单', accepted: '已接单', in_progress: '服务中', delivered: '待验收', completed: '已完成', cancelled: '已取消', disputed: '平台处理中', refunded: '已退款', processing: '退款处理中', requested: '退款协商中', escalated: '平台介入中', approved: '退款通过', none: '无售后', unpaid: '未支付', paid: '已支付', frozen: '已冻结' }
/** 将整数分展示为最多两位小数金额。 */
export function money(cents: number | string | null | undefined): string { return (Number(cents || 0) / 100).toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1') }
/** 将有效服务端日期展示为月日与时分。 */
export function time(value: string | number | null | undefined): string { if (!value) return ''; const date = new Date(value); if (Number.isNaN(date.getTime())) return ''; return `${date.getMonth() + 1}/${date.getDate()} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}` }
/** 显示订单与审核状态，未知值保持可辨识。 */
export function statusLabel(status: string | null | undefined): string { return status ? statusNames[status] || status : '未知状态' }
/** 委托统一导航解析入口和二级返回路径。 */
export function go(url: string): void { navigate(url) }
/** 审核结果保留完整原因供修改，普通操作沿用短提示。 */
export function notify(error: unknown): void {
  const code = errorCode(error)
  if (code === 'moderation_rejected' || code === 'moderation_unavailable') {
    uni.showModal({ title: code === 'moderation_rejected' ? '内容需修改' : '审核暂不可用', content: errorMessage(error), showCancel: false, confirmText: '我知道了' })
    return
  }
  uni.showToast({ title: errorMessage(error), icon: 'none', duration: 2500 })
}
/** 将受后端托管的相对资源解析到当前服务端。 */
export function assetUrl(url: string | null | undefined): string { const value = String(url || ''); return value.startsWith('/api/') || (value.startsWith('/static/covers/') || value.startsWith('/static/avatars/')) ? API_BASE.replace(/\/api\/?$/, '') + value : value }
