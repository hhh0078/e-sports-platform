/** 文件作用：为俱乐部图文图片生成可信尺寸占位，避免冷图加载改变后续内容位置。 */
import type { CSSProperties } from 'vue'
import type { CommunityBlock } from '../types/models'

/** 使用服务端保存的实际宽高预留比例；旧图无元信息时保持16:9稳定占位，图片完整适配显示。 */
export function communityImageStyle(block: CommunityBlock): CSSProperties {
  const width = block.image_width, height = block.image_height
  const ratio = typeof width === 'number' && typeof height === 'number' && Number.isSafeInteger(width) && Number.isSafeInteger(height) && width > 0 && height > 0 ? height / width : 9 / 16
  return { paddingTop: ratio * 100 + '%' }
}
