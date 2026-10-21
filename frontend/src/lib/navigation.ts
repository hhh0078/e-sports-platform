/** 文件作用：维护唯一主页面的标签状态、深链归一化及二级页面返回导航。 */
import { reactive, watch, onBeforeUnmount } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'

/** 主页面四个常驻视图的标识。 */
export type MainTab = 'player' | 'worker' | 'orders' | 'me'
/** 路由查询项，保持微信页面参数的字符串形式。 */
export type PageQuery = Record<string, string | undefined>

export const MAIN_PAGE = '/pages/main/index'
export const mainNavigation = reactive({ tab: 'player' as MainTab, query: {} as PageQuery, revision: 0 })
const aliases: Record<string, string> = {
  '/pages/index/index': MAIN_PAGE + '?tab=player',
  '/pages/worker/index': MAIN_PAGE + '?tab=worker',
  '/pages/orders/index': MAIN_PAGE + '?tab=orders',
  '/pages/me/index': MAIN_PAGE + '?tab=me',
  '/pages/service/index': '/pages/services/detail',
  '/pages/publish/index': '/pages/services/publish',
  '/pages/request/index': '/pages/requests/publish',
  '/pages/order/index': '/pages/orders/detail',
  '/pages/login/index': '/pages/account/login',
  '/pages/profile/index': '/pages/account/profile',
  '/pages/agreements/index': '/pages/account/agreements',
  '/pages/clubs/index': '/pages/community/clubs/index',
  '/pages/club/index': '/pages/community/clubs/detail',
  '/pages/club-edit/index': '/pages/community/clubs/edit',
  '/pages/player/index': '/pages/community/players/detail',
  '/pages/admin/index': '/pages/manage/index',
  '/pages/admin-settings/index': '/pages/manage/settings',
}

/** 将旧分享路径和入口统一转换为当前业务路由，保留查询参数。 */
export function normalizePageUrl(url: string): string {
  const index = url.indexOf('?')
  const path = index < 0 ? url : url.slice(0, index)
  const query = index < 0 ? '' : url.slice(index + 1)
  const target = aliases[path] || path
  return target + (query ? (target.includes('?') ? '&' : '?') + query : '')
}

/** 解读内部页面查询串，非法编码保持原文，避免破坏导航。 */
export function parsePageQuery(url: string): PageQuery {
  const query: PageQuery = {}
  const text = url.split('?').slice(1).join('?')
  for (const part of text.split('&')) {
    if (!part) continue
    const index = part.indexOf('=')
    const key = index < 0 ? part : part.slice(0, index)
    const value = index < 0 ? '' : part.slice(index + 1)
    try { query[decodeURIComponent(key)] = decodeURIComponent(value) }
    catch { query[key] = value }
  }
  return query
}

/** 只更新主视图状态，保持页面实例与滚动容器不重建。 */
export function selectMainTab(tab: MainTab, query: PageQuery = {}): void {
  mainNavigation.tab = tab
  mainNavigation.query = query
  mainNavigation.revision++
}

/** 应用入口查询，未知主标签回到玩家页。 */
export function applyMainQuery(query: PageQuery = {}): void {
  const tabs: MainTab[] = ['player', 'worker', 'orders', 'me']
  const tab = tabs.find(item => item === query.tab) || 'player'
  selectMainTab(tab, query)
}

/** 打开页面；主页面已经在栈中时直接返回并切换内容。 */
export function navigate(url: string): void {
  const target = normalizePageUrl(url)
  const fail = () => uni.showToast({ title: '页面打开失败，请重试', icon: 'none' })
  if (target.split('?')[0] !== MAIN_PAGE) {
    uni.navigateTo({ url: target, fail })
    return
  }
  applyMainQuery(parsePageQuery(target))
  const pages = getCurrentPages()
  const mainIndex = pages.findIndex(page => page.route === MAIN_PAGE.slice(1))
  if (mainIndex === pages.length - 1 && mainIndex >= 0) return
  if (mainIndex >= 0) uni.navigateBack({ delta: pages.length - 1 - mainIndex, fail })
  else uni.reLaunch({ url: target, fail })
}

/** 统一普通顶栏与悬浮按钮的返回行为；无历史或原生返回失败时回到唯一主页面。 */
export function back(): void {
  const url = MAIN_PAGE + '?tab=player'
  if (getCurrentPages().length > 1) uni.navigateBack({ delta: 1, fail: () => uni.reLaunch({ url, fail: () => uni.showToast({ title: '返回失败，请重新打开小程序', icon: 'none' }) }) })
  else navigate(url)
}

/**
 * 刷新主视图已加载的完整列表并多读一条判断后续页，避免返回后缩为首屏导致滚动截断。
 * loadedCount 为当前展示数量；read 每次最多读取一百条；current 在隐藏或会话失效时终止后续请求。
 */
export async function refreshMainList<T>(loadedCount: number, pageSize: number, read: (limit: number, offset: number) => Promise<T[]>, current: () => boolean): Promise<{ items: T[]; hasMore: boolean } | null> {
  const target = Math.max(pageSize, loadedCount), items: T[] = []
  while (items.length <= target) {
    if (!current()) return null
    const limit = Math.min(100, target + 1 - items.length)
    const batch = await read(limit, items.length)
    if (!current()) return null
    items.push(...batch)
    if (batch.length < limit) break
  }
  return { items: items.slice(0, target), hasMore: items.length > target }
}

/** 为常驻视图合并页面生命周期与标签可见性，隐藏时停止业务请求。 */
export function useMainVisibility(isActive: () => boolean, show: () => void, hide: () => void): void {
  let pageVisible = false
  let currentVisible = false
  /** 仅在可见性确实变化时激活或停止当前视图。 */
  function synchronize(): void {
    const next = pageVisible && isActive()
    if (next === currentVisible) return
    currentVisible = next
    if (next) show()
    else hide()
  }
  onShow(() => { pageVisible = true; synchronize() })
  onHide(() => { pageVisible = false; synchronize() })
  watch(isActive, synchronize, { immediate: true })
  onBeforeUnmount(() => { if (currentVisible) hide() })
}
