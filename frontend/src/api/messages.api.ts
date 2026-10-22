/** 文件作用：封装站内消息分页接收、本人已读删除确认及平台管理员发送的具名接口。 */
import { request, queryString } from './client'
import type { MessageInbox, MessageListQuery, AdminMessageInput, AdminMessageResult } from '../types/messages'

/** 分页收取当前用户消息，调用本方法不会修改未读状态或删除服务器数据。 */
export function listSystemMessages(query: MessageListQuery = {}): Promise<MessageInbox> { return request('/messages' + queryString(query)) }
/** 已读及全文持久成功后确认移除本人远端消息；服务器支持重复确认。 */
export function acknowledgeMessage(id: string): Promise<{ ok: boolean }> { return request('/messages/' + encodeURIComponent(id), 'DELETE') }
/** 平台管理员向指定账号或所有有效账号发送消息，最终权限与长度由后端核验。 */
export function sendAdminMessage(body: AdminMessageInput): Promise<AdminMessageResult> { return request('/admin/messages', 'POST', body) }
