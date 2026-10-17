/** 文件作用：通用金额、时间、状态显示和页面导航。 */
import { API_BASE } from './api.js'
const statusNames = { pending: '审核中', active: '已上架', rejected: '未通过', paused: '已暂停', pending_payment: '待支付', open: '等待接单', accepted: '已接单', in_progress: '服务中', delivered: '待验收', completed: '已完成', cancelled: '已取消', disputed: '平台处理中', refunded: '已退款', processing: '退款处理中', requested: '退款协商中', escalated: '平台介入中', approved: '退款通过', none: '无售后', unpaid: '未支付', paid: '已支付', frozen: '已冻结' }
/** 将整数分转为最多两位小数的金额。 */
export function money(cents) { return (Number(cents || 0) / 100).toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1') }
/** 格式化服务端时间，不受机器地区显示差异影响。 */
export function time(value) { if (!value) return ''; const date = new Date(value); if (Number.isNaN(date.getTime())) return ''; return `${date.getMonth() + 1}/${date.getDate()} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}` }
/** 取得订单或审核状态的中文含义。 */
export function statusLabel(status) { return statusNames[status] || status || '未知状态' }
/** 切换主导航时重置页面栈，其余页面保留返回路径。 */
export function go(url) { const primary = ['/pages/index/index', '/pages/worker/index', '/pages/orders/index', '/pages/me/index']; if (primary.includes(url)) uni.reLaunch({ url }); else uni.navigateTo({ url }) }
/** 以统一短提示显示操作反馈。 */
export function notify(error) { uni.showToast({ title: typeof error === 'string' ? error : error?.message || '操作失败，请重试', icon: 'none', duration: 2500 }) }

/** 将后台上传资源的相对地址转成当前服务端地址，内置静态封面保留本地路径。 */
export function assetUrl(url) { const value = String(url || ''); return value.startsWith('/api/') ? API_BASE.replace(/\/api\/?$/, '') + value : value }
