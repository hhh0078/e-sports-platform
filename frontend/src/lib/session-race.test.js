/** 文件作用：直接执行共享会话、个人页和订单页脚本，使用可控请求验证身份切换、资料保存及迟到响应。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed } from 'vue'

const storeSource = readFileSync(new URL('./store.js', import.meta.url), 'utf8')
const pageSource = readFileSync(new URL('../pages/me/index.vue', import.meta.url), 'utf8').match(/<script setup>([\s\S]*?)<\/script>/)[1]
const ordersSource = readFileSync(new URL('../pages/orders/index.vue', import.meta.url), 'utf8').match(/<script setup>([\s\S]*?)<\/script>/)[1]
const accountA = { id: 'A', nickname: '原始昵称', bio: '可以一起玩游戏', provider_enabled: true }
const accountB = { id: 'B', nickname: '另一玩家', bio: '另一个账号', provider_enabled: false }

/** 去掉模块边界，由测试注入请求及生命周期依赖；执行的业务函数仍全部来自真实源码。 */
function executable(source) { return source.replace(/^import .*$/gm, '').replace(/^export /gm, '') }

/** 创建真实 Vue 状态和真实页面函数，并记录尚未完成的网络请求。 */
function harness(pageName = 'me') {
  const storage = new Map([['jijie_token', 'token-A']]), requests = [], notices = []
  const context = vm.createContext({
    reactive, ref, computed,
    uni: { getStorageSync: key => storage.get(key), setStorageSync: (key, value) => storage.set(key, value), removeStorageSync: key => storage.delete(key) },
    token: () => storage.get('jijie_token') || '',
    /** 请求由测试明确安排返回顺序，不通过等待时长碰运气。 */
    api(path, method = 'GET', body) { return new Promise((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) },
    notify: value => notices.push(value), onShow: () => {}, onHide: () => {}, onLoad: () => {}, onBeforeUnmount: () => {},
  })
  vm.runInContext(executable(storeSource) + '\nglobalThis.store = { state, refreshMe, login, logout, sessionRevision }', context)
  context.store.state.user = { ...accountA }
  context.store.state.ready = true
  const source = pageName === 'orders' ? ordersSource : pageSource
  const exposed = pageName === 'orders' ? 'load, orders, loading, error, hasMore' : 'load, saveProfile, toggleProvider, signOut, hidePage, stats, form, saving'
  vm.runInContext(executable(source) + '\nglobalThis.page = { ' + exposed + ' }', context)
  return {
    store: context.store, page: context.page, storage, notices,
    /** 取出一次确实发出的请求，防止测试断言不存在的调用。 */
    take(path, method = 'GET') {
      const request = requests.find(item => !item.taken && item.path === path && item.method === method)
      assert.ok(request, `缺少请求 ${method} ${path}`)
      request.taken = true
      return request
    },
  }
}

/** 排空当前微任务，确保页面中的 await 已推进到下一次受控请求。 */
async function flush() { await new Promise(resolve => setImmediate(resolve)) }

/** 完成一次实际登录，供迟到读取与退出回调复用同一切换场景。 */
async function loginB(subject) {
  const pending = subject.store.login('B')
  subject.take('/auth/dev', 'POST').resolve({ token: 'token-B', user: { ...accountB } })
  await pending
}

/** 完成修改后的重新读取，验证最终统计也属于修改完成后的页面。 */
async function finishRefresh(subject, user) {
  await flush()
  subject.take('/me').resolve(user)
  await flush()
  subject.take('/me/stats').resolve({ buyer_order_count: 7 })
  await flush()
  assert.equal(subject.page.stats.value.buyer_order_count, 7)
}

test('旧账号资料成功或401都不能覆盖后来登录的账号', async t => {
  for (const outcome of ['success', '401']) {
    await t.test(outcome, async () => {
      const subject = harness(), pending = subject.store.refreshMe(), old = subject.take('/me')
      await loginB(subject)
      if (outcome === 'success') old.resolve({ ...accountA })
      else old.reject(Object.assign(new Error('旧会话已失效'), { status: 401 }))
      await pending
      assert.equal(subject.storage.get('jijie_token'), 'token-B')
      assert.equal(subject.store.state.user.id, 'B')
    })
  }
})

test('当前401仍清理身份，旧退出请求不能退出新账号', async t => {
  await t.test('current401', async () => {
    const subject = harness(), pending = subject.store.refreshMe()
    subject.take('/me').reject(Object.assign(new Error('会话已失效'), { status: 401 }))
    await assert.rejects(pending, error => error.status === 401)
    assert.equal(subject.store.state.user, null)
    assert.equal(subject.storage.has('jijie_token'), false)
  })
  await t.test('oldLogout', async () => {
    const subject = harness(), pending = subject.store.logout(), old = subject.take('/auth/logout', 'POST')
    await loginB(subject)
    old.resolve({})
    await pending
    assert.equal(subject.storage.get('jijie_token'), 'token-B')
    assert.equal(subject.store.state.user.id, 'B')
  })
})

test('个人页旧读取不能覆盖刚保存的资料或接单开关', async t => {
  for (const action of ['profile', 'provider']) {
    await t.test(action, async () => {
      const subject = harness(), reading = subject.page.load()
      await flush()
      const old = subject.take('/me')
      Object.assign(subject.page.form, { nickname: '保存后昵称', bio: accountA.bio })
      const saving = action === 'profile' ? subject.page.saveProfile() : subject.page.toggleProvider({ detail: { value: false } })
      const updated = { ...accountA, ...(action === 'profile' ? { nickname: '保存后昵称' } : { provider_enabled: false }) }
      subject.take('/me', 'PATCH').resolve(updated)
      await saving
      old.resolve({ ...accountA })
      await reading
      assert.equal(subject.store.state.user.nickname, updated.nickname)
      assert.equal(subject.store.state.user.provider_enabled, updated.provider_enabled)
      assert.equal(subject.page.saving.value, false)
      await finishRefresh(subject, updated)
    })
  }
})

test('个人页跨会话回调和隐藏后的统计不再提交', async t => {
  await t.test('old401', async () => {
    const subject = harness(), reading = subject.page.load()
    await flush()
    const old = subject.take('/me')
    await loginB(subject)
    old.reject(Object.assign(new Error('旧会话已失效'), { status: 401 }))
    await reading
    assert.equal(subject.storage.get('jijie_token'), 'token-B')
    assert.equal(subject.store.state.user.id, 'B')
  })
  await t.test('oldPatch', async () => {
    const subject = harness()
    Object.assign(subject.page.form, { nickname: '旧账号新昵称', bio: accountA.bio })
    const saving = subject.page.saveProfile(), old = subject.take('/me', 'PATCH')
    await loginB(subject)
    old.resolve({ ...accountA, nickname: '旧账号新昵称' })
    await saving
    assert.equal(subject.store.state.user.id, 'B')
    assert.equal(subject.notices.length, 0)
  })
  await t.test('hiddenStats', async () => {
    const subject = harness(), reading = subject.page.load()
    await flush()
    subject.take('/me').resolve({ ...accountA })
    await flush()
    const old = subject.take('/me/stats')
    subject.page.hidePage()
    old.resolve({ buyer_order_count: 999 })
    await reading
    assert.equal(Object.keys(subject.page.stats.value).length, 0)
  })
  await t.test('logoutBeforeOldRead', async () => {
    const subject = harness(), reading = subject.page.load()
    await flush()
    const old = subject.take('/me'), signingOut = subject.page.signOut()
    subject.take('/auth/logout', 'POST').resolve({})
    await signingOut
    old.resolve({ ...accountA })
    await reading
    assert.equal(subject.store.state.user, null)
    assert.equal(subject.storage.has('jijie_token'), false)
  })
})

test('订单当前401切回可登录空态，旧账号响应不影响新账号', async t => {
  await t.test('expiredSession', async () => {
    const subject = harness('orders')
    subject.page.loading.value = false
    subject.page.orders.value = [{ id: 'old-order' }]
    subject.page.hasMore.value = true
    const reading = subject.page.load(true)
    await flush()
    subject.take('/orders?role=buyer&status=all&limit=20&offset=1').reject(Object.assign(new Error('请先登录'), { status: 401 }))
    await reading
    assert.equal(subject.store.state.user, null)
    assert.equal(subject.storage.has('jijie_token'), false)
    assert.equal(subject.page.orders.value.length, 0)
    assert.equal(subject.page.hasMore.value, false)
    assert.equal(subject.page.error.value, '')
    assert.equal(subject.page.loading.value, false)
  })
  for (const outcome of ['old401', 'oldOrders']) {
    await t.test(outcome, async () => {
      const subject = harness('orders'), reading = subject.page.load()
      await flush()
      const old = subject.take('/orders?role=buyer&status=all&limit=20&offset=0')
      await loginB(subject)
      if (outcome === 'old401') old.reject(Object.assign(new Error('旧会话失效'), { status: 401 }))
      else old.resolve([{ id: 'private-A-order', buyer_id: 'A' }])
      await reading
      assert.equal(subject.storage.get('jijie_token'), 'token-B')
      assert.equal(subject.store.state.user.id, 'B')
      assert.equal(subject.page.orders.value.length, 0)
      assert.equal(subject.page.error.value, '')
    })
  }
})
