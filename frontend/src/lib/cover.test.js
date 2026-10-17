/** 文件作用：验证六标签极端布局、混合锚点、空值和长度限制，防止封面文字重叠。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { layoutLabels, POSITIONS, newLabel } from './cover.js'

/** 验证输出标签在边界内且任意两项不重叠。 */
function assertLayout(labels) {
  const result = layoutLabels(labels)
  for (let index = 0; index < result.length; index++) {
    const item = result[index]
    assert.ok(item.left >= 0 && item.top >= 0)
    assert.ok(item.left + item.width <= 100.001 && item.top + item.height <= 100.001)
    for (const other of result.slice(index + 1)) {
      assert.ok(item.left + item.width <= other.left || other.left + other.width <= item.left || item.top + item.height <= other.top || other.top + other.height <= item.top, '文案不能重叠')
    }
  }
  return result
}

test('六条最长最大字号文案在全部九宫格锚点不越界不重叠', () => {
  for (const [position] of POSITIONS) {
    assert.equal(assertLayout(Array.from({ length: 6 }, (_, id) => ({ id, text: '三角洲护航任务陪练一起玩游戏', position, font_size: 24 }))).length, 6)
  }
})
test('混合位置、字号与英文文本仍能安全布局', () => {
  for (let seed = 0; seed < 100; seed++) {
    assertLayout(Array.from({ length: 6 }, (_, index) => ({ text: index % 2 ? '轻松找队友' : 'WWWWWWWWWWWW', position: POSITIONS[(seed + index) % 9][0], font_size: 12 + (seed * 7 + index) % 13 })))
  }
})
test('默认左下、无数据和超量标签处理', () => {
  assert.deepEqual(layoutLabels(null), [])
  assert.equal(layoutLabels([{ text: '极速接单' }])[0].left, 8 / 360 * 100)
  assert.equal(layoutLabels(Array.from({ length: 10 }, () => ({ text: '1' }))).length, 6)
  assert.equal([...layoutLabels([{ text: '123456789012345' }])[0].text].length, 12)
})
test('浅色封面标签采用深色字，深色标签采用白字', () => {
  assert.equal(newLabel('#E6C76D').color, '#101714')
  assert.equal(newLabel('#00e18a').color, '#101714')
  assert.equal(newLabel('#07130f').color, '#ffffff')
})
