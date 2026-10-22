/** 文件作用：执行真实管理列表组合函数，验证分页偏移、旧搜索和跨管理员回调不会污染当前页。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { ref, shallowRef, reactive, computed, watch, effectScope } from 'vue'
import { executable, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'

const resourceSource = readFileSync(new URL('./manage-resource.ts', import.meta.url), 'utf8')
const storeSource = readFileSync(new URL('./store.ts', import.meta.url), 'utf8')
/** 建立可控制真实请求完成顺序的列表环境。 */
function harness(role = 'admin') {
  const storage = new Map([['jijie_token', 'token-A']]), requests: DeferredRequest[] = [], notices: unknown[] = [], scope = effectScope()
  const lifecycle: PageLifecycle = { load: () => {}, show: () => {}, hide: () => {}, unload: () => {} }
  const context = vm.createContext({ ref, shallowRef, reactive, computed, watch, params: {}, cleared: 0, preserved: false,
    uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: string) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key) },
    token: () => storage.get('jijie_token') || '', notify: (value: unknown) => notices.push(value),
    /** 只模拟网络边界，列表和审核逻辑使用生产实现。 */
    api(path: string, method = 'GET', body: Record<string, unknown> = {}) { return new Promise<unknown>((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) },
    onShow: (callback: () => void) => { lifecycle.show = callback }, onHide: (callback: () => void) => { lifecycle.hide = callback }, onUnload: (callback: () => void) => { lifecycle.unload = callback },
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource) + '\nglobalThis.store = { state, login, sessionRevision }', context)
  context.store.state.user = { id: 'A', nickname: '管理员A', role }; context.store.state.ready = true
  scope.run(() => vm.runInContext(executable(resourceSource) + '\nglobalThis.list = useManageResource("users", { params: () => params, preserveDraft: () => preserved, clearDraft: () => { cleared++ } })', context))
  return { list: context.list, store: context.store, context, requests, notices, lifecycle, scope,
    /** 获取真实发出的目标请求，不容许重复消费。 */
    take(path: string, method = 'GET') { const request = [...requests].reverse().find(item => !item.taken && item.path === path && item.method === method); assert.ok(request, `缺少请求 ${method} ${path}`); request.taken = true; return request },
  }
}
/** 推进到下一个明确的异步边界。 */
async function flush(): Promise<void> { await new Promise(resolve => setImmediate(resolve)) }
/** 使用实际登录实现轮换账号及令牌。 */
async function loginB(subject: ReturnType<typeof harness>): Promise<void> { const pending = subject.store.login('B'); subject.take('/auth/dev', 'POST').resolve({ token: 'token-B', user: { id: 'B', role: 'admin', nickname: '管理员B' } }); await pending; await flush() }

test('非管理员不请求管理资源，也不沿用旧列表', async () => {
  const subject = harness('user'); subject.list.items.value = [{ id: 'private' }]
  await subject.list.showPage()
  assert.equal(subject.requests.length, 0); assert.equal(subject.list.items.value.length, 0); assert.ok(subject.context.cleared > 0); subject.scope.stop()
})

test('更早搜索的成功或失败均不能替换最新搜索', async () => {
  const subject = harness(), showing = subject.list.showPage(); await flush(); const initial = subject.take('/admin/users?limit=20&offset=0')
  subject.list.query.value = '新关键词'; const searching = subject.list.loadData(), latest = subject.take('/admin/users?limit=20&offset=0&q=%E6%96%B0%E5%85%B3%E9%94%AE%E8%AF%8D')
  latest.resolve([{ id: 'new' }]); await searching; initial.reject(new Error('旧请求失败')); await showing
  assert.deepEqual(subject.list.items.value.map((item: { id: string }) => item.id), ['new']); assert.equal(subject.list.error.value, ''); subject.scope.stop()
})

test('分页按服务器原始记录推进偏移，重复记录只展示一次', async () => {
  const subject = harness(), showing = subject.list.showPage(); await flush()
  subject.take('/admin/users?limit=20&offset=0').resolve(Array.from({ length: 20 }, (_, index) => ({ id: String(index) }))); await showing
  const second = subject.list.loadData(true); subject.take('/admin/users?limit=20&offset=20').resolve(Array.from({ length: 20 }, (_, index) => ({ id: String(index + 19) }))); await second
  assert.equal(subject.list.items.value.length, 39)
  const third = subject.list.loadData(true); subject.take('/admin/users?limit=20&offset=40').resolve([]); await third
  assert.equal(subject.list.hasMore.value, false); subject.scope.stop()
})

test('账号切换立即清除列表，旧账号迟到响应不进入新页', async () => {
  const subject = harness(), showing = subject.list.showPage(); await flush(); const old = subject.take('/admin/users?limit=20&offset=0')
  subject.list.items.value = [{ id: 'private-A' }]; await loginB(subject)
  assert.equal(subject.list.items.value.length, 0)
  const current = subject.take('/admin/users?limit=20&offset=0'); current.resolve([{ id: 'B-visible' }]); await flush(); old.resolve([{ id: 'A-stale' }]); await showing
  assert.deepEqual(subject.list.items.value.map((item: { id: string }) => item.id), ['B-visible']); assert.equal(subject.notices.length, 0); subject.scope.stop()
})

test('审核请求重复点击仅提交一次，切换账号后不提示成功或额外刷新', async () => {
  const subject = harness(), showing = subject.list.showPage(); await flush(); subject.take('/admin/users?limit=20&offset=0').resolve([]); await showing
  const action = () => vm.runInContext('moderateResource("users", "target", { status: "frozen" })', subject.context)
  const saving = subject.list.mutate(action); await subject.list.mutate(action); const old = subject.take('/admin/users/target', 'PATCH')
  assert.equal(subject.requests.filter(item => item.method === 'PATCH').length, 1)
  await loginB(subject); subject.take('/admin/users?limit=20&offset=0').resolve([]); await flush()
  const count = subject.requests.length; old.resolve({ id: 'target' }); await saving
  assert.equal(subject.requests.length, count); assert.equal(subject.notices.length, 0); assert.equal(subject.list.busy.value, false); subject.scope.stop()
})

test('隐藏清除管理数据且拒绝旧读取，原生选图只保留草稿', async () => {
  const subject = harness(), showing = subject.list.showPage(); await flush(); const request = subject.take('/admin/users?limit=20&offset=0')
  subject.context.preserved = true; const clears = subject.context.cleared; subject.lifecycle.hide(); request.resolve([{ id: 'hidden' }]); await showing
  assert.equal(subject.list.items.value.length, 0); assert.equal(subject.context.cleared, clears)
  subject.lifecycle.unload(); assert.ok(subject.context.cleared > clears); subject.scope.stop()
})

test('服务端读取失败保留可重试状态，审核成功通过真实接口刷新', async () => {
  const subject = harness(), showing = subject.list.showPage(); await flush(); subject.take('/admin/users?limit=20&offset=0').reject(new Error('管理数据暂不可用')); await showing
  assert.equal(subject.list.error.value, '管理数据暂不可用'); assert.equal(subject.list.loading.value, false)
  const retry = subject.list.loadData(); subject.take('/admin/users?limit=20&offset=0').resolve([{ id: 'target', status: 'active' }]); await retry
  assert.equal(subject.list.error.value, '')
  const saving = subject.list.mutate(() => vm.runInContext('moderateResource("users", "target", { status: "frozen" })', subject.context))
  subject.take('/admin/users/target', 'PATCH').resolve({ id: 'target', status: 'frozen' }); await flush()
  subject.take('/admin/users?limit=20&offset=0').resolve([{ id: 'target', status: 'frozen' }]); await saving
  assert.equal(subject.list.items.value[0].status, 'frozen'); assert.equal(subject.notices.at(-1), '处理结果已保存'); subject.scope.stop()
})

test('筛选条件改变后拒绝旧分页结果，不混入新状态', async () => {
  const subject = harness(); subject.context.params = { status: 'pending' }
  const showing = subject.list.showPage(); await flush(); const old = subject.take('/admin/users?status=pending&limit=20&offset=0')
  subject.context.params = { status: 'active' }; const filtered = subject.list.loadData(); subject.take('/admin/users?status=active&limit=20&offset=0').resolve([{ id: 'active' }]); await filtered
  old.resolve([{ id: 'pending' }]); await showing
  assert.deepEqual(subject.list.items.value.map((item: { id: string }) => item.id), ['active']); subject.scope.stop()
})

test('修改搜索词后点击更多从新条件首页读取，不混合两次查询', async () => {
  const subject = harness(), showing = subject.list.showPage(); await flush()
  subject.take('/admin/users?limit=20&offset=0').resolve(Array.from({ length: 20 }, (_, index) => ({ id: 'A-' + index }))); await showing
  subject.list.query.value = 'B'; const more = subject.list.loadData(true)
  subject.take('/admin/users?limit=20&offset=0&q=B').resolve([{ id: 'B-first-page' }]); await more
  assert.deepEqual(subject.list.items.value.map((item: { id: string }) => item.id), ['B-first-page']); assert.equal(subject.list.hasMore.value, false); subject.scope.stop()
})

test('冷启动配置读取失败可重新核验身份并加载列表，不停留在空白管理页', async () => {
  const subject = harness(); subject.store.state.user = null; subject.store.state.ready = false
  const first = subject.list.showPage(); subject.take('/bootstrap').reject(new Error('初始化网络失败')); await first
  assert.equal(subject.list.error.value, '初始化网络失败'); assert.equal(subject.store.state.ready, false)
  const retry = subject.list.showPage(); assert.equal(subject.list.error.value, '')
  subject.take('/bootstrap').resolve({ games: [], dev_mode: true }); await flush(); subject.take('/me').resolve({ id: 'A', role: 'admin', nickname: '管理员A' }); await flush()
  subject.take('/admin/users?limit=20&offset=0').resolve([{ id: 'recovered' }]); await retry; await flush()
  assert.equal(subject.store.state.ready, true); assert.equal(subject.store.state.user.role, 'admin'); assert.deepEqual(subject.list.items.value.map((item: { id: string }) => item.id), ['recovered']); assert.equal(subject.list.error.value, ''); subject.scope.stop()
})
