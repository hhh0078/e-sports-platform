/** 文件作用：为所有卡片共享窗口宽度及单一原生尺寸监听，避免长列表逐卡读取原生信息。 */
import { nextTick, onBeforeUnmount, onMounted, readonly, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'

const viewportWidth = ref(390)
let consumers = 0, refreshQueued = false

/** 同一轮挂载和显示只读取一次原生窗口，接口暂不可用时保留已有有效宽度。 */
function refreshViewport(): void {
  if (refreshQueued) return
  refreshQueued = true
  void nextTick(() => {
    refreshQueued = false
    if (!consumers) return
    try {
      const width = uni.getWindowInfo().windowWidth
      if (Number.isFinite(width) && width > 0) viewportWidth.value = width
    } catch { /* 原生窗口尚未就绪时等待下次显示或尺寸事件，不逐卡重试。 */ }
  })
}

/** 优先直接使用原生尺寸事件的宽度，缺失有效尺寸时合并回退读取。 */
function windowResized(event?: { size?: { windowWidth?: number } }): void {
  const width = event?.size?.windowWidth
  if (typeof width === 'number' && Number.isFinite(width) && width > 0) viewportWidth.value = width
  else refreshViewport()
}

/** 注册组件对共享视口的使用，最后一个使用者卸载后移除唯一原生监听。 */
export function useViewportWidth() {
  let registered = false
  onMounted(() => {
    if (registered) return
    registered = true
    if (consumers++ === 0) uni.onWindowResize?.(windowResized)
    refreshViewport()
  })
  onShow(refreshViewport)
  onBeforeUnmount(() => {
    if (!registered) return
    registered = false
    if (consumers > 0 && --consumers === 0) uni.offWindowResize?.(windowResized)
  })
  return readonly(viewportWidth)
}
