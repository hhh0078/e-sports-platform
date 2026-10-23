/** 文件作用：验证响应式设计换算保持390px基线，并按320px设备比例缩放及拒绝异常尺寸。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { designRpx, designRpxValue, designPixels } from './design-units'

test('390px设计图统一映射750rpx且保留零值和负偏移', () => {
  assert.equal(designRpx(390), '750rpx')
  assert.equal(designRpx(0), '0rpx')
  assert.equal(designRpx(-13), '-25rpx')
  assert.ok(Math.abs(designRpxValue(16) * 390 / 750 - 16) < 0.001)
})

test('真实像素接口只按实际视口换算一次', () => {
  assert.equal(designPixels(44, 390), 44)
  assert.ok(Math.abs(designPixels(44, 320) - 36.1025641026) < 0.001)
  assert.equal(designPixels(0, 320), 0)
})

test('非法尺寸不能生成NaN样式或无效绘制坐标', () => {
  assert.throws(() => designRpx(Number.NaN), /有限/)
  assert.throws(() => designPixels(12, 0), /无效/)
  assert.throws(() => designPixels(Infinity, 390), /无效/)
})
