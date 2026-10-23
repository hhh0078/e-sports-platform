/** 文件作用：统一390px设计基线到750rpx及实际设备像素的换算，供共享图标和实测绘制边界使用。 */
export const DESIGN_WIDTH = 390
export const RPX_WIDTH = 750

/**
 * 将设计尺寸转换为rpx数值，避免组件各自采用不同换算基线。
 * @param value - 390px设计图中的尺寸。
 * @returns 保留四位小数的rpx数值。
 */
export function designRpxValue(value: number): number {
  if (!Number.isFinite(value)) throw new Error('设计尺寸必须是有限数值')
  return Math.round(value * RPX_WIDTH / DESIGN_WIDTH * 10000) / 10000
}

/**
 * 将设计尺寸转换为可直接用于内联样式的rpx字符串。
 * @param value - 390px设计图中的尺寸。
 * @returns 带rpx单位的样式值。
 */
export function designRpx(value: number): string { return designRpxValue(value) + 'rpx' }

/**
 * 根据实际视口宽度换算设计尺寸，供必须接收物理布局坐标的接口使用。
 * @param value - 390px设计图中的尺寸。
 * @param windowWidth - 原生接口测得的逻辑像素宽度。
 * @returns 实际设备上的逻辑像素值。
 */
export function designPixels(value: number, windowWidth: number): number {
  if (!Number.isFinite(value) || !Number.isFinite(windowWidth) || windowWidth <= 0) throw new Error('实际窗口与设计尺寸无效')
  return value * windowWidth / DESIGN_WIDTH
}
