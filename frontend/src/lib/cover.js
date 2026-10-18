/** 文件作用：将封面文案布局成不重叠的矩形，供预览、列表和详情共用。 */
export const COVER_WIDTH = 360
export const COVER_HEIGHT = 225
export const POSITIONS = [
  ['top-left', '左上'], ['top-center', '上方'], ['top-right', '右上'],
  ['middle-left', '左侧'], ['middle-center', '居中'], ['middle-right', '右侧'],
  ['bottom-left', '左下'], ['bottom-center', '下方'], ['bottom-right', '右下']
]

/**
 * 将文字按完整表情及组合字符分段，兼容未提供 Intl.Segmenter 的小程序运行环境。
 * @param {string} text - 需要分段的文字。
 * @returns {string[]} 不拆分连接符表情、肤色、旗帜及组合音标的字符组。
 */
function characters(text) {
  const result = []
  for (const character of Array.from(text)) {
    const code = character.codePointAt(0)
    const previous = result[result.length - 1] || ''
    const modifier = (code >= 0x300 && code <= 0x36f) || (code >= 0x1ab0 && code <= 0x1aff) || (code >= 0x1dc0 && code <= 0x1dff) || (code >= 0x20d0 && code <= 0x20ff) || (code >= 0xfe20 && code <= 0xfe2f) || (code >= 0xfe00 && code <= 0xfe0f) || (code >= 0x1f3fb && code <= 0x1f3ff) || (code >= 0xe0020 && code <= 0xe007f) || (code >= 0xe0100 && code <= 0xe01ef)
    const pairedFlag = code >= 0x1f1e6 && code <= 0x1f1ff && Array.from(previous).length === 1 && previous.codePointAt(0) >= 0x1f1e6 && previous.codePointAt(0) <= 0x1f1ff
    if (previous && (modifier || code === 0x200d || previous.endsWith('\u200d') || pairedFlag)) result[result.length - 1] += character
    else result.push(character)
  }
  return result
}

/**
 * 按服务端十二码点上限裁剪文字，只在完整字符组边界截断。
 * @param {unknown} value - 编辑器或接口返回的文字。
 * @returns {string} 不含换行且不会截断完整表情的文案。
 */
export function normalizeLabelText(value) {
  let text = '', length = 0
  for (const character of characters(String(value ?? '').replace(/[\r\n\t]/g, ' ').trim())) {
    const size = Array.from(character).length
    if (length + size > 12) break
    text += character
    length += size
  }
  return text
}

/**
 * 为中英文和完整表情预留安全字宽，避免粗体 W 或表情被窄标签遮挡。
 * @param {string} text - 已裁剪的文字。
 * @param {number} fontSize - 设计画布字号。
 * @returns {number} 包含左右内边距的标签宽度。
 */
function textWidth(text, fontSize) {
  const units = characters(text).reduce((sum, character) => sum + (/^[ilI1.,'`!|:; ]$/.test(character) ? 0.48 : /^[MW@#%&]$/.test(character) ? 1.05 : /^[\x20-\x7e]$/.test(character) ? 0.76 : 1.12), 0)
  return Math.min(344, units * fontSize + 16)
}

/**
 * 取得当前游戏可选择的去重底图，空图库时使用该游戏默认图。
 * @param {object|null} game - 游戏配置。
 * @returns {string[]} 独立底图路径，不包含文字图层。
 */
export function gameCovers(game) {
  const covers = (Array.isArray(game?.covers) ? game.covers : []).filter(src => typeof src === 'string' && src.trim())
  return [...new Set(covers.length ? covers : game?.cover_url ? [game.cover_url] : [])]
}

/**
 * 随机选取游戏底图；有多张时排除当前图，单张图库保持原图。
 * @param {object|null} game - 游戏配置。
 * @param {string} current - 主动换图时需要排除的当前底图。
 * @param {Function} random - 随机数来源，便于验证边界。
 * @returns {string} 随机底图路径或空字符串。
 */
export function randomGameCover(game, current = '', random = Math.random) {
  const covers = gameCovers(game)
  const alternatives = covers.filter(src => src !== current)
  const options = alternatives.length ? alternatives : covers
  const value = Number(random())
  const index = Math.min(options.length - 1, Math.max(0, Math.floor((Number.isFinite(value) ? value : 0) * options.length)))
  return options[index] || ''
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
  const entries = (Array.isArray(labels) ? labels : []).filter(label => label && typeof label === 'object').slice(0, 6).map((label, index) => {
    const text = normalizeLabelText(label.text)
    const requestedSize = Number(label.font_size)
    const size = Number.isFinite(requestedSize) ? Math.min(24, Math.max(12, Math.round(requestedSize) || 16)) : 16
    const position = POSITIONS.some(item => item[0] === label.position) ? label.position : 'bottom-left'
    return { ...label, id: label.id || `label-${index}`, text, position, font_size: size, w: textWidth(text, size), h: size + 7 }
  }).filter(label => label.text)
  const placed = []
  let crowded = false
  for (const entry of entries) {
    const desired = desiredPosition(entry.position, entry.w, entry.h)
    const candidates = [{ ...desired, w: entry.w, h: entry.h }]
    for (let x = 8; x <= 352 - entry.w; x += 4) candidates.push({ x, y: desired.y, w: entry.w, h: entry.h })
    for (let y = 8; y <= 217 - entry.h; y += 4) {
      for (let x = 8; x <= 352 - entry.w; x += 4) candidates.push({ x, y, w: entry.w, h: entry.h })
    }
    candidates.sort((a, b) => entry.position === 'bottom-left' ? b.y - a.y || a.x - b.x : Math.abs(a.x - desired.x) + Math.abs(a.y - desired.y) * 2 - Math.abs(b.x - desired.x) - Math.abs(b.y - desired.y) * 2)
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
