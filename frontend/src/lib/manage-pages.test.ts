/** 文件作用：执行独立管理页面真实脚本，验证安全深链、审核接口及用户冻结确认的生命周期边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { ref, shallowRef, reactive, computed, watch, effectScope } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'

const storeSource = readFileSync(new URL('./store.ts', import.meta.url), 'utf8'), resourceSource = readFileSync(new URL('./manage-resource.ts', import.meta.url), 'utf8')
/** 页面脚本测试支持的四个独立模块。 */
type Module = 'users' | 'services' | 'clubs' | 'orders'
/** 模拟官方确认弹框，不替换页面的确认和权限判断。 */
interface DeferredDialog { resolve: (value: { confirm: boolean; cancel?: boolean }) => void; reject: (error: unknown) => void }
/** 使用实际页面、共享列表及领域接口搭建受控环境。 */
function harness(module: Module, role = 'admin') {
  const storage = new Map([['jijie_token', 'token-A']]), requests: DeferredRequest[] = [], notices: unknown[] = [], dialogs: DeferredDialog[] = [], navigations: string[] = [], scope = effectScope()
  const lifecycle: PageLifecycle = { load: () => {}, show: () => {}, hide: () => {}, unload: () => {} }
  const context = vm.createContext({ ref, shallowRef, reactive, computed, watch,
    uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: string) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key), showModal: () => new Promise((resolve, reject) => dialogs.push({ resolve, reject })) },
    token: () => storage.get('jijie_token') || '', notify: (value: unknown) => notices.push(value), go: (value: string) => navigations.push(value),
    /** 生产领域接口的实际路径由测试记录，网络时序可独立控制。 */
    api(path: string, method = 'GET', body: Record<string, unknown> = {}) { return new Promise<unknown>((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) },
    onLoad: (callback: PageLifecycle['load']) => { lifecycle.load = callback }, onShow: (callback: () => void) => { lifecycle.show = callback }, onHide: (callback: () => void) => { lifecycle.hide = callback }, onUnload: (callback: () => void) => { lifecycle.unload = callback },
  })
  injectBusinessAPIs(context); vm.runInContext(executable(storeSource) + '\nglobalThis.store = { state, login }', context); context.store.state.user = { id: 'A', role }; context.store.state.ready = true
  const exposed = { users: 'users, toggleUser, confirming', services: 'services, review, status, chooseStatus', clubs: 'clubs, review, status, chooseStatus, openClub', orders: 'orders, refundOnly, chooseFilter, refundName' }[module]
  scope.run(() => vm.runInContext(executable(resourceSource) + '\n' + executable(pageScript('manage/' + module + '.vue')) + '\nglobalThis.page = { loadData, ' + exposed + ' }', context))
  return { page: context.page, store: context.store, requests, notices, dialogs, navigations, lifecycle, scope,
    /** 取得最新未消费请求，避免账号轮换期间误拿更早读取。 */
    take(path: string, method = 'GET') { const request = [...requests].reverse().find(item => !item.taken && item.path === path && item.method === method); assert.ok(request, `缺少请求 ${method} ${path}`); request.taken = true; return request },
  }
}
/** 等待真实脚本推进到下一次异步网络边界。 */
async function flush(): Promise<void> { await new Promise(resolve => setImmediate(resolve)) }
/** 显示并完成当前模块的首次真实列表读取。 */
async function load(subject: ReturnType<typeof harness>, path: string, rows: unknown[] = []): Promise<void> { subject.lifecycle.show(); await flush(); subject.take(path).resolve(rows); await flush() }

test('服务与俱乐部待办深链只接安全状态，并交给服务端筛选', async t => {
  for (const module of ['services', 'clubs'] as const) await t.test(module, async () => {
    const subject = harness(module); subject.lifecycle.load({ status: 'pending' }); assert.equal(subject.page.status.value, 'pending')
    await load(subject, '/admin/' + module + '?status=pending&limit=20&offset=0')
    subject.lifecycle.load({ status: "pending' OR TRUE" }); assert.equal(subject.page.status.value, 'pending')
    subject.scope.stop()
  })
})

test('服务通过后重新获取当前待审核列表，不把已通过服务留在当前结果', async () => {
  const subject = harness('services'); subject.lifecycle.load({ status: 'pending' }); await load(subject, '/admin/services?status=pending&limit=20&offset=0', [{ id: 'service-A', status: 'pending' }])
  const saving = subject.page.review('service-A', 'active'), write = subject.take('/admin/services/service-A', 'PATCH'); assert.equal(write.body.status, 'active'); write.resolve({ id: 'service-A', status: 'active' }); await flush(); subject.take('/admin/services?status=pending&limit=20&offset=0').resolve([]); await saving
  assert.equal(subject.page.services.value.length, 0); subject.scope.stop()
})

test('俱乐部审核与取消认证保留真实字段，三个入口指向已有主页的安全页签', async () => {
  const subject = harness('clubs'); await load(subject, '/admin/clubs?limit=20&offset=0')
  const saving = subject.page.review({ id: 'club-A' }, 'active', false), write = subject.take('/admin/clubs/club-A', 'PATCH'); assert.equal(write.body.status, 'active'); assert.equal(write.body.verified, false); write.resolve({ id: 'club-A' }); await flush(); subject.take('/admin/clubs?limit=20&offset=0').resolve([]); await saving
  subject.page.openClub('club A'); subject.page.openClub('club A', 'members'); subject.page.openClub('club A', 'orders')
  assert.deepEqual(subject.navigations, ['/pages/community/clubs/detail?id=club%20A&tab=info', '/pages/community/clubs/detail?id=club%20A&tab=members', '/pages/community/clubs/detail?id=club%20A&tab=orders']); subject.scope.stop()
})

test('订单refund深链与切换使用后端售后过滤，不新增前端局部状态筛选', async () => {
  const subject = harness('orders'); subject.lifecycle.load({ refund: '1' }); assert.equal(subject.page.refundOnly.value, true); await load(subject, '/admin/orders?refund=1&limit=20&offset=0')
  subject.page.chooseFilter(false); subject.take('/admin/orders?limit=20&offset=0').resolve([]); await flush(); assert.equal(subject.page.refundOnly.value, false)
  assert.equal(subject.page.refundName('processing'), '退款通道处理中'); assert.equal(subject.page.refundName('approved'), '退款完成'); subject.scope.stop()
})

test('冻结操作必须确认且禁止冻结自己，重复点击只出现一次弹框', async () => {
  const subject = harness('users'); await load(subject, '/admin/users?limit=20&offset=0')
  await subject.page.toggleUser({ id: 'A', status: 'active' }); assert.equal(subject.dialogs.length, 0)
  const cancelled = subject.page.toggleUser({ id: 'target', nickname: '目标用户', status: 'active' }); await subject.page.toggleUser({ id: 'target', status: 'active' }); assert.equal(subject.dialogs.length, 1); subject.dialogs[0].resolve({ confirm: false }); await cancelled; assert.equal(subject.requests.filter(item => item.method === 'PATCH').length, 0)
  const saving = subject.page.toggleUser({ id: 'target', nickname: '目标用户', status: 'active' }); subject.dialogs[1].resolve({ confirm: true }); await flush(); const write = subject.take('/admin/users/target', 'PATCH'); assert.equal(write.body.status, 'frozen'); write.resolve({ id: 'target' }); await flush(); subject.take('/admin/users?limit=20&offset=0').resolve([]); await saving
  assert.equal(subject.page.confirming.value, false); subject.scope.stop()
})

test('旧冻结弹框在离开再返回后确认，也不能执行原操作', async () => {
  const subject = harness('users'); await load(subject, '/admin/users?limit=20&offset=0'); const pending = subject.page.toggleUser({ id: 'target', status: 'active' })
  subject.lifecycle.hide(); await load(subject, '/admin/users?limit=20&offset=0'); subject.dialogs[0].resolve({ confirm: true }); await pending
  assert.equal(subject.requests.filter(item => item.method === 'PATCH').length, 0); assert.equal(subject.page.confirming.value, false); subject.scope.stop()
})

test('玩家和俱乐部管理员不能读取或审核平台资源', async t => {
  for (const role of ['user', 'clubowner']) for (const module of ['users', 'services', 'clubs', 'orders'] as const) await t.test(role + ':' + module, async () => {
    const subject = harness(module, role); subject.lifecycle.show(); await flush()
    if (module === 'services') await subject.page.review('service-A', 'active')
    if (module === 'clubs') await subject.page.review({ id: 'club-A' }, 'active', true)
    if (module === 'users') await subject.page.toggleUser({ id: 'target', status: 'active' })
    assert.equal(subject.requests.length, 0); assert.equal(subject.dialogs.length, 0); subject.scope.stop()
  })
})
