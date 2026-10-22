/** 文件作用：验证资金页面精确费率、微额提现边界及不同组织原请求隔离。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { parseRate, formatRate, fundsIntentKey, paymentPurposeName } from './finance'
import { parseWalletAmount } from './wallet'

test('抽成使用整数基点且严格拒绝无效输入',()=>{
  assert.equal(parseRate('0'),0);assert.equal(parseRate('5.25'),525);assert.equal(parseRate('100.00'),10000)
  assert.equal(formatRate(0),'0%');assert.equal(formatRate(525),'5.25%');assert.equal(formatRate(1000),'10%')
  for(const value of ['-1','100.01','1e2','0.001','','NaN'])assert.throws(()=>parseRate(value))
})
test('小额提现只降低指定操作下限，充值继续遵守原边界',()=>{
  assert.equal(parseWalletAmount('0.01',20000000,1),1)
  assert.equal(parseWalletAmount('0.30',20000000,30),30)
  assert.throws(()=>parseWalletAmount('0.29',20000000,30))
  assert.throws(()=>parseWalletAmount('0',20000000,1))
  assert.throws(()=>parseWalletAmount('0.99'))
})
test('资金意图按账号组织用途和金额隔离，押金名称不混用',()=>{
  const original=fundsIntentKey('user','club-A','member_club:pay',100)
  assert.notEqual(original,fundsIntentKey('user','club-B','member_club:pay',100))
  assert.notEqual(original,fundsIntentKey('other','club-A','member_club:pay',100))
  assert.notEqual(original,fundsIntentKey('user','club-A','club_platform:pay',100))
  assert.equal(paymentPurposeName('member_club'),'成员俱乐部押金')
  assert.equal(paymentPurposeName('club_platform'),'俱乐部平台押金')
})
