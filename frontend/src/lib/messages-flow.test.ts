/** 文件作用：执行消息页面及缓存真实脚本，验证未读不删、已读持久后确认、失败重试和跨账号竞态。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { ref, reactive, computed, watch, effectScope } from 'vue'
import { executable, injectBusinessAPIs, pageScript } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'
import type { CachedMessage, SystemMessage } from '../types/messages'

const sources = ['store.ts', 'messages-cache.ts', 'use-messages.ts'].map(name => readFileSync(new URL('./' + name, import.meta.url), 'utf8'))
const message: SystemMessage = { id: 'message-1', title: '订单进度更新', body: '完整通知第一段\n\n完整通知第二段', created_at: '2026-10-22T08:00:00Z', user_id: 'A' }
/** 以实际SFC、缓存、store和领域API搭建环境，只替换网络及官方同步存储边界。 */
function harness(kind: 'list' | 'detail' = 'list', loggedIn = true) {
  const storage = new Map<string, unknown>(loggedIn ? [['jijie_token', 'token-A']] : []), requests: DeferredRequest[] = [], navigations: string[] = [], control = { fail: false, silent: false }, scope = effectScope()
  const lifecycle: PageLifecycle = { load: () => {}, show: () => {}, hide: () => {}, unload: () => {} }
  const context = vm.createContext({ ref, reactive, computed, watch,
    uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: unknown) => { if (key.startsWith('jijie_messages_v1:') && control.fail) throw new Error('quota exceeded'); if (!key.startsWith('jijie_messages_v1:') || !control.silent) storage.set(key, value) }, removeStorageSync: (key: string) => storage.delete(key) },
    token: () => String(storage.get('jijie_token') || ''), go: (value: string) => navigations.push(value),
    /** 记录真实领域请求，不复制消息同步算法。 */
    api(path: string, method = 'GET', body: Record<string, unknown> = {}) { return new Promise<unknown>((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) },
    onLoad: (callback: PageLifecycle['load']) => { lifecycle.load = callback }, onShow: (callback: () => void) => { lifecycle.show = callback }, onHide: (callback: () => void) => { lifecycle.hide = callback }, onUnload: (callback: () => void) => { lifecycle.unload = callback },
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(sources[0]) + '\nglobalThis.store={state,login,clearSession}', context)
  context.store.state.user = loggedIn ? { id: 'A', nickname: '用户A', role: 'user' } : null; context.store.state.ready = true
  const exposed = kind === 'list' ? 'items,loading,error,pendingCount,showPage,retryPending,openMessage' : 'selected,cacheOwner,loading,error,showPage,retryPending,id'
  scope.run(() => vm.runInContext(executable(sources[1]) + '\n' + executable(sources[2]) + '\n' + executable(pageScript('account/messages/' + (kind === 'list' ? 'index' : 'detail') + '.vue')) + '\nglobalThis.page={' + exposed + '};globalThis.cache={readMessageCache,writeMessageCache,mergeMessageCache,messageCacheKey}', context))
  if (kind === 'detail') lifecycle.load({ id: message.id })
  return { page: context.page, store: context.store, context, cache: context.cache, storage, requests, navigations, control, lifecycle, scope,
    /** 查找最新未消费的目标请求，账号轮换时更早请求仍可单独验证。 */
    take(path: string, method = 'GET') { const request = [...requests].reverse().find(item => !item.taken && item.path === path && item.method === method); assert.ok(request, `缺少请求 ${method} ${path}`); request.taken = true; return request },
    /** 使用真实缓存写入建立已存在的本机历史。 */
    seed(owner: string, items: CachedMessage[]) { context.cache.writeMessageCache(owner, items) },
  }
}
/** 让真实异步函数推进到下一受控网络边界。 */
async function flush(): Promise<void> { await new Promise(resolve => setImmediate(resolve)) }
/** 经真实store登录方法轮换账号、令牌及会话修订号。 */
async function loginB(subject: ReturnType<typeof harness>): Promise<void> { const pending = subject.store.login('B'); subject.take('/auth/dev', 'POST').resolve({ token: 'token-B', user: { id: 'B', nickname: '用户B', role: 'user' } }); await pending; await flush() }

test('GET只缓存全文和未读，绝不DELETE；列表跳转沿用本机详情路由', async () => {
  const subject = harness(), showing = subject.page.showPage(); await flush(); subject.take('/messages?limit=100&offset=0').resolve({ items: [message], total: 1 }); await showing
  const stored = subject.cache.readMessageCache('A')[0]; assert.equal(stored.body, message.body); assert.equal(stored.read, false); assert.equal(stored.ack_pending, false); assert.equal(subject.requests.filter(item => item.method === 'DELETE').length, 0)
  subject.page.openMessage(message.id); assert.equal(subject.navigations[0], '/pages/account/messages/detail?id=message-1'); subject.scope.stop()
})

test('收完全部分页并落盘后才确认已读，未读在服务器保留', async () => {
  const subject = harness(); subject.seed('A', [{ ...message, read: true, ack_pending: true }])
  const showing = subject.page.showPage(); await flush(); const first = [message, ...Array.from({ length: 99 }, (_, index) => ({ ...message, id: 'unread-' + index }))]
  subject.take('/messages?limit=100&offset=0').resolve({ items: first, total: 101 }); await flush(); assert.equal(subject.requests.filter(item => item.method === 'DELETE').length, 0)
  subject.take('/messages?limit=100&offset=100').resolve({ items: [{ ...message, id: 'last-unread' }], total: 101 }); await flush(); assert.equal(subject.cache.readMessageCache('A').length, 101)
  subject.take('/messages/message-1', 'DELETE').resolve({ ok: true }); await showing
  assert.equal(subject.requests.filter(item => item.method === 'DELETE').length, 1); assert.equal(subject.cache.readMessageCache('A').filter((item: CachedMessage) => !item.read).length, 100); subject.scope.stop()
})

test('点击本机详情先持久全文和已读再DELETE，成功后本机全文继续保留', async () => {
  const subject = harness('detail'); subject.seed('A', [{ ...message, read: false, ack_pending: false }]); const showing = subject.page.showPage(); await flush()
  const request = subject.take('/messages/message-1', 'DELETE'), before = subject.cache.readMessageCache('A')[0]
  assert.equal(before.read, true); assert.equal(before.ack_pending, true); assert.equal(before.body, message.body); assert.equal(subject.page.selected.value.body, message.body); assert.equal(subject.requests.some(item => item.method === 'GET'), false)
  request.resolve({ ok: true }); await showing; const after = subject.cache.readMessageCache('A')[0]; assert.equal(after.read, true); assert.equal(after.ack_pending, false); assert.equal(after.body, message.body); subject.scope.stop()
})

test('已读保存异常或静默失败都不能删除远端，正文仍能本机阅读', async t => {
  for (const mode of ['fail', 'silent'] as const) await t.test(mode, async () => {
    const subject = harness('detail'); subject.seed('A', [{ ...message, read: false, ack_pending: false }]); subject.control[mode] = true; await subject.page.showPage()
    assert.equal(subject.requests.length, 0); assert.equal(subject.cache.readMessageCache('A')[0].read, false); assert.equal(subject.page.selected.value.body, message.body); assert.match(subject.page.error.value, /保存失败/); subject.scope.stop()
  })
})

test('已读确认失败保留pending，本机重试不依赖远端正文', async () => {
  const subject = harness('detail'); subject.seed('A', [{ ...message, read: false, ack_pending: false }]); const showing = subject.page.showPage(); await flush(); subject.take('/messages/message-1', 'DELETE').reject(new Error('网络暂不可用')); await showing
  assert.equal(subject.cache.readMessageCache('A')[0].read, true); assert.equal(subject.cache.readMessageCache('A')[0].ack_pending, true); assert.match(subject.page.error.value, /待重试/)
  const retry = subject.page.retryPending(); subject.take('/messages/message-1', 'DELETE').resolve({ ok: true }); await retry
  assert.equal(subject.cache.readMessageCache('A')[0].ack_pending, false); assert.equal(subject.page.selected.value.body, message.body); assert.equal(subject.requests.some(item => item.method === 'GET'), false); subject.scope.stop()
})

test('已确认的本机已读详情再次打开无需GET或DELETE', async () => {
  const subject = harness('detail'); subject.seed('A', [{ ...message, read: true, ack_pending: false }]); await subject.page.showPage()
  assert.equal(subject.page.selected.value.body, message.body); assert.equal(subject.requests.length, 0); subject.scope.stop()
})

test('服务器旧条目不会覆盖本机已读或全文，只对已读重试确认', async () => {
  const subject = harness(); subject.seed('A', [{ ...message, read: true, ack_pending: false }]); const showing = subject.page.showPage(); await flush(); subject.take('/messages?limit=100&offset=0').resolve({ items: [{ ...message, title: '旧标题', body: '旧正文' }], total: 1 }); await flush(); const ack = subject.take('/messages/message-1', 'DELETE')
  assert.equal(subject.page.items.value[0].read, true); assert.equal(subject.page.items.value[0].body, message.body); ack.resolve({ ok: true }); await showing; subject.scope.stop()
})

test('跨账号迟到GET不写旧缓存也不显示在新账号', async () => {
  const subject = harness(), showing = subject.page.showPage(); await flush(); const old = subject.take('/messages?limit=100&offset=0'); await loginB(subject)
  subject.take('/messages?limit=100&offset=0').resolve({ items: [{ ...message, id: 'B-message', user_id: 'B', body: 'B自己的正文' }], total: 1 }); await flush(); old.resolve({ items: [message], total: 1 }); await showing
  assert.equal(subject.page.items.value[0].id, 'B-message'); assert.equal(subject.cache.readMessageCache('A').length, 0); assert.equal(subject.cache.readMessageCache('B')[0].body, 'B自己的正文'); subject.scope.stop()
})

test('跨账号迟到DELETE不能修改新账号同ID消息的待确认状态', async () => {
  const subject = harness('detail'); subject.seed('A', [{ ...message, read: true, ack_pending: true }]); subject.seed('B', [{ ...message, user_id: 'B', body: 'B的同ID正文', read: true, ack_pending: true }])
  const showing = subject.page.showPage(); await flush(); const old = subject.take('/messages/message-1', 'DELETE'); await loginB(subject); const current = subject.take('/messages/message-1', 'DELETE')
  old.resolve({ ok: true }); await showing; assert.equal(subject.cache.readMessageCache('B')[0].ack_pending, true); assert.equal(subject.page.selected.value.body, 'B的同ID正文')
  current.resolve({ ok: true }); await flush(); assert.equal(subject.cache.readMessageCache('B')[0].ack_pending, false); assert.equal(subject.cache.readMessageCache('A')[0].ack_pending, true); subject.scope.stop()
})

test('登出保留原账号缓存，访客页面不展示或请求任何账号消息', async () => {
  const subject = harness(); subject.seed('A', [{ ...message, read: false, ack_pending: false }]); const showing = subject.page.showPage(); await flush(); subject.take('/messages?limit=100&offset=0').resolve({ items: [], total: 0 }); await showing
  subject.store.clearSession(); await flush(); assert.equal(subject.page.items.value.length, 0); assert.equal(subject.cache.readMessageCache('A')[0].body, message.body); const count = subject.requests.length; await subject.page.showPage(); assert.equal(subject.requests.length, count); subject.scope.stop()
})

test('拉取失败保留本机历史；损坏缓存和远端归属异常均不确认删除', async t => {
  await t.test('网络失败', async () => { const subject = harness(); subject.seed('A', [{ ...message, read: false, ack_pending: false }]); const showing = subject.page.showPage(); await flush(); subject.take('/messages?limit=100&offset=0').reject(new Error('读取失败')); await showing; assert.equal(subject.page.items.value[0].body, message.body); assert.match(subject.page.error.value, /读取失败/); assert.equal(subject.requests.some(item => item.method === 'DELETE'), false); subject.scope.stop() })
  await t.test('损坏缓存', async () => { const subject = harness(); subject.storage.set(subject.cache.messageCacheKey('A'), '{broken'); await subject.page.showPage(); assert.equal(subject.requests.length, 0); assert.match(subject.page.error.value, /缓存无法读取/); subject.scope.stop() })
  await t.test('远端账号异常', async () => { const subject = harness(); const showing = subject.page.showPage(); await flush(); subject.take('/messages?limit=100&offset=0').resolve({ items: [{ ...message, user_id: 'B' }], total: 1 }); await showing; assert.equal(subject.requests.some(item => item.method === 'DELETE'), false); assert.equal(subject.cache.readMessageCache('A').length, 0); assert.match(subject.page.error.value, /账号归属/); subject.scope.stop() })
})

test('详情缺失只显示本机空态，不按ID请求远端或删除其他待确认消息', async () => {
  const subject = harness('detail'); subject.seed('A', [{ ...message, id: 'other', read: true, ack_pending: true }]); await subject.page.showPage()
  assert.equal(subject.page.selected.value, null); assert.equal(subject.requests.length, 0); subject.scope.stop()
})

test('页面隐藏后的迟到消息不写入缓存，新显示可重新接收', async () => {
  const subject = harness(), first = subject.page.showPage(); await flush(); const old = subject.take('/messages?limit=100&offset=0'); subject.lifecycle.hide(); old.resolve({ items: [message], total: 1 }); await first; assert.equal(subject.cache.readMessageCache('A').length, 0)
  const retry = subject.page.showPage(); await flush(); subject.take('/messages?limit=100&offset=0').resolve({ items: [message], total: 1 }); await retry; assert.equal(subject.page.items.value.length, 1); subject.scope.stop()
})

test('消息领域注入不覆盖订单聊天室的同类方法', async () => {
  const subject = harness(); const pending = vm.runInContext('listMessages("order-A")', subject.context); subject.take('/orders/order-A/messages').resolve([]); await pending; subject.scope.stop()
})

test('DELETE成功但确认标记写入失败时，重启可安全重放幂等确认且全文仍在', async () => {
  const subject = harness('detail'); subject.seed('A', [{ ...message, read: false, ack_pending: false }]); const showing = subject.page.showPage(); await flush(); const request = subject.take('/messages/message-1', 'DELETE')
  subject.control.fail = true; request.resolve({ ok: true }); await showing
  const original = subject.cache.readMessageCache('A')[0]; assert.equal(original.read, true); assert.equal(original.ack_pending, true); assert.equal(original.body, message.body); assert.match(subject.page.error.value, /保存失败/)
  const restarted = harness('detail'); restarted.storage.set(restarted.cache.messageCacheKey('A'), subject.storage.get(subject.cache.messageCacheKey('A'))); const reloading = restarted.page.showPage(); await flush(); restarted.take('/messages/message-1', 'DELETE').resolve({ ok: true }); await reloading
  assert.equal(restarted.cache.readMessageCache('A')[0].ack_pending, false); assert.equal(restarted.page.selected.value.body, message.body); assert.equal(restarted.requests.some(item => item.method === 'GET'), false); subject.scope.stop(); restarted.scope.stop()
})

test('第二页返回前切换账号，旧分页不写新账号也不确认旧已读', async () => {
  const subject = harness(); subject.seed('A', [{ ...message, read: true, ack_pending: true }]); const showing = subject.page.showPage(); await flush()
  subject.take('/messages?limit=100&offset=0').resolve({ items: Array.from({ length: 100 }, (_, index) => ({ ...message, id: index === 0 ? message.id : 'A-' + index })), total: 101 }); await flush(); const oldPage = subject.take('/messages?limit=100&offset=100')
  await loginB(subject); subject.take('/messages?limit=100&offset=0').resolve({ items: [{ ...message, id: 'B-only', user_id: 'B' }], total: 1 }); await flush(); oldPage.resolve({ items: [{ ...message, id: 'A-stale-last' }], total: 101 }); await showing
  assert.equal(subject.page.items.value[0].id, 'B-only'); assert.equal(subject.cache.readMessageCache('A').length, 100); assert.equal(subject.cache.readMessageCache('B').length, 1); assert.equal(subject.requests.some(item => item.method === 'DELETE'), false); subject.scope.stop()
})

test('连续刷新让旧operation失效，迟到成功不能覆盖最新本机列表', async () => {
  const subject = harness(), first = subject.page.showPage(); await flush(); const old = subject.take('/messages?limit=100&offset=0')
  const second = subject.page.showPage(); await flush(); subject.take('/messages?limit=100&offset=0').resolve({ items: [{ ...message, id: 'latest' }], total: 1 }); await second
  old.resolve({ items: [{ ...message, id: 'stale' }], total: 1 }); await first
  assert.deepEqual(Array.from(subject.page.items.value, (item: CachedMessage) => item.id), ['latest']); assert.equal(subject.cache.readMessageCache('A').some((item: CachedMessage) => item.id === 'stale'), false); subject.scope.stop()
})
