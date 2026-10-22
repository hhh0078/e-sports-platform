/** 文件作用：复用管理子页的具名资源分页、身份隔离和页面生命周期，页面保留各自业务布局。 */
import { ref, shallowRef, watch } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import { listAdminResource } from '../api/admin.api'
import type { AdminResources } from '../api/admin.api'
import { errorMessage, token } from '../api/client'
import type { ListQuery } from '../types/models'
import { state, init, sessionRevision } from './store'
import { notify } from './format'

/** 子页只声明自己的资源与筛选，原生选图期间可保留同账号编辑草稿。 */
interface ManageResourceOptions {
  params?: () => ListQuery
  preserveDraft?: () => boolean
  clearDraft?: () => void
}

/** 创建一个独立管理资源列表；旧账号、旧搜索和隐藏页面的结果均不能回填。 */
export function useManageResource<K extends keyof AdminResources>(resource: K, options: ManageResourceOptions = {}) {
  const items = shallowRef<AdminResources[K][]>([]), query = ref(''), loading = ref(false), busy = ref(false), error = ref(''), hasMore = ref(false)
  let visible = false, loadVersion = 0, actionVersion = 0, showVersion = 0, nextOffset = 0, appliedSearch = ''

  /** 捕获当前登录身份，不把原生选图隐藏误判为账号切换。 */
  function currentSession() { return { id: state.user?.id, token: token(), revision: sessionRevision() } }
  /** 仅同一管理员可继续操作；上传可明确放宽页面可见限制。 */
  function sameSession(session: ReturnType<typeof currentSession>, requireVisible = true): boolean { return (!requireVisible || visible) && state.user?.role === 'admin' && session.id === state.user.id && session.token === token() && session.revision === sessionRevision() }
  /** 清除分页及错误，不保留其他管理员的资源。 */
  function clearList(): void { items.value = []; nextOffset = 0; appliedSearch = ''; hasMore.value = false; error.value = '' }
  /** 从当前资源的服务端接口搜索和分页，偏移使用原始记录数以免去重造成漏页。 */
  async function loadData(append = false): Promise<void> {
    if (!visible || state.user?.role !== 'admin' || (append && (loading.value || !hasMore.value))) return
    const version = ++loadVersion, session = currentSession(), search = query.value.trim(), extra = options.params?.() || {}, filter = JSON.stringify(extra), searchKey = JSON.stringify([search, filter]), extending = append && appliedSearch === searchKey, offset = extending ? nextOffset : 0
    loading.value = true; error.value = ''
    try {
      const result = await listAdminResource(resource, { ...extra, limit: 20, offset, q: search })
      if (version !== loadVersion || !sameSession(session) || search !== query.value.trim() || filter !== JSON.stringify(options.params?.() || {})) return
      const known = new Set(extending ? items.value.map(item => item.id) : [])
      items.value = extending ? [...items.value, ...result.filter(item => !known.has(item.id))] : result
      nextOffset = offset + result.length; appliedSearch = searchKey; hasMore.value = result.length === 20
    } catch (err) { if (version === loadVersion && sameSession(session)) error.value = errorMessage(err) }
    finally { if (version === loadVersion) loading.value = false }
  }
  /** 对真实审核操作去重，成功后刷新同账号列表，旧响应不发成功提示。 */
  async function mutate(action: () => Promise<unknown>, message = '处理结果已保存'): Promise<void> {
    if (!visible || busy.value || state.user?.role !== 'admin') return
    const version = ++actionVersion, session = currentSession(); busy.value = true; error.value = ''
    try { await action(); if (version !== actionVersion || !sameSession(session)) return; await loadData(); if (version === actionVersion && sameSession(session)) notify(message) }
    catch (err) { if (version === actionVersion && sameSession(session)) { error.value = errorMessage(err); notify(err) } }
    finally { if (version === actionVersion) busy.value = false }
  }
  /** 显示页面后初始化身份，只为仍可见的当前管理员发起列表请求。 */
  async function showPage(): Promise<void> {
    visible = true; error.value = ''; const version = ++showVersion
    try { await init(); if (version !== showVersion || !visible) return; if (state.user?.role !== 'admin') { clearList(); options.clearDraft?.(); return }; if (!options.preserveDraft?.()) await loadData() }
    catch (err) { if (visible && version === showVersion) error.value = errorMessage(err) }
  }
  /** 隐藏即使旧请求失效；只有同管理员原生选图能保留编辑草稿。 */
  function hidePage(): void { visible = false; showVersion++; loadVersion++; actionVersion++; loading.value = false; busy.value = false; clearList(); if (!options.preserveDraft?.()) options.clearDraft?.() }
  watch([() => state.user?.id, () => state.user?.role, sessionRevision], () => { const wasVisible = visible; hidePage(); options.clearDraft?.(); if (wasVisible) void showPage() }, { flush: 'sync' })
  onShow(showPage)
  onHide(hidePage)
  onUnload(() => { hidePage(); options.clearDraft?.() })
  return { items, query, loading, busy, error, hasMore, loadData, mutate, currentSession, sameSession, showPage, hidePage }
}
