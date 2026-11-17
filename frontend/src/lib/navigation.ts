/** 文件作用：维护四个独立主页面的路由、参数交接及二级页面返回导航。 */
import { reactive, watch, onBeforeUnmount } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import { isTimelinePreview } from './sharing'

/** 四个独立主页面的标识。 */
export type MainTab = 'player' | 'worker' | 'orders' | 'me'
/** 路由查询项，保持微信页面参数的字符串形式。 */
export type PageQuery = Record<string, string | undefined>

export const MAIN_PAGE = '/pages/main/index'
/** 四个实际注册的主导航页面；订单可无参数直接打开。 */
export const MAIN_PAGES:Record<MainTab,string>={player:MAIN_PAGE,worker:'/pages/main/worker/index',orders:'/pages/main/orders/index',me:'/pages/main/me/index'}
export const mainNavigation = reactive({ tab: 'player' as MainTab, query: {} as PageQuery, revision: 0 })
const aliases: Record<string, string> = {
  '/pages/index/index': MAIN_PAGE + '?tab=player',
  '/pages/worker/index': MAIN_PAGES.worker,
  '/pages/orders/index': MAIN_PAGES.orders,
  '/pages/me/index': MAIN_PAGES.me,
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
  if(target.split('?')[0]===MAIN_PAGE){
    const values=parsePageQuery(target+(query?(target.includes('?')?'&':'?')+query:''))
    const tab=(['player','worker','orders','me'] as const).find(item=>item===values.tab)||'player'
    delete values.tab
    const rest=Object.entries(values).filter(([,v])=>v!==undefined).map(([k,v])=>encodeURIComponent(k)+'='+encodeURIComponent(v!)).join('&')
    return MAIN_PAGES[tab]+(rest?'?'+rest:'')
  }
  return target+(query?(target.includes('?')?'&':'?')+query:'')
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

/** 直接进入独立主页面时交接筛选参数，普通返回不重置本页筛选。 */
export function applyTabQuery(tab:MainTab,query:PageQuery={}):void{if(Object.keys(query).length||mainNavigation.tab!==tab)selectMainTab(tab,query)}

/** 主导航只用switchTab；先交接参数，因为官方switchTab的URL不能带查询串。 */
export function navigate(url:string):void{
 if(isTimelinePreview()){uni.showToast({title:'请从微信顶部打开小程序后继续',icon:'none'});return}
 const target=normalizePageUrl(url),path=target.split('?')[0],fail=()=>uni.showToast({title:'页面打开失败，请重试',icon:'none'})
 const tab=(Object.keys(MAIN_PAGES) as MainTab[]).find(key=>MAIN_PAGES[key]===path)
 if(!tab){uni.navigateTo({url:target,fail});return}
 selectMainTab(tab,parsePageQuery(target))
 const pages=getCurrentPages()
 if(pages.at(-1)?.route===path.slice(1))return
 uni.switchTab({url:path,fail})
}

/** 统一二级页面返回；无历史或返回失败时进入独立首页。 */
export function back():void{
 if(isTimelinePreview()){uni.showToast({title:'请从微信顶部打开小程序查看详情',icon:'none'});return}
 if(getCurrentPages().length>1)uni.navigateBack({delta:1,fail:()=>navigate(MAIN_PAGE)})
 else navigate(MAIN_PAGE)
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
