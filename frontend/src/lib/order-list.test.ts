/** 文件作用：验证订单深链归一化、登录返回及买卖操作文案，防止入口与身份语义错误。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { parseOrderQuery, orderListPath, orderActionLabel } from './order-list.ts'
test('保留有效订单深链，拒绝未知身份与筛选', () => {
  assert.deepEqual(parseOrderQuery({ role: 'provider', status: 'working' }), { role: 'provider', filter: 'working' })
  assert.deepEqual(parseOrderQuery({ role: 'all', status: 'completed' }), { role: 'all', filter: 'completed' })
  assert.deepEqual(parseOrderQuery({ role: 'admin&all=1', status: 'invalid' }), { role: 'buyer', filter: 'all' })
  assert.equal(orderListPath('provider', 'refund'), '/pages/main/index?tab=orders&role=provider&status=refund')
})
test('付款入口只向本人买方显示，售后与履约优先级明确', () => {
  const order = { buyer_id: 'buyer', provider_id: 'worker', status: 'pending_payment', refund_status: 'none' }
  assert.equal(orderActionLabel(order, 'buyer'), '继续支付')
  assert.equal(orderActionLabel(order, 'worker'), '等待买家支付')
  assert.equal(orderActionLabel({ ...order, status: 'accepted' }, 'worker'), '订单沟通')
  assert.equal(orderActionLabel({ ...order, status: 'completed', refund_status: 'requested' }, 'worker'), '查看售后')
  assert.equal(orderActionLabel({ ...order, status: 'refunded', refund_status: 'approved' }, 'buyer'), '查看订单')
})
