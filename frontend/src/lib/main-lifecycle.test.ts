/** 文件作用：执行真实主视图、导航和会话脚本，验证深链消费、分页保留与隐藏期间的身份隔离。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch, effectScope, proxyRefs } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import { appendUniqueRows } from './list-page'
import type { DeferredRequest } from './sfc-test'

/** 本测试执行的四个常驻内容视图。 */
type ViewName = 'player' | 'worker' | 'orders' | 'me'
/** 列表夹具只使用实际业务函数需要检查的字段。 */
interface Row { id: string; provider_id: string; requirements: string; status: string }
const accountA = { id: 'A', nickname: '甲队友', bio: '一起认真玩游戏', provider_enabled: true }
const accountB = { id: 'B', nickname: '乙队友', bio: '另一位游戏爱好者', provider_enabled: true }

/** 将一次主页面显示、隐藏及传输边界交给测试控制，其余执行真实业务实现。 */
function harness(t: TestContext, view: ViewName, total = 125) {
  const storage = new Map([['jijie_token', 'token-A']]), props = reactive({ active: true })
  const shows: Array<() => void> = [], hides: Array<() => void> = [], held = new Set<string>()
  const requests: Array<{ path: string; method: string }> = [], pending: DeferredRequest[] = [], navigations: string[] = []
  const rows: Row[] = Array.from({ length: total }, (_, index) => ({ id: 'row-' + index, provider_id: 'A', requirements: '完成约定任务', status: 'pending' }))
  const game = { id: 'delta', categories: ['任务协作'], covers: [], primary_color: '#00e18a' }
  const context = vm.createContext({
    reactive, ref, computed, watch, appendUniqueRows, defineExpose:()=>{},
    defineProps: () => props,
    onShow: (callback: () => void) => shows.push(callback), onHide: (callback: () => void) => hides.push(callback), onBeforeUnmount: () => {},
    uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: string) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key) },
    token: () => storage.get('jijie_token') || '', notify: () => {}, go: (url: string) => navigations.push(url),
    /** 记录实际业务 API 的查询参数，只有明确挂起的路径需要测试手动完成。 */
    api(path: string, method = 'GET', body: Record<string, unknown> = {}): Promise<unknown> {
      requests.push({ path, method })
      const url = new URL('http://unit.test' + path)
      if (held.has(url.pathname)) return new Promise((resolve, reject) => pending.push({ path, method, body, resolve, reject, taken: false }))
      if (url.pathname === '/bootstrap') return Promise.resolve({ games: [game], dev_mode: true })
      if (url.pathname === '/me') return Promise.resolve(context.store.state.user)
      if (url.pathname === '/me/stats') return Promise.resolve({ services_count: 7, buyer_order_count: 5 })
      if (url.pathname === '/clubs') return Promise.resolve([])
      if (['/services', '/orders', '/jobs'].includes(url.pathname)) { const limit = Number(url.searchParams.get('limit')), offset = Number(url.searchParams.get('offset')); return Promise.resolve(rows.slice(offset, offset + limit)) }
      return Promise.resolve({})
    },
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(readFileSync(new URL('./store.ts', import.meta.url), 'utf8')) + '\nglobalThis.store={state,login,clearSession,sessionRevision}', context)
  context.store.state.user = { ...accountA }
  context.store.state.ready = true
  context.store.state.games = [game]
  vm.runInContext(executable(readFileSync(new URL('./navigation.ts', import.meta.url), 'utf8')) + '\nglobalThis.navigation={mainNavigation,applyMainQuery}', context)
  vm.runInContext(executable(readFileSync(new URL('./order-list.ts', import.meta.url), 'utf8')), context)
  const exposed: Record<ViewName, string> = {
    player: 'refreshKey,refresh,loadMore,retryList,services,hasMore,error,moreError,query,category,loading,hasLoaded,loadServices,reload,showPage,hidePage',
    worker: 'refreshKey,refresh,loadMore,retryList,publicClubs,nextOffsets,mine,jobs,stats,claimClubs,hasMore,error,moreError,tab,currentListCount,loading,loadedTabs,load,showPage,stopLoading,changeTab,claim',
    orders: 'refreshKey,refresh,loadMore,retryList,orders,hasMore,error,moreError,role,filter,loading,hasLoaded,load,showPage,hidePage,chooseFilter,chooseRole',
    me: 'refresh,wallet,walletError,stats,load,showPage,hidePage',
  }
  const scope = effectScope()
  scope.run(() => vm.runInContext(executable(pageScript('main/' + view + '/' + view + '.vue')) + '\nglobalThis.page={' + exposed[view] + '}', context))
  const source = readFileSync(new URL('../pages/main/' + view + '/' + view + '.vue', import.meta.url), 'utf8')
  const loaderCondition = source.match(/v-else-if="([^"]+)"\s+class="loading-line"/)?.[1]
  t.after(() => scope.stop())
  return {
    page: context.page, store: context.store, navigation: context.navigation, storage, props, held, pending, requests, rows, game, navigations,
    /** 触发主页面本身的真实显示生命周期。 */
    show() { shows.forEach(callback => callback()) },
    /** 触发主页面本身的真实隐藏生命周期。 */
    hide() { hides.forEach(callback => callback()) },
    /** 用真实SFC加载分支和Vue自动解引用检查当前是否会把原空态替换成短加载行。 */
    showsInitialLoader() { assert.ok(loaderCondition, view + '缺少实际加载分支'); return Boolean(new Function('page', 'with (page) { return ' + loaderCondition + ' }')(proxyRefs(context.page))) },
    /** 只统计该视图的列表请求，配置和资料刷新另行记录。 */
    listRequests() { return requests.filter(request => /^\/(services|orders|jobs)\?/.test(request.path)) },
    /** 取得测试明确挂起的一次请求。 */
    take(path: string) { const request = pending.find(item => !item.taken && item.path.split('?')[0] === path); assert.ok(request, '缺少挂起请求 ' + path); request.taken = true; return request },
  }
}

/** 排空 Vue 侦听与业务 Promise，不使用固定耗时模拟网络完成。 */
async function flush(): Promise<void> { await new Promise<void>(resolve => setImmediate(resolve)) }

test('实际SFC已加载空列表复激活时保留空态，初始化和列表刷新等待及失败均不缩为加载行', async t => {
  for (const view of ['player', 'worker', 'orders'] as const) await t.test(view, async subtest => {
    const subject = harness(subtest, view, 0), route = view === 'orders' ? '/orders' : '/services'
    subject.held.add(route); subject.show(); await flush()
    assert.equal(subject.showsInitialLoader(), true, '首次加载应显示加载提示')
    subject.take(route).resolve([]); await flush()
    assert.equal(subject.showsInitialLoader(), false)
    subject.props.active = false; await flush()
    subject.store.state.ready = false; subject.held.add('/bootstrap')
    subject.props.active = true; await flush()
    assert.equal(subject.showsInitialLoader(), false, '复激活等待初始化时原空态不能消失')
    subject.take('/bootstrap').resolve({ games: [subject.game], dev_mode: true }); await flush()
    assert.equal(subject.page.loading.value, true)
    assert.equal(subject.showsInitialLoader(), false, '复激活等待业务列表时原空态不能消失')
    subject.take(route).reject(new Error('刷新暂时失败')); await flush()
    assert.equal(subject.page.error.value, '')
    assert.equal(subject.page.moreError.value, '刷新暂时失败')
    assert.equal(subject.showsInitialLoader(), false)
    const list = view === 'player' ? subject.page.services : view === 'worker' ? subject.page.mine : subject.page.orders
    assert.equal(list.value.length, 0)
  })
})

test('访客订单和工作台成功空态在重显等待初始化时保留', async t => {
  for (const view of ['worker', 'orders'] as const) await t.test(view, async subtest => {
    const subject = harness(subtest, view, 0)
    subject.store.clearSession(); subject.show(); await flush()
    assert.equal(subject.showsInitialLoader(), false)
    if (view === 'worker') assert.equal(subject.page.loadedTabs.value.services, true)
    else assert.equal(subject.page.hasLoaded.value, true)
    subject.props.active = false; await flush()
    subject.store.state.ready = false; subject.held.add('/bootstrap')
    subject.props.active = true; await flush()
    assert.equal(subject.page.loading.value, true)
    assert.equal(subject.showsInitialLoader(), false)
    subject.take('/bootstrap').resolve({ games: [subject.game], dev_mode: true }); await flush()
    assert.equal(subject.showsInitialLoader(), false)
  })
})

test('新筛选仍显示首次加载，切号立即清除上一身份的成功空态标记', async t => {
  for (const view of ['player', 'worker', 'orders'] as const) await t.test(view, async subtest => {
    const subject = harness(subtest, view, 0)
    subject.show(); await flush()
    const route = view === 'orders' ? '/orders' : view === 'worker' ? '/jobs' : '/services'
    subject.held.add(route)
    if (view === 'orders') subject.page.chooseFilter('completed')
    else if (view === 'worker') subject.page.changeTab('jobs')
    else { subject.page.query.value = '新条件'; void subject.page.loadServices() }
    await flush()
    assert.equal(subject.showsInitialLoader(), true, '新筛选尚无成功结果，不能沿用其它空态标记')
    subject.take(route).resolve([]); await flush()
    assert.equal(subject.showsInitialLoader(), false)
    if (view !== 'player') {
      subject.props.active = false; await flush(); subject.store.clearSession()
      if (view === 'orders') assert.equal(subject.page.hasLoaded.value, false)
      else assert.deepEqual({ ...subject.page.loadedTabs.value }, { services: false, jobs: false, clubs: false })
    }
  })
})

test('订单深链每个导航版本只消费一次，详情返回保留用户之后选择的筛选', async t => {
  const subject = harness(t, 'orders')
  subject.navigation.applyMainQuery({ tab: 'orders', role: 'provider', status: 'working' })
  subject.show(); await flush()
  assert.equal(subject.page.filter.value, 'working')
  subject.page.chooseFilter('completed'); await flush()
  subject.hide(); subject.show(); await flush()
  assert.equal(subject.page.filter.value, 'completed')
  assert.equal(subject.page.role.value, 'provider')
  const before = subject.listRequests().length
  subject.navigation.applyMainQuery({ tab: 'orders', role: 'buyer', status: 'pending_payment' }); await flush()
  assert.equal(subject.page.filter.value, 'pending_payment')
  assert.equal(subject.page.role.value, 'buyer')
  assert.equal(subject.listRequests().length, before + 1)
})

test('订单标签激活和新深链同帧到达仅发起一次列表读取', async t => {
  const subject = harness(t, 'orders')
  subject.props.active = false; subject.show(); await flush()
  assert.equal(subject.listRequests().length, 0)
  subject.navigation.applyMainQuery({ tab: 'orders', role: 'provider', status: 'working' })
  subject.props.active = true; await flush()
  assert.equal(subject.listRequests().length, 1)
  assert.equal(subject.page.filter.value, 'working')
})

test('三类列表返回时保留超过一百条的已加载内容，并正确判断后续页', async t => {
  for (const view of ['player', 'worker', 'orders'] as const) await t.test(view, async subtest => {
    const subject = harness(subtest, view), list = view === 'player' ? subject.page.services : view === 'worker' ? subject.page.mine : subject.page.orders
    list.value = subject.rows.slice(0, 120); subject.page.hasMore.value = true
    subject.show(); await flush()
    assert.equal(list.value.length, 120)
    assert.equal(list.value[119].id, 'row-119')
    assert.equal(subject.page.hasMore.value, true)
    const pages = subject.listRequests().map(request => new URL('http://unit.test' + request.path))
    assert.deepEqual(pages.map(url => [url.searchParams.get('limit'), url.searchParams.get('offset')]), [['100', '0'], ['21', '100']])
    subject.hide(); subject.show(); await flush()
    assert.equal(list.value.length, 120)
  })
})

test('返回刷新不会把已经到底的列表错误标记为仍有下一页', async t => {
  const subject = harness(t, 'orders', 120)
  subject.page.orders.value = subject.rows.slice(); subject.page.hasMore.value = false
  subject.show(); await flush()
  assert.equal(subject.page.orders.value.length, 120)
  assert.equal(subject.page.hasMore.value, false)
})

test('重显刷新失败保留旧列表与分页，不用整页错误态截断滚动容器', async t => {
  for (const view of ['player', 'worker', 'orders'] as const) await t.test(view, async subtest => {
    const subject = harness(subtest, view), list = view === 'player' ? subject.page.services : view === 'worker' ? subject.page.mine : subject.page.orders
    list.value = subject.rows.slice(0, 40); subject.page.hasMore.value = true
    const route = view === 'orders' ? '/orders' : '/services'
    subject.held.add(route); subject.show(); await flush()
    subject.take(route).reject(new Error('暂时离线')); await flush()
    assert.equal(list.value.length, 40)
    assert.equal(subject.page.hasMore.value, true)
    assert.equal(subject.page.error.value, '')
    assert.equal(subject.page.moreError.value, '暂时离线')
  })
})

test('三类列表在初始化尚未完成时隐藏，不再发起业务列表请求', async t => {
  for (const view of ['player', 'worker', 'orders'] as const) await t.test(view, async subtest => {
    const subject = harness(subtest, view)
    subject.store.state.ready = false; subject.held.add('/bootstrap')
    subject.show(); await flush(); subject.hide()
    subject.take('/bootstrap').resolve({ games: [subject.game], dev_mode: true }); await flush()
    assert.equal(subject.listRequests().length, 0)
  })
})

test('私有常驻视图隐藏时同账号会话轮换也立即清空缓存', async t => {
  for (const view of ['worker', 'orders', 'me'] as const) await t.test(view, async subtest => {
    const subject = harness(subtest, view)
    if (view === 'worker') { subject.page.mine.value = [subject.rows[0]]; subject.page.claimClubs.value = [{ id: 'private-club' }] }
    if (view === 'orders') subject.page.orders.value = [subject.rows[0]]
    if (view !== 'orders') subject.page.stats.value = { services_count: 99 }
    subject.held.add('/auth/dev')
    const before = subject.listRequests().length, signingIn = subject.store.login('A')
    if (view === 'worker') { assert.equal(subject.page.mine.value.length, 0); assert.equal(subject.page.claimClubs.value.length, 0) }
    if (view === 'orders') assert.equal(subject.page.orders.value.length, 0)
    if (view !== 'orders') assert.equal(Object.keys(subject.page.stats.value).length, 0)
    assert.equal(subject.listRequests().length, before)
    subject.take('/auth/dev').resolve({ token: 'rotated-A', user: { ...accountA } }); await signingIn; await flush()
  })
})

test('工作台新账号请求等待期间保持空态，旧账号迟到服务不能重新出现', async t => {
  const subject = harness(t, 'worker')
  subject.held.add('/services'); subject.show(); await flush()
  const old = subject.take('/services')
  subject.page.mine.value = [subject.rows[0]]
  subject.hide(); subject.held.add('/auth/dev')
  const signingIn = subject.store.login('B')
  assert.equal(subject.page.mine.value.length, 0)
  subject.take('/auth/dev').resolve({ token: 'token-B', user: { ...accountB } }); await signingIn
  subject.show(); await flush()
  assert.equal(subject.page.mine.value.length, 0)
  old.resolve([subject.rows[0]]); await flush()
  assert.equal(subject.page.mine.value.length, 0)
  subject.take('/services').resolve([]); await flush()
  assert.equal(subject.page.mine.value.length, 0)
  assert.equal(subject.store.state.user.id, 'B')
})


test('工作台旧本地接单标记为false时仍可申请领取，不再要求手动启用身份', async t => {
  const subject=harness(t,'worker')
  subject.show();await flush()
  subject.store.state.user.provider_enabled=false
  subject.held.add('/orders/job-A/actions')
  const claiming=subject.page.claim({id:'job-A',game_id:'delta'})
  await flush()
  const request=subject.take('/orders/job-A/actions')
  assert.equal(request.method,'POST')
  assert.equal(request.body.action,'claim')
  assert.equal(subject.requests.filter(item=>item.path==='/me'&&item.method==='PATCH').length,0)
  request.resolve({id:'accepted-order'});await claiming
  assert.deepEqual(subject.navigations,['/pages/orders/detail?id=accepted-order'])
})


test('三个主列表连续触底只追加一次，重复ID不导致原始offset倒退',async t=>{for(const view of ['player','worker','orders'] as const)await t.test(view,async subtest=>{const s=harness(subtest,view,90),route=view==='orders'?'/orders':'/services',list=view==='player'?s.page.services:view==='worker'?s.page.mine:s.page.orders,size=view==='player'?24:20;s.show();await flush();s.held.add(route);const first=s.page.loadMore(),duplicate=s.page.loadMore();await flush();assert.equal(s.pending.filter(item=>item.path.split('?')[0]===route).length,1);const page=[s.rows[size-1],...s.rows.slice(size,size*2-1)];s.take(route).resolve(page);await Promise.all([first,duplicate]);assert.equal(list.value.length,size*2-1);assert.equal(new Set(list.value.map((item:Row)=>item.id)).size,list.value.length);const next=s.page.loadMore();await flush();const request=s.take(route),url=new URL('http://unit.test'+request.path);assert.equal(Number(url.searchParams.get('offset')),size*2);request.resolve([]);await next;assert.equal(s.page.hasMore.value,false);const count=s.requests.length;await s.page.loadMore();assert.equal(s.requests.length,count)})})
test('下拉失败保留主列表，错误重试仍刷新原范围而不是跳下一页',async t=>{for(const view of ['player','worker','orders'] as const)await t.test(view,async subtest=>{const s=harness(subtest,view,65),route=view==='orders'?'/orders':'/services',list=view==='player'?s.page.services:view==='worker'?s.page.mine:s.page.orders;s.show();await flush();const before=list.value.length;s.held.add(route);const refreshing=s.page.refresh();await flush();s.take(route).reject(new Error('下拉网络失败'));await refreshing;assert.equal(list.value.length,before);assert.equal(s.page.moreError.value,'下拉网络失败');const retry=s.page.retryList();await flush();const request=s.take(route);assert.equal(new URL('http://unit.test'+request.path).searchParams.get('offset'),'0');request.resolve(s.rows.slice(0,before+1));await retry})})
test('俱乐部页签恰好20条时保留刷新正确判定到底，不再触底空转',async t=>{const s=harness(t,'worker');s.show();await flush();s.held.add('/clubs');s.page.changeTab('clubs');await flush();s.take('/clubs').resolve(s.rows.slice(0,20));await flush();assert.equal(s.page.hasMore.value,true);const refreshing=s.page.refresh();await flush();const request=s.take('/clubs');assert.equal(new URL('http://unit.test'+request.path).searchParams.get('limit'),'21');request.resolve(s.rows.slice(0,20));await refreshing;assert.equal(s.page.publicClubs.value.length,20);assert.equal(s.page.hasMore.value,false)})
test('我的下拉等待和故障保留已确认统计与余额',async t=>{const s=harness(t,'me');s.show();await flush();s.page.stats.value={buyer_order_count:42};s.page.wallet.value={available_cents:12345};s.held.add('/me');const refreshing=s.page.refresh();await flush();assert.equal(s.page.stats.value.buyer_order_count,42);assert.equal(s.page.wallet.value.available_cents,12345);s.take('/me').reject(new Error('暂时离线'));await refreshing;assert.equal(s.page.stats.value.buyer_order_count,42);assert.equal(s.page.wallet.value.available_cents,12345)})

test('真实子视图更换分类、子页签及订单筛选会更新容器上下文',async t=>{for(const view of ['player','worker','orders'] as const)await t.test(view,async subtest=>{const s=harness(subtest,view);s.show();await flush();const key=s.page.refreshKey.value;if(view==='player')await s.page.loadServices();else if(view==='worker')s.page.changeTab('jobs');else s.page.chooseFilter('working');assert.ok(s.page.refreshKey.value>key)})})

test('我的下拉成功后清除历史钱包同步错误，不保留过期警告',async t=>{const s=harness(t,'me');s.show();await flush();s.page.walletError.value='历史钱包读取失败';await s.page.refresh();assert.equal(s.page.walletError.value,'')})

test('主列表原生触底在刷新失败后恢复首读，追加失败后仍重试原页',async t=>{for(const view of ['player','worker','orders'] as const)await t.test(view,async subtest=>{const s=harness(subtest,view,65),route=view==='orders'?'/orders':'/services',size=view==='player'?24:20;s.show();await flush();s.held.add(route);const refreshing=s.page.refresh();await flush();s.take(route).reject(new Error('刷新失败'));await refreshing;const retry=s.page.loadMore();await flush();const first=s.take(route);assert.equal(new URL('http://unit.test'+first.path).searchParams.get('offset'),'0');first.resolve(s.rows.slice(0,size+1));await retry;const appending=s.page.loadMore();await flush();const failed=s.take(route);assert.equal(Number(new URL('http://unit.test'+failed.path).searchParams.get('offset')),size);failed.reject(new Error('追加失败'));await appending;const again=s.page.loadMore();await flush();const repeated=s.take(route);assert.equal(Number(new URL('http://unit.test'+repeated.path).searchParams.get('offset')),size);repeated.resolve([]);await again;assert.equal(s.page.hasMore.value,false)})})
