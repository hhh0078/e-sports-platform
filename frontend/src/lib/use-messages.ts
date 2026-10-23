/** 文件作用：管理系统消息的分页收取、本机已读后远端确认、失败重试与账号/页面竞态隔离。 */
import { ref, computed, watch } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import { listSystemMessages, acknowledgeMessage } from '../api/messages.api'
import { errorMessage, token } from '../api/client'
import { state, init, sessionRevision } from './store'
import { readMessageCache, writeMessageCache, mergeMessageCache, markCachedMessageRead } from './messages-cache'
import type { CachedMessage, MessageInbox } from '../types/messages'
import { useNativePageRefresh } from './native-page-refresh'

/** 详情只读取本机消息，不再请求可能已删除的远端正文。 */
interface MessageViewOptions { detailId?: () => string }

/** 站内消息页面共用可靠本机记录，未读消息永不确认删除。 */
export function useMessages(options: MessageViewOptions = {}) {
  const items = ref<CachedMessage[]>([]), selected = ref<CachedMessage | null>(null), loading = ref(false), error = ref(''), cacheOwner = ref('')
  const unreadCount = computed(() => items.value.filter(item => !item.read).length), pendingCount = computed(() => items.value.filter(item => item.read && item.ack_pending).length)
  const renderLimit=ref(20),visibleItems=computed(()=>items.value.slice(0,renderLimit.value)),hasMoreLocal=computed(()=>renderLimit.value<items.value.length)
  let visible = false, disposed = false, pageVersion = 0, operationVersion = 0

  /** 捕获明确账号、令牌和页面版本，旧响应不能写入另一身份。 */
  function currentSession() { return { id: state.user?.id || '', token: token(), revision: sessionRevision(), page: pageVersion } }
  /** 只有当前可见页面的原账号可接受网络结果或继续确认删除。 */
  function sameSession(session: ReturnType<typeof currentSession>): boolean { return !disposed && visible && !!session.id && session.id === state.user?.id && session.token === token() && session.revision === sessionRevision() && session.page === pageVersion }
  /** 同步内存与详情选择，不向本机缓存写入任何其他身份数据。 */
  function showCached(userID: string, values: CachedMessage[]): void { cacheOwner.value = userID; items.value = values; selected.value = options.detailId ? values.find(item => item.id === options.detailId?.()) || null : null }
  /** 读取响应分页契约；异常响应不能伪装成空列表或触发确认。 */
  function validateInbox(value: MessageInbox): void { if (!value || !Array.isArray(value.items) || !Number.isSafeInteger(value.total) || value.total < 0) throw new Error('消息响应格式异常，未删除服务器消息') }
  /** 仅确认本机已读且持久成功的消息，确认失败保留待重试状态。 */
  async function acknowledgePending(session: ReturnType<typeof currentSession>, operation: number, onlyID?: string): Promise<void> {
    let firstFailure = ''
    const pending = readMessageCache(session.id).filter(item => item.read && item.ack_pending && (!onlyID || item.id === onlyID))
    for (const item of pending) {
      if (!sameSession(session) || operation !== operationVersion) return
      const before = readMessageCache(session.id).find(current => current.id === item.id)
      if (!before?.read || !before.ack_pending) continue
      try { const result = await acknowledgeMessage(item.id); if (!result?.ok) throw new Error('服务器未确认已读消息') }
      catch (err) { if (sameSession(session) && operation === operationVersion) firstFailure ||= errorMessage(err); continue }
      if (!sameSession(session) || operation !== operationVersion) return
      const latest = readMessageCache(session.id), updated = latest.map(current => current.id === item.id ? { ...current, ack_pending: false } : current)
      writeMessageCache(session.id, updated)
      if (sameSession(session)) showCached(session.id, updated)
    }
    if (firstFailure && sameSession(session) && operation === operationVersion) error.value = '已读状态已保存在本机，服务器确认待重试：' + firstFailure
  }
  /** 先收完整分页并持久全文，最后才重试已读确认，避免删除改变offset导致跳页。 */
  async function receiveMessages(session: ReturnType<typeof currentSession>, operation: number): Promise<void> {
    let offset = 0
    while (sameSession(session) && operation === operationVersion) {
      const result = await listSystemMessages({ limit: 100, offset })
      if (!sameSession(session) || operation !== operationVersion) return
      validateInbox(result)
      const local = readMessageCache(session.id), merged = mergeMessageCache(session.id, local, result.items)
      if (result.items.length) writeMessageCache(session.id, merged)
      showCached(session.id, merged)
      offset += result.items.length
      if (offset >= result.total) break
      if (!result.items.length) throw new Error('消息列表发生变化，请刷新后继续接收；未删除消息')
    }
    if (sameSession(session) && operation === operationVersion) await acknowledgePending(session, operation)
  }
  /** 先把详情已读标记和全文一起落盘，失败时保留正文并绝不发送DELETE。 */
  async function openLocalMessage(session: ReturnType<typeof currentSession>, operation: number, id: string): Promise<void> {
    const local = readMessageCache(session.id), found = local.find(item => item.id === id)
    showCached(session.id, local)
    if (!found || !sameSession(session)) return
    const updated = markCachedMessageRead(local, id)
    if (!found.read) { writeMessageCache(session.id, updated); if (!sameSession(session)) return; showCached(session.id, updated) }
    if (sameSession(session) && operation === operationVersion) await acknowledgePending(session, operation, id)
  }
  /** 显示列表时收取消息，显示详情时只读本机；游客使用正常空态。 */
  async function showPage(): Promise<void> {
    if (disposed) return
    visible = true; const page = ++pageVersion, operation = ++operationVersion; loading.value = true; error.value = ''
    try {
      await init(); if (!visible || page !== pageVersion || disposed) return
      const session = currentSession()
      if (!session.id) { showCached('', []); return }
      const local = readMessageCache(session.id); showCached(session.id, local)
      if (options.detailId) await openLocalMessage(session, operation, options.detailId())
      else await receiveMessages(session, operation)
    } catch (err) { if (visible && page === pageVersion && operation === operationVersion) error.value = errorMessage(err) }
    finally { if (page === pageVersion && operation === operationVersion) loading.value = false }
  }
  /** 用户主动重试本机已读确认，不重新依赖服务端正文，也不删除任何未读。 */
  async function retryPending(): Promise<void> {
    if (!visible || loading.value || !state.user?.id) return
    const session = currentSession(), operation = ++operationVersion; loading.value = true; error.value = ''
    try { await acknowledgePending(session, operation, options.detailId?.()) }
    catch (err) { if (sameSession(session) && operation === operationVersion) error.value = errorMessage(err) }
    finally { if (sameSession(session) && operation === operationVersion) loading.value = false }
  }
  /** 清除内存中的旧身份数据，但按要求保留该账号在设备里的消息历史。 */
  function clearView(): void { items.value = []; selected.value = null; cacheOwner.value = ''; error.value = ''; loading.value = false;renderLimit.value=20 }
  /** 页面隐藏立即作废迟到响应，远端未确认消息留待下一次同步。 */
  function hidePage(): void { visible = false; pageVersion++; operationVersion++; clearView() }
  watch([() => state.user?.id, sessionRevision], () => { const wasVisible = visible; pageVersion++; operationVersion++; clearView(); if (wasVisible) void showPage() }, { flush: 'sync' })
  onShow(showPage)
  onHide(hidePage)
  onUnload(() => { disposed = true; hidePage() })
  if(!options.detailId)useNativePageRefresh({refresh:showPage,context:()=>[state.user?.id,state.user?.role,sessionRevision()],more:async()=>{if(hasMoreLocal.value)renderLimit.value+=20},blocked:()=>loading.value})
  return { items,visibleItems,hasMoreLocal, selected, loading, error, cacheOwner, unreadCount, pendingCount, showPage, retryPending }
}
