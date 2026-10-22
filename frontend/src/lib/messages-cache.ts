/** 文件作用：按账号保存系统消息全文与已读状态，核验持久化成功后才允许确认删除远端。 */
import type { SystemMessage, CachedMessage, MessageCacheDocument } from '../types/messages'

/** 仅抽象同步本机存储边界，生产使用官方uni存储，测试可模拟配额和读回故障。 */
export interface MessageStorage { read: (key: string) => unknown; write: (key: string, value: string) => void }
const deviceStorage: MessageStorage = { read: key => uni.getStorageSync(key), write: (key, value) => uni.setStorageSync(key, value) }

/** 账号ID编码进独立命名空间，空身份绝不读取缓存。 */
export function messageCacheKey(userID: string): string { if (!userID.trim()) throw new Error('请先登录后读取消息'); return 'jijie_messages_v1:' + encodeURIComponent(userID) }
/** 验证完整纯文本消息，异常远端或本机数据不得触发删除。 */
export function isSystemMessage(value: unknown): value is SystemMessage {
  if (typeof value !== 'object' || value === null) return false
  const item = value as Record<string, unknown>
  return typeof item.id === 'string' && !!item.id.trim() && typeof item.title === 'string' && typeof item.body === 'string' && typeof item.created_at === 'string' && Number.isFinite(Date.parse(item.created_at)) && (item.user_id === undefined || typeof item.user_id === 'string')
}
/** 复制声明字段，避免服务端或存储中的额外字段进入本机历史。 */
function messageFields(item: SystemMessage): SystemMessage { return { id: item.id, title: item.title, body: item.body, created_at: item.created_at, ...(item.user_id ? { user_id: item.user_id } : {}) } }
/** 校验账号归属和已读确认关系，禁止未读消息携带待删除标记。 */
function checkedItems(userID: string, value: unknown): CachedMessage[] {
  if (!Array.isArray(value)) throw new Error('本机消息缓存格式异常，未删除服务器消息')
  const known = new Set<string>()
  return value.map((item: unknown) => {
    if (!isSystemMessage(item) || typeof (item as CachedMessage).read !== 'boolean' || typeof (item as CachedMessage).ack_pending !== 'boolean' || ((item as CachedMessage).ack_pending && !(item as CachedMessage).read) || (item.user_id && item.user_id !== userID) || known.has(item.id)) throw new Error('本机消息缓存格式或账号归属异常，未删除服务器消息')
    known.add(item.id); return { ...messageFields(item), read: (item as CachedMessage).read, ack_pending: (item as CachedMessage).ack_pending }
  })
}
/** 读取账号自己的完整历史，损坏数据不静默覆盖为空列表。 */
export function readMessageCache(userID: string, storage: MessageStorage = deviceStorage): CachedMessage[] {
  const value = storage.read(messageCacheKey(userID))
  if (value === '' || value === undefined || value === null) return []
  let parsed: unknown; try { parsed = typeof value === 'string' ? JSON.parse(value) : value } catch { throw new Error('本机消息缓存无法读取，未删除服务器消息') }
  if (typeof parsed !== 'object' || parsed === null) throw new Error('本机消息缓存格式异常，未删除服务器消息')
  const document = parsed as Partial<MessageCacheDocument>
  if (document.version !== 1 || document.user_id !== userID) throw new Error('本机消息缓存版本或账号不匹配，未删除服务器消息')
  return checkedItems(userID, document.items)
}
/** 写入全文、已读和待确认状态并精确读回核验；失败时调用者不可发送删除。 */
export function writeMessageCache(userID: string, items: CachedMessage[], storage: MessageStorage = deviceStorage): void {
  const key = messageCacheKey(userID), document: MessageCacheDocument = { version: 1, user_id: userID, items: checkedItems(userID, items) }, serialized = JSON.stringify(document)
  try { storage.write(key, serialized); if (storage.read(key) !== serialized) throw new Error('消息存储读回核验失败') }
  catch { throw new Error('本机消息保存失败，请检查存储空间；未确认删除服务器消息') }
}
/** 合并服务端消息，已存在的全文和已读不可被重送旧条目覆盖，未读永不待删除。 */
export function mergeMessageCache(userID: string, local: CachedMessage[], incoming: SystemMessage[]): CachedMessage[] {
  const merged = new Map(checkedItems(userID, local).map(item => [item.id, item]))
  for (const item of incoming) {
    if (!isSystemMessage(item) || (item.user_id && item.user_id !== userID)) throw new Error('服务器消息格式或账号归属异常，未删除消息')
    const existing = merged.get(item.id)
    merged.set(item.id, existing ? { ...existing, ack_pending: existing.read } : { ...messageFields(item), read: false, ack_pending: false })
  }
  return [...merged.values()].sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at) || right.id.localeCompare(left.id))
}
/** 标记单条本机消息已读并待确认；缺失消息不能凭ID伪造删除资格。 */
export function markCachedMessageRead(items: CachedMessage[], id: string): CachedMessage[] { return items.map(item => item.id === id && !item.read ? { ...item, read: true, ack_pending: true } : { ...item }) }
