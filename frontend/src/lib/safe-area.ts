/** 文件作用：统一微信状态栏、胶囊和悬浮返回位置测量，H5零占位，原生异常采用安全默认尺寸。 */

/** 所有尺寸单位为逻辑像素，水平与垂直避让分别命名。 */
export interface SafeAreaMetrics {
  statusBarHeight: number;
  /** 状态栏下胶囊导航区域的垂直占高，包含上下间隙。 */
  capsuleSpace: number;
  /** 页面右侧需避让胶囊的水平宽度，已包含12像素间隔。 */
  capsuleRightInset: number;
  /** 悬浮返回按钮的顶部位置，与有效胶囊顶部对齐。 */
  floatingTop: number;
  /** 完整状态栏加胶囊导航区域的底部位置。 */
  contentTop: number;
}

/** 检查平台给出的数值是否有限且在合理像素范围内。 */
function valid(value: unknown, min: number, max: number): value is number { return typeof value === 'number' && Number.isFinite(value) && value >= min && value <= max }

/** 读取当前平台安全区；每次独立读取，窗口尺寸变化后可重新调用，不修改平台状态。 */
export function readSafeArea(): SafeAreaMetrics {
  // #ifdef H5
  return { statusBarHeight: 0, capsuleSpace: 0, capsuleRightInset: 0, floatingTop: 0, contentTop: 0 }
  // #endif
  // #ifdef MP-WEIXIN
  let statusBarHeight = 20, windowWidth = 375
  try { const window = uni.getWindowInfo(); if (valid(window.statusBarHeight, 0, 100)) statusBarHeight = window.statusBarHeight; if (valid(window.windowWidth, 100, 10000)) windowWidth = window.windowWidth } catch { /* 旧基础库或原生接口异常时仍保留可用布局。 */ }
  let top = statusBarHeight + 8, height = 32, left = windowWidth - 96
  try {
    const capsule = uni.getMenuButtonBoundingClientRect()
    if (valid(capsule.top, statusBarHeight, statusBarHeight + 100) && valid(capsule.height, 20, 60) && valid(capsule.left, 0, windowWidth) && valid(capsule.right, capsule.left + 20, windowWidth)) { top = capsule.top; height = capsule.height; left = capsule.left }
  } catch { /* 微信胶囊暂不可读时使用标准胶囊尺寸，避免内容进入系统操作区。 */ }
  const capsuleSpace = (top - statusBarHeight) * 2 + height
  return { statusBarHeight, capsuleSpace, capsuleRightInset: windowWidth - left + 12, floatingTop: top, contentTop: statusBarHeight + capsuleSpace }
  // #endif
}
