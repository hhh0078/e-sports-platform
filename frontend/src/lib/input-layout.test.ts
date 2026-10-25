/** 文件作用：按真实CSS级联校验原生单行输入的文字区，防止聚焦前后上下padding与固定行高挤压占位文字。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import postcss, { type Rule } from 'postcss'

const globalStyles = readFileSync(new URL('../styles/global.css', import.meta.url), 'utf8')

/**
 * 读取真实Vue局部样式，保留生产声明而不在测试复制控件尺寸。
 * @param path - 相对src的Vue路径。
 * @returns 所有style块的原始CSS。
 */
function localStyles(path: string): string {
  return [...readFileSync(new URL('../' + path, import.meta.url), 'utf8').matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map(match => match[1]).join('\n')
}

/**
 * 计算此次涉及的标签／类及后代选择器级联，展开padding简写以检查高权重分节覆盖。
 * @param css - 真实公共CSS和对应组件CSS。
 * @param ancestry - 从外到内的元素，例如['view.form-section','input.input']。
 * @returns 最终声明；忽略与本场景无关的媒体查询和伪类。
 */
function computedDeclarations(css: string, ancestry: string[]): Record<string, string> {
  const result = new Map<string, { value: string; priority: number }>()
  postcss.parse(css).walkRules(rule => {
    if (rule.parent?.type !== 'root') return
    for (const selector of rule.selectors) {
      const tokens = selector.trim().split(/\s+/)
      if (tokens.some(token => !/^[\w.-]+$/.test(token))) continue
      let index = ancestry.length - 1, matched = true
      for (let i = tokens.length - 1; i >= 0; i--) {
        const token = tokens[i], tag = token.match(/^[\w-]+/)?.[0], classes = [...token.matchAll(/\.([\w-]+)/g)].map(match => match[1])
        while (index >= 0 && !((!tag || ancestry[index].split('.')[0] === tag) && classes.every(name => ancestry[index].split('.').includes(name)))) index--
        if (index < 0 || (i === tokens.length - 1 && index !== ancestry.length - 1)) { matched = false; break }
        index--
      }
      if (!matched) continue
      const specificity = tokens.reduce((score, token) => score + (token.match(/\./g)?.length || 0) * 10 + (/^[\w-]/.test(token) ? 1 : 0), 0)
      rule.walkDecls(declaration => {
        const priority = specificity + (declaration.important ? 10000 : 0)
        const entries: Array<[string, string]> = [[declaration.prop, declaration.value]]
        if (declaration.prop === 'padding') {
          const values = declaration.value.split(/\s+/)
          for (const [side, value] of [['top', values[0]], ['right', values[1] || values[0]], ['bottom', values[2] || values[0]], ['left', values[3] || values[1] || values[0]]]) entries.push(['padding-' + side, value])
        }
        for (const [property, value] of entries) if (priority >= (result.get(property)?.priority ?? -1)) result.set(property, { value, priority })
      })
    }
  })
  return Object.fromEntries([...result].map(([property, entry]) => [property, entry.value]))
}

/**
 * 将真实rpx或px换成指定窗口的实际像素，控件变量继续读取公共page定义。
 * @param value - 生产CSS中的长度。
 * @param width - 320或390实际视口宽度。
 * @returns 对应像素值。
 */
function pixels(value: string | undefined, width: number): number {
  if (!value || value === '0') return 0
  const variable = value.match(/^var\((--[\w-]+)\)$/)?.[1]
  if (variable) return pixels(computedDeclarations(globalStyles, ['page'])[variable], width)
  const dimension = value.match(/^([\d.]+)(rpx|px)$/)
  assert.ok(dimension, '无法解析实际输入长度：' + value)
  return Number(dimension[1]) * (dimension[2] === 'rpx' ? width / 750 : 1)
}

test('普通与分节原生输入在微信和H5均为零上下padding，自然行盒保留足够文字高度', () => {
  for (const tag of ['input', 'uni-input']) for (const insideSection of [false, true]) {
    const ancestry = [...insideSection ? ['view.form-section'] : [], tag + '.input']
    const style = computedDeclarations(globalStyles, ancestry)
    assert.equal(style.display, 'block')
    assert.equal(style['padding-top'], '0')
    assert.equal(style['padding-bottom'], '0')
    assert.equal(style['line-height'], 'normal')
    for (const width of [320, 390]) {
      const height = Math.max(pixels(style.height, width), pixels(style['min-height'], width))
      assert.ok(height >= 44, '真实触控高度不足44px')
      assert.ok(height >= pixels(style['font-size'] || '26.9231rpx', width) * 1.3, '原生文字区被挤压')
    }
  }
})

test('单行修复不移除多行textarea内边距或带图标选择器的Flex布局', () => {
  for (const insideSection of [false, true]) {
    const ancestry = insideSection ? ['view.form-section'] : []
    const textarea = computedDeclarations(globalStyles, [...ancestry, 'textarea.input'])
    assert.ok(pixels(textarea['padding-top'], 390) > 0)
    assert.ok(pixels(textarea['padding-bottom'], 390) > 0)
    assert.ok(pixels(textarea.height, 390) >= 100)
    assert.equal(computedDeclarations(globalStyles, [...ancestry, 'view.input']).display, 'flex')
  }
})

test('搜索、金额、聊天、昵称和色值专用输入保留字号且不叠加垂直padding或固定行高', () => {
  const inputs = [
    ['components/FundsForm.vue', 'funds-amount-input'], ['components/OrderAdjustments.vue', 'adjustment-amount-input'],
    ['components/GameSelector.vue', 'catalog-input'], ['components/ManageList.vue', 'manage-search-input'],
    ['pages/manage/games.vue', 'color-value'], ['pages/orders/detail.vue', 'composer-input'],
    ['pages/account/login.vue', 'offer-name-input'], ['pages/services/publish.vue', 'color-code'],
    ['pages/main/player/player.vue', 'search-input'], ['pages/community/clubs/index.vue', 'search-input'],
    ['pages/manage/finance/settings.vue', 'finance-input'],
  ]
  for (const [path, name] of inputs) {
    const style = computedDeclarations(globalStyles + '\n' + localStyles(path), ['input.' + name])
    assert.equal(pixels(style['padding-top'], 390), 0, path + ' 顶部padding')
    assert.equal(pixels(style['padding-bottom'], 390), 0, path + ' 底部padding')
    assert.ok(!style['line-height'] || style['line-height'] === 'normal', path + ' 不应固定原生行高')
    for (const width of [320, 390]) assert.ok(pixels(style.height, width) >= pixels(style['font-size'], width) * 1.2, path + ' 文字高度不足')
    if (name.includes('amount-input')) assert.ok(pixels(style['font-size'], 390) >= 29, '金额仍保持设计大字号')
  }
})

test('业务焦点样式不得改变同页控件的高度、内边距或文字几何', () => {
  const paths = readdirSync(new URL('../', import.meta.url), { recursive: true, withFileTypes: true }).filter(entry => entry.isFile() && /\.vue$/.test(entry.name))
  for (const entry of paths) {
    const source = readFileSync(entry.parentPath + '/' + entry.name, 'utf8')
    for (const match of source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) postcss.parse(match[1]).walkRules((rule: Rule) => {
      if (!/:focus(?:-within|-visible)?\b/.test(rule.selector)) return
      rule.walkDecls(declaration => assert.doesNotMatch(declaration.prop, /^(?:height|min-height|max-height|padding(?:-.+)?|line-height|font-size|transform|top|bottom|margin(?:-.+)?)$/, entry.name + ' 聚焦改变文字几何'))
    })
  }
})
