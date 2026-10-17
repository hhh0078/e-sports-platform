/** 文件作用：将封面文案布局成不重叠的矩形，供预览、列表和详情共用。 */
export const COVER_WIDTH = 360
export const COVER_HEIGHT = 225
export const POSITIONS = [
  ['top-left', '左上'], ['top-center', '上方'], ['top-right', '右上'],
  ['middle-left', '左侧'], ['middle-center', '居中'], ['middle-right', '右侧'],
  ['bottom-left', '左下'], ['bottom-center', '下方'], ['bottom-right', '右下']
]

/** 按中英文字符估计标签宽度，并保留内边距；返回设计坐标中的宽度。 */
function textWidth(text, fontSize) {
  return Math.min(344, [...text].reduce((sum, char) => sum + (/[^\x00-\xff]/.test(char) ? 1 : 0.64) * fontSize, 0) + 16)
}

/** 判断两块标签是否碰撞，保留四个设计像素的阅读间距。 */
function overlaps(a, b) {
  return a.x < b.x + b.w + 4 && a.x + a.w + 4 > b.x && a.y < b.y + b.h + 4 && a.y + a.h + 4 > b.y
}

/** 根据九宫格位置计算期望左上坐标，返回设计坐标。 */
function desiredPosition(position, width, height) {
  const [vertical, horizontal] = String(position || 'bottom-left').split('-')
  return {
    x: horizontal === 'right' ? 352 - width : horizontal === 'center' ? (360 - width) / 2 : 8,
    y: vertical === 'top' ? 8 : vertical === 'middle' ? (225 - height) / 2 : 217 - height
  }
}

/** 为最多六个标签生成百分比坐标；发生冲突时就近排列，极端拥挤时退回分行布局。 */
export function layoutLabels(labels = []) {
  const entries = (Array.isArray(labels) ? labels : []).slice(0, 6).map((label, index) => {
    const text = [...String(label.text || '')].slice(0, 12).join('')
    const size = Math.min(24, Math.max(12, Number(label.font_size) || 16))
    return { ...label, id: label.id || `label-${index}`, text, font_size: size, w: textWidth(text, size), h: size + 6 }
  }).filter(label => label.text)
  const placed = []
  let crowded = false
  for (const entry of entries) {
    const desired = desiredPosition(entry.position, entry.w, entry.h)
    const candidates = [{ ...desired, w: entry.w, h: entry.h }]
    for (let y = 8; y <= 217 - entry.h; y += 4) {
      for (let x = 8; x <= 352 - entry.w; x += 4) candidates.push({ x, y, w: entry.w, h: entry.h })
    }
    candidates.sort((a, b) => Math.abs(a.x - desired.x) + Math.abs(a.y - desired.y) * 2 - Math.abs(b.x - desired.x) - Math.abs(b.y - desired.y) * 2)
    const spot = candidates.find(candidate => !placed.some(previous => overlaps(candidate, previous)))
    if (!spot) { crowded = true; break }
    placed.push({ ...entry, ...spot })
  }
  const result = crowded ? entries.map((entry, index) => ({
    ...entry,
    x: desiredPosition(entry.position, entry.w, entry.h).x,
    y: 217 - entries.slice(index).reduce((sum, item) => sum + item.h, 0) - (entries.length - index - 1) * 4
  })) : placed
  return result.map(entry => ({
    ...entry,
    left: entry.x / COVER_WIDTH * 100,
    top: entry.y / COVER_HEIGHT * 100,
    width: entry.w / COVER_WIDTH * 100,
    height: entry.h / COVER_HEIGHT * 100
  }))
}

/** 创建默认左下角文案标签；返回可直接保存到服务的配置。 */
export function newLabel(color = '#00e18a') {
  const rgb = Number.parseInt(String(color).replace('#', ''), 16)
  const brightness = ((rgb >> 16) & 255) * 0.299 + ((rgb >> 8) & 255) * 0.587 + (rgb & 255) * 0.114
  return { id: `label-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`, text: '新文字', color: brightness > 128 ? '#101714' : '#ffffff', background: color, position: 'bottom-left', font_size: 16 }
}
