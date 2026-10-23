/** 文件作用：提供平台管理员图文审核队列、完整详情、人工决定、AI重试与鉴权图片接口。 */
import { request, queryString, download } from './client'
import type { ModerationInbox, ModerationQuery, ModerationTask } from '../types/moderation'

/** 按状态读取审核队列和真实汇总，最终管理权限由服务端检查。 */
export function listModerationTasks(query: ModerationQuery = {}): Promise<ModerationInbox> { return request('/admin/moderation' + queryString(query)) }
/** 仅管理员可读取完整原文，页面必须按纯文本展示。 */
export function getModerationTask(id: string): Promise<ModerationTask> { return request('/admin/moderation/' + encodeURIComponent(id)) }
/** 提交针对当前任务的明确人工结论，拒绝原因由客户端与服务端分别校验。 */
export function decideModerationTask(id: string, decision: 'allow' | 'reject', reason: string): Promise<ModerationTask> { return request('/admin/moderation/' + encodeURIComponent(id) + '/decision', 'POST', { decision, reason }) }
/** 仅待人工任务可重试AI，无配置时任务继续留在人工队列。 */
export function retryModerationTask(id: string): Promise<ModerationTask> { return request('/admin/moderation/' + encodeURIComponent(id) + '/retry', 'POST', {}) }
/** 固定同源鉴权下载路径，不信任上游任意图片地址，也不把令牌放进URL。 */
export function getModerationImage(id: string): Promise<string> { return download('/admin/moderation/' + encodeURIComponent(id) + '/image') }
