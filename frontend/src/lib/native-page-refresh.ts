/** 文件作用：为原生滚动的二级页面统一注册下拉刷新与触底事件，管理动画、并发及页面隐藏后的交互状态。 */
import { ref, watch } from 'vue'
import { onHide, onShow, onUnload, onPullDownRefresh, onReachBottom } from '@dcloudio/uni-app'
import { notify } from './format'

/** 页面保留自己的身份、筛选及分页守卫；共享层只负责原生手势调度。 */
export interface NativePageRefreshOptions {
  refresh: () => Promise<void>
  more?: () => Promise<void>
  blocked?: () => boolean
  context?: () => unknown
}

/**
 * 接入二级页原生刷新，刷新可使正在追加的旧页失效，数据回填仍由领域加载方法校验。
 * @param options - 只读刷新、可选下一页及资金操作或草稿弹层期间的阻止条件。
 * @returns 可供错误重试使用的同一入口及当前手势加载状态。
 */
export function useNativePageRefresh(options: NativePageRefreshOptions) {
  const refreshing = ref(false), loadingMore = ref(false)
  let visible = true, version = 0

  /** 停止官方动画；页面隐藏或受阻时也必须结束下拉状态。 */
  function stopRefresh(): void { uni.stopPullDownRefresh() }

  /** 当前页下拉只执行一次，意外错误保留统一轻提示，最终停止原生动画。 */
  async function refresh(): Promise<void> {
    if (refreshing.value) return
    if (!visible || options.blocked?.()) { stopRefresh(); return }
    const current = ++version
    loadingMore.value = false
    refreshing.value = true
    try { await options.refresh() }
    catch (error) { if (visible && current === version) notify(error) }
    finally { if (current === version) { refreshing.value = false; stopRefresh() } }
  }

  /** 触底只追加一次，刷新或业务弹层期间不启动下一页。 */
  async function loadMore(): Promise<void> {
    if (!visible || refreshing.value || loadingMore.value || !options.more || options.blocked?.()) return
    const current = version
    loadingMore.value = true
    try { await options.more() }
    catch (error) { if (visible && current === version) notify(error) }
    finally { if (current === version) loadingMore.value = false }
  }

  /** 筛选或身份变化立即取消旧交互标记，旧回包不能关闭新上下文的刷新动画。 */
  function invalidate(): void { version++; refreshing.value = false; loadingMore.value = false; stopRefresh() }
  /** 隐藏立即撤销交互，数据回填仍由页面自己的请求版本负责。 */
  function hide(): void { visible = false; invalidate() }
  if (options.context) watch(options.context, invalidate, { flush: 'sync' })
  onShow(() => { visible = true })
  onHide(hide)
  onUnload(hide)
  onPullDownRefresh(() => { void refresh() })
  onReachBottom(() => { void loadMore() })
  return { refreshing, loadingMore, refresh, loadMore, invalidate }
}
