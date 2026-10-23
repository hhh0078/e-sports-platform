/** 文件作用：验证消息本机缓存的账号隔离、全文保留、已读合并及持久化失败保护。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { messageCacheKey, readMessageCache, writeMessageCache, mergeMessageCache, markCachedMessageRead } from './messages-cache'
import type { MessageStorage } from './messages-cache'
import type { CachedMessage, SystemMessage } from '../types/messages'

const message: SystemMessage = { id: 'message-1', title: '审核结果', body: '第一段正文\n\n第二段正文', created_at: '2026-10-22T08:00:00Z' }
/** 提供可模拟异常与静默写入失败的同步存储边界。 */
function storageHarness() {
  const values = new Map<string, unknown>(), control = { fail: false, silent: false }
  const storage: MessageStorage = { read: key => values.get(key), write: (key, value) => { if (control.fail) throw new Error('quota exceeded'); if (!control.silent) values.set(key, value) } }
  return { values, control, storage }
}

test('账号使用独立缓存键，退出并不删除历史且空身份不能读取', () => {
  const subject = storageHarness(), items = mergeMessageCache('A', [], [message])
  writeMessageCache('A', items, subject.storage)
  assert.equal(readMessageCache('A', subject.storage)[0].body, message.body); assert.deepEqual(readMessageCache('B', subject.storage), []); assert.throws(() => messageCacheKey(''), /登录/)
  assert.notEqual(messageCacheKey('A:B'), messageCacheKey('A%3AB'))
})

test('首次收取仅缓存未读，读取详情才生成待确认，已确认的已读不重复标记', () => {
  const unread = mergeMessageCache('A', [], [message]); assert.equal(unread[0].read, false); assert.equal(unread[0].ack_pending, false)
  const read = markCachedMessageRead(unread, message.id); assert.equal(read[0].read, true); assert.equal(read[0].ack_pending, true); assert.equal(unread[0].read, false)
  read[0].ack_pending = false; assert.equal(markCachedMessageRead(read, message.id)[0].ack_pending, false)
})

test('服务端重送不能覆盖本机全文和已读，已读重送只重新等待确认', () => {
  const cached: CachedMessage = { ...message, read: true, ack_pending: false }
  const result = mergeMessageCache('A', [cached], [{ ...message, title: '旧服务器标题', body: '旧服务器正文' }])
  assert.equal(result.length, 1); assert.equal(result[0].title, message.title); assert.equal(result[0].body, message.body); assert.equal(result[0].read, true); assert.equal(result[0].ack_pending, true)
})

test('损坏缓存、跨账号数据和未读待删除状态均拒绝使用，不静默覆盖历史', () => {
  const subject = storageHarness(), key = messageCacheKey('A')
  for (const value of ['{broken', { version: 2, user_id: 'A', items: [] }, { version: 1, user_id: 'B', items: [] }, { version: 1, user_id: 'A', items: [{ ...message, read: false, ack_pending: true }] }]) {
    subject.values.set(key, value); assert.throws(() => readMessageCache('A', subject.storage), /消息缓存/); assert.equal(subject.values.get(key), value)
  }
  assert.throws(() => mergeMessageCache('A', [], [{ ...message, user_id: 'B' }]), /账号归属/)
})

test('写入异常与静默失败都不能通过持久化核验', () => {
  const subject = storageHarness(), items = mergeMessageCache('A', [], [message]); writeMessageCache('A', items, subject.storage)
  const original = subject.values.get(messageCacheKey('A')), read = markCachedMessageRead(items, message.id)
  subject.control.fail = true; assert.throws(() => writeMessageCache('A', read, subject.storage), /保存失败/); assert.equal(subject.values.get(messageCacheKey('A')), original)
  subject.control.fail = false; subject.control.silent = true; assert.throws(() => writeMessageCache('A', read, subject.storage), /保存失败/); assert.equal(readMessageCache('A', subject.storage)[0].read, false)
})

test('缓存只保存声明消息字段，不把外部额外配置复制进本机', () => {
  const subject = storageHarness(), incoming = { ...message, unexpected: 'not-a-message-field' }
  writeMessageCache('A', mergeMessageCache('A', [], [incoming]), subject.storage)
  assert.equal('unexpected' in readMessageCache('A', subject.storage)[0], false)
})


test('审核消息的可选行动和字段关联经已读落盘保留，旧消息不强制新增字段',()=>{
  const subject=storageHarness(),related={...message,action_path:'/pages/services/publish?id=service-A',action_label:'修改服务',target_type:'services',target_id:'service-A',field_label:'服务标题'}
  const unread=mergeMessageCache('A',[],[related]);writeMessageCache('A',markCachedMessageRead(unread,message.id),subject.storage)
  const local=readMessageCache('A',subject.storage)[0]
  assert.equal(local.read,true);assert.equal(local.ack_pending,true);assert.equal(local.action_path,related.action_path);assert.equal(local.action_label,related.action_label);assert.equal(local.target_type,'services');assert.equal(local.target_id,'service-A');assert.equal(local.field_label,'服务标题');assert.equal(local.body,message.body)
  const resent=mergeMessageCache('A',[local],[{...message,action_path:'/pages/orders/detail?id=old'}]);assert.equal(resent[0].action_path,related.action_path,'旧重送不能覆盖本机行动和全文')
  writeMessageCache('B',mergeMessageCache('B',[],[message]),subject.storage);assert.equal(readMessageCache('B',subject.storage)[0].action_path,undefined)
})
