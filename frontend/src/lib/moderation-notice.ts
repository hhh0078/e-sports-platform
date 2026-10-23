/** 文件作用：将后置审核回执转换为非阻塞轻提示，延迟到业务成功提示之后并丢弃旧账号响应。 */

/** 后端已持久化任务的安全回执。 */
export interface ModerationReceipt { status: 'ai_pending' | 'manual_pending'; estimated_seconds: number; pending_count: number }
let noticeTimer: ReturnType<typeof setTimeout> | undefined

/**
 * 只接受后端明确的待审核状态，不把审核故障或任意文案解释成通过。
 * @param value - 响应中的审核元数据。
 * @returns 有效回执，无待审任务时返回null。
 */
export function moderationReceipt(value: unknown): ModerationReceipt | null {
  if (!value || typeof value !== 'object') return null
  const item = value as Record<string, unknown>
  if (item.status !== 'ai_pending' && item.status !== 'manual_pending') return null
  if (!Number.isSafeInteger(item.pending_count) || Number(item.pending_count) <= 0) return null
  return { status: item.status, estimated_seconds: 20, pending_count: Number(item.pending_count) }
}

/**
 * 保存成功后展示可继续操作的轻提示；人工待审不承诺20秒完成。
 * @param value - 后端回执。
 * @param current - 调度及展示时核对原请求账号。
 * @param uploaded - 是否为独立图片上传。
 */
export function queueModerationNotice(value: unknown, current: () => boolean, uploaded = false): void {
  const receipt = moderationReceipt(value)
  if (!receipt || !current()) return
  if (noticeTimer !== undefined) clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    noticeTimer = undefined
    if (!current()) return
    const prefix = uploaded ? '图片已上传' : '已保存'
    const title = receipt.status === 'manual_pending' ? prefix + '，等待人工审核' : prefix + '，图文审核中，预计20秒左右'
    // 提示能力异常不能把已提交成功的业务误报成保存失败。
    try { uni.showToast({ title, icon: 'none', duration: 3000 }) } catch { /* 业务已保存，审核结果仍会通过消息通知。 */ }
  }, 0)
}
