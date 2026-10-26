/** 文件作用：验证rpx规范门禁能拦截设计px，并准确保留设备、细线和Canvas实际像素。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { fixedVisualPixels } from './responsive-style-policy'

test('普通间距字号和圆角的固定px必须被拦截', () => {
  assert.deepEqual(fixedVisualPixels('.card{padding:16px;font-size:14px;border-radius:1px}').map(item => item.property), ['padding', 'font-size', 'border-radius'])
  assert.equal(fixedVisualPixels('.card{padding:30.7692rpx;font-size:26.9231rpx}').length, 0)
})

test('安全区fallback不掩盖同一声明内新增的固定间距', () => {
  assert.equal(fixedVisualPixels('.hero{padding:calc(var(--status-bar-height,20px) + 16px)}').length, 1)
  assert.equal(fixedVisualPixels('.hero{padding:calc(var(--status-bar-height,20px) + 30.7692rpx)}').length, 0)
})

test('真实断点细线触控下限和桌面cap保留物理布局单位', () => {
  const css = '@media(max-width:350px){.sheet{max-width:600px;border:1px solid red;min-height:max(44px,84.6154rpx);min-width:44px}.divider{height:1px}}'
  assert.equal(fixedVisualPixels(css).length, 0)
  assert.equal(fixedVisualPixels('.button{min-width:45px;min-height:45px}').length, 2)
})

test('真实位图大小允许px，普通预览图仍必须响应式', () => {
  assert.equal(fixedVisualPixels('.poster-canvas{width:750px;height:1120px}.share-canvas{left:-10000px}').length, 0)
  assert.equal(fixedVisualPixels('.poster-preview{height:1120px}').length, 1)
})
