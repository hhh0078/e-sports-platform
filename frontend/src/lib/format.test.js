/** 文件作用：验证金额显示与异常时间不会破坏交易界面。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { money, time, statusLabel, assetUrl, go } from './format.js'

test('金额按分准确展示，保留有效小数', () => {
  assert.equal(money(0), '0')
  assert.equal(money(1), '0.01')
  assert.equal(money(1099), '10.99')
  assert.equal(money(1050), '10.5')
  assert.equal(money(1000000), '10000')
})

test('空值和无效时间安全显示，未知状态保留原值', () => {
  assert.equal(money(null), '0')
  assert.equal(time(null), '')
  assert.equal(time('not-a-date'), '')
  assert.equal(statusLabel('pending_payment'), '待支付')
  assert.equal(statusLabel('refunded'), '已退款')
  assert.equal(statusLabel('future_status'), 'future_status')
})

test('上传封面采用后端地址，内置图片保持小程序相对路径', () => {
  assert.equal(assetUrl('/api/media/cover123'), 'http://127.0.0.1:8090/api/media/cover123')
  assert.equal(assetUrl('/static/covers/delta-1.jpg'), 'http://127.0.0.1:8090/static/covers/delta-1.jpg')
  assert.equal(assetUrl(null), '')
})

test('主导航筛选保留参数并重置页面栈，详情保留返回且失败给出提示', () => {
  const calls = []
  globalThis.uni = { reLaunch: options => calls.push({ type: 'reLaunch', options }), navigateTo: options => calls.push({ type: 'navigateTo', options }), showToast: options => calls.push({ type: 'toast', options }) }
  go('/pages/orders/index?role=provider&status=working')
  assert.equal(calls[0].type, 'reLaunch')
  assert.equal(calls[0].options.url, '/pages/orders/index?role=provider&status=working')
  go('/pages/order/index?id=example')
  assert.equal(calls[1].type, 'navigateTo')
  calls[1].options.fail()
  assert.equal(calls[2].type, 'toast')
  go('/pages/me/index')
  assert.equal(calls[3].type, 'reLaunch')
  delete globalThis.uni
})
