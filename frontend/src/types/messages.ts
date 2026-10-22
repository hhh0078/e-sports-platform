/** 文件作用：定义站内系统消息、按账号本机缓存、已读删除确认及管理员发送的数据契约。 */

/** 服务端待接收消息；正文始终为纯文本。 */
export interface SystemMessage { id: string; title: string; body: string; created_at: string; user_id?: string }
/** 本机持久消息；只有已读落盘后才允许确认删除远端。 */
export interface CachedMessage extends SystemMessage { read: boolean; ack_pending: boolean }
/** 服务端待接收列表，支持在删除前完整分页获取。 */
export interface MessageInbox { items: SystemMessage[]; total: number }
/** 本机缓存版本与所属用户，禁止使用当前账号读取其他用户的数据。 */
export interface MessageCacheDocument { version: 1; user_id: string; items: CachedMessage[] }
/** 消息收取分页参数。 */
export interface MessageListQuery { limit?: number; offset?: number }
/** 管理员发送纯文本消息，收件人为空表示广播。 */
export interface AdminMessageInput { recipient_id?: string; title: string; body: string }
/** 发送结果仅返回实际收件人数。 */
export interface AdminMessageResult { recipient_count: number }
