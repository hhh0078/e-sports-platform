/** 文件作用：执行真实原生刷新入口，验证重复手势、刷新与追加竞争、错误复位及隐藏迟到回包。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { ref, watch } from 'vue'
import { executable } from './sfc-test'

/** 可由测试明确完成的网络请求，避免依赖固定等待。 */
function deferred() { let resolve!: () => void, reject!: (error: Error) => void; const promise = new Promise<void>((done, fail) => { resolve = done; reject = fail }); return { promise, resolve, reject } }

/** 执行真实共享入口，只替换官方生命周期和原生动画边界。 */
function setup() {
  const hooks: Record<string, () => void> = {}, errors: unknown[] = [], refreshes: ReturnType<typeof deferred>[] = [], pages: ReturnType<typeof deferred>[] = []
  let stopped = 0, blocked = false
  const key = ref('first')
  const context = vm.createContext({ ref, watch, uni: { stopPullDownRefresh: () => stopped++ }, notify: (error: unknown) => errors.push(error), onShow: (fn: () => void) => { hooks.show = fn }, onHide: (fn: () => void) => { hooks.hide = fn }, onUnload: (fn: () => void) => { hooks.unload = fn }, onPullDownRefresh: (fn: () => void) => { hooks.pull = fn }, onReachBottom: (fn: () => void) => { hooks.bottom = fn }, options: { refresh: () => { const item = deferred(); refreshes.push(item); return item.promise }, more: () => { const item = deferred(); pages.push(item); return item.promise }, blocked: () => blocked, context: () => key.value } })
  vm.runInContext(executable(readFileSync(new URL('./native-page-refresh.ts', import.meta.url), 'utf8')) + '\nglobalThis.subject=useNativePageRefresh(options)', context)
  return { subject: context.subject, hooks, refreshes, pages, errors, stopped: () => stopped, block: (value: boolean) => { blocked = value }, changeContext: (value: string) => { key.value = value } }
}
/** 排空真实Promise回调，不模拟网络耗时。 */
async function flush(): Promise<void> { await new Promise<void>(resolve => setImmediate(resolve)) }

test('连续下拉和触底均只启动一次，刷新期间不追加', async () => {
  const x = setup()
  x.hooks.pull(); x.hooks.pull(); x.hooks.bottom()
  assert.equal(x.refreshes.length, 1); assert.equal(x.pages.length, 0)
  assert.equal(x.stopped(), 0, '首请求未完成时重复下拉不能提前停止动画')
  x.refreshes[0].resolve(); await flush()
  assert.equal(x.subject.refreshing.value, false); assert.ok(x.stopped() > 0)
  x.hooks.bottom(); x.hooks.bottom(); assert.equal(x.pages.length, 1)
  x.pages[0].resolve(); await flush(); assert.equal(x.subject.loadingMore.value, false)
})

test('正在追加时可刷新，两个请求分别结束而不重复触底', async () => {
  const x = setup()
  x.hooks.bottom(); x.hooks.pull(); x.hooks.bottom()
  assert.equal(x.pages.length, 1); assert.equal(x.refreshes.length, 1)
  x.refreshes[0].resolve(); await flush()
  assert.equal(x.subject.refreshing.value, false); assert.equal(x.subject.loadingMore.value, false, '成功刷新应撤销被替代的追加锁')
  x.hooks.bottom(); assert.equal(x.pages.length, 2, '新首屏成功后不应等待旧下一页超时')
  x.pages[0].resolve(); await flush(); assert.equal(x.subject.loadingMore.value, true, '旧页finally不能关闭新下一页')
  x.pages[1].resolve(); await flush(); assert.equal(x.subject.loadingMore.value, false)
})

test('错误和业务受阻均停止动画，后续可重试', async () => {
  const x = setup()
  x.hooks.pull(); x.refreshes[0].reject(new Error('网络失败')); await flush()
  assert.equal(x.errors.length, 1); assert.equal(x.subject.refreshing.value, false)
  x.block(true); const before = x.stopped(); x.hooks.pull(); x.hooks.bottom()
  assert.equal(x.refreshes.length, 1); assert.equal(x.pages.length, 0); assert.equal(x.stopped(), before + 1)
  x.block(false); x.hooks.pull(); x.refreshes[1].resolve(); await flush()
  assert.equal(x.refreshes.length, 2)
})

test('隐藏后的迟到结束不能关闭重新显示页的新动画或产生旧错误', async () => {
  const x = setup()
  x.hooks.pull(); x.hooks.hide(); x.hooks.bottom(); assert.equal(x.pages.length, 0)
  x.hooks.show(); x.hooks.pull(); const before = x.stopped()
  x.refreshes[0].reject(new Error('旧请求失败')); await flush()
  assert.equal(x.subject.refreshing.value, true); assert.equal(x.stopped(), before); assert.equal(x.errors.length, 0)
  x.refreshes[1].resolve(); await flush(); assert.equal(x.subject.refreshing.value, false)
})

test('只有已确认的可变二级页启用原生下拉，编辑与发布表单不重置草稿', () => {
  const config = JSON.parse(readFileSync(new URL('../pages.json', import.meta.url), 'utf8')) as { pages: Array<{ path: string; style: { enablePullDownRefresh?: boolean } }> }
  for (const path of ['services/detail','orders/detail','account/wallet','community/clubs/index','community/clubs/detail','community/players/detail','manage/index','manage/wallet','manage/users','manage/services','manage/clubs','manage/orders','manage/games','manage/audit','account/messages/index','manage/moderation','community/clubs/console','community/clubs/funds','community/clubs/deposit','manage/finance/index','manage/finance/detail','requests/detail','account/subscriptions','manage/subscriptions']) assert.equal(config.pages.find(page => page.path === 'pages/' + path)?.style.enablePullDownRefresh, true, path)
  for (const path of ['main/index','account/login','account/profile','services/publish','requests/publish','community/clubs/edit','manage/platform','manage/support','manage/ai','manage/finance/settings']) assert.notEqual(config.pages.find(page => page.path === 'pages/' + path)?.style.enablePullDownRefresh, true, path + '不能原生下拉重置')
})

test('同页换号或筛选立即放开旧追加锁，旧结束不能关闭新加载', async () => {
  const x = setup()
  x.hooks.bottom(); x.changeContext('new-account-or-filter'); x.hooks.bottom()
  assert.equal(x.pages.length, 2, '新上下文不能等待旧请求超时')
  x.pages[0].resolve(); await flush(); assert.equal(x.subject.loadingMore.value, true)
  x.pages[1].resolve(); await flush(); assert.equal(x.subject.loadingMore.value, false)
  x.hooks.pull(); x.changeContext('third'); x.hooks.pull()
  x.refreshes[0].resolve(); await flush(); assert.equal(x.subject.refreshing.value, true)
  x.refreshes[1].resolve(); await flush(); assert.equal(x.subject.refreshing.value, false)
})
