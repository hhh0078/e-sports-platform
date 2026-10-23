/** 文件作用：直接执行共享会话、个人中心、资料页和订单页脚本，使用可控请求验证身份切换、资料保存及迟到响应。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'
import { refreshMainList } from './navigation'

const storeSource = readFileSync(new URL('./store.ts', import.meta.url), 'utf8')
const phoneSource = readFileSync(new URL('./phone.ts', import.meta.url), 'utf8')
const meSource = pageScript('main/me/me.vue')
const profileSource = pageScript('account/profile.vue')
const ordersSource = pageScript('main/orders/orders.vue')
const accountA = { id: 'A', nickname: '原始昵称', bio: '可以一起玩游戏', provider_enabled: true }
const accountB = { id: 'B', nickname: '另一玩家', bio: '另一个账号', provider_enabled: true }

/** 创建真实 Vue 状态和真实页面函数，并记录尚未完成的网络请求。 */
function harness(pageName: 'me' | 'profile' | 'orders' = 'me') {
  const storage = new Map<string, string>([['jijie_token', 'token-A']]), requests: DeferredRequest[] = [], notices: unknown[] = [], navigations: string[] = []
  const context = vm.createContext({
    reactive, ref, computed, watch, refreshMainList, defineProps: () => ({ active: true }), useMainVisibility: () => {}, mainNavigation: reactive({ query: {}, revision: 0 }),
    uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: string) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key) },
    token: () => storage.get('jijie_token') || '',
    /** 请求由测试明确安排返回顺序，不通过等待时长碰运气。 */
    api(path: string, method = 'GET', body: Record<string, unknown> = {}) { return new Promise<unknown>((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) },
    notify: (value: unknown) => notices.push(value), go: (value: string) => navigations.push(value), onShow: () => {}, onHide: () => {}, onLoad: () => {}, onUnload: () => {}, onMounted: () => {}, onBeforeUnmount: () => {},
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource) + '\nglobalThis.store = { state, refreshMe, login, prepareWechatLogin, wechatLogin, logout, sessionRevision, lastLoginWasNew, clearSession }', context)
  context.store.state.user = { ...accountA }
  context.store.state.ready = true
  if(pageName==='me')vm.runInContext('(function(){'+executable(phoneSource)+';globalThis.requirePhone=requirePhone;globalThis.phone={phoneGate,finishPhone}})()',context)
  const source = { me: meSource, profile: profileSource, orders: ordersSource }[pageName]
  const exposed = { me: 'load, verifyPhone, hidePage, stats,wallet,walletError,orderCount,walletAmount', profile: 'load, saveProfile, signOut, hidePage, form, saving', orders: 'load, orders, loading, error, hasMore' }[pageName]
  vm.runInContext(executable(source) + '\nglobalThis.page = { ' + exposed + ' }', context)
  if (pageName === 'me' || pageName === 'orders') vm.runInContext('visible = true', context)
  return {
    store: context.store, page: context.page, phone: context.phone, storage, notices, navigations,
    /** 取出一次确实发出的请求，防止测试断言不存在的调用。 */
    take(path: string, method = 'GET') {
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
async function loginB(subject: ReturnType<typeof harness>) {
  const pending = subject.store.login('B')
  subject.take('/auth/dev', 'POST').resolve({ token: 'token-B', user: { ...accountB } })
  await pending
}

test('个人中心手机号读取失败正常提示且不会产生未处理拒绝', async () => {
  const subject=harness(),verifying=subject.page.verifyPhone()
  subject.take('/me').reject(new Error('手机号状态读取失败'))
  await assert.doesNotReject(verifying)
  assert.equal(subject.notices.length,1)
  assert.match(String(subject.notices[0]),/手机号状态读取失败/)
  assert.equal(subject.phone.phoneGate.visible,false)
  assert.equal(subject.storage.get('jijie_token'),'token-A')
})

test('离页或换账号后的手机号读取失败不弹到其他页面或身份', async t => {
  for(const boundary of ['hide','switch'])await t.test(boundary,async()=>{
    const subject=harness(),verifying=subject.page.verifyPhone(),old=subject.take('/me')
    if(boundary==='hide')subject.page.hidePage();else await loginB(subject)
    old.reject(new Error('旧手机号请求失败'))
    await assert.doesNotReject(verifying)
    assert.deepEqual(subject.notices,[])
    assert.equal(subject.phone.phoneGate.visible,false)
    if(boundary==='switch')assert.equal(subject.store.state.user.id,'B')
  })
})

test('个人中心正常打开手机号门槛，关闭只取消授权不提示失败', async () => {
  const subject=harness(),verifying=subject.page.verifyPhone()
  subject.take('/me').resolve({...accountA,phone_verified:false});await flush()
  assert.equal(subject.phone.phoneGate.visible,true)
  subject.phone.finishPhone(false)
  await assert.doesNotReject(verifying)
  assert.equal(subject.phone.phoneGate.visible,false)
  assert.deepEqual(subject.notices,[])
})

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
    await assert.rejects(pending, error => typeof error === 'object' && error !== null && 'status' in error && error.status === 401)
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

test('资料页的旧读取不能覆盖刚保存的资料', async () => {
  const subject=harness('profile'),reading=subject.page.load()
  await flush()
  const old=subject.take('/me')
  Object.assign(subject.page.form,{nickname:'保存后昵称',bio:accountA.bio})
  const saving=subject.page.saveProfile(),updated={...accountA,nickname:'保存后昵称'}
  subject.take('/me/profile','PATCH').resolve(updated)
  await saving
  old.resolve({...accountA});await reading
  assert.equal(subject.store.state.user.nickname,updated.nickname)
  assert.equal(subject.store.state.user.provider_enabled,true)
  assert.equal(subject.page.saving.value,false)
  assert.equal(subject.page.form.nickname,updated.nickname)
})

test('个人中心和资料页的跨会话回调及隐藏后的统计不再提交', async t => {
  for (const pageName of ['me', 'profile'] as const) {
    await t.test(pageName + '-old401', async () => {
      const subject = harness(pageName), reading = subject.page.load()
      await flush()
      const old = subject.take('/me')
      await loginB(subject)
      old.reject(Object.assign(new Error('旧会话已失效'), { status: 401 }))
      await reading
      assert.equal(subject.storage.get('jijie_token'), 'token-B')
      assert.equal(subject.store.state.user.id, 'B')
    })
  }
  await t.test('oldPatch', async () => {
    const subject = harness('profile')
    Object.assign(subject.page.form, { nickname: '旧账号新昵称', bio: accountA.bio })
    const saving = subject.page.saveProfile(), old = subject.take('/me/profile', 'PATCH')
    await loginB(subject)
    old.resolve({ ...accountA, nickname: '旧账号新昵称' })
    await saving
    assert.equal(subject.store.state.user.id, 'B')
    assert.equal(subject.storage.get('jijie_token'), 'token-B')
    assert.equal(subject.notices.length, 0)
  })
  await t.test('hiddenStats', async () => {
    const subject = harness(), reading = subject.page.load()
    await flush()
    subject.take('/me').resolve({ ...accountA })
    await flush()
    const old = subject.take('/me/stats')
    const oldWallet=subject.take('/wallet')
    subject.page.hidePage()
    old.resolve({ buyer_order_count: 999 })
    oldWallet.resolve({available_cents:99900})
    await reading
    assert.equal(Object.keys(subject.page.stats.value).length, 0)
    assert.equal(subject.page.wallet.value,null)
  })
  await t.test('logoutBeforeOldRead', async () => {
    const subject = harness('profile'), reading = subject.page.load()
    await flush()
    const old = subject.take('/me'), signingOut = subject.page.signOut()
    subject.take('/auth/logout', 'POST').resolve({})
    await signingOut
    old.resolve({ ...accountA })
    await reading
    assert.equal(subject.store.state.user, null)
    assert.equal(subject.storage.has('jijie_token'), false)
    assert.equal(subject.page.form.nickname, '')
    assert.equal(subject.navigations.at(-1), '/pages/main/index?tab=me')
  })
})

test('个人中心钱包分区与订单角标读取真实数据，充值不混入可提现余额',async()=>{
  const subject=harness(),reading=subject.page.load();await flush();subject.take('/me').resolve({...accountA});await flush()
  subject.take('/me/stats').resolve({buyer_pending_payment_count:2,buyer_working_count:120,buyer_delivered_count:0,buyer_refund_count:-1})
  subject.take('/wallet').resolve({available_cents:1000,recharge_cents:6000,ongoing_cents:2500,settling_cents:3000,withdrawal_frozen_cents:500})
  await reading
  assert.equal(subject.page.walletAmount('available_cents'),'¥10.00');assert.equal(subject.page.walletAmount('recharge_cents'),'¥60.00');assert.equal(subject.page.walletAmount('ongoing_cents'),'¥25.00')
  assert.equal(subject.page.orderCount('buyer_pending_payment_count'),'2');assert.equal(subject.page.orderCount('buyer_working_count'),'99+');assert.equal(subject.page.orderCount('buyer_delivered_count'),'');assert.equal(subject.page.orderCount('buyer_refund_count'),'')
})

test('旧钱包迟到不能暴露给新账号，资金失败保持未知而不假报零',async t=>{
  await t.test('oldWallet',async()=>{
    const subject=harness(),reading=subject.page.load();await flush();subject.take('/me').resolve({...accountA});await flush()
    const stats=subject.take('/me/stats'),wallet=subject.take('/wallet');await loginB(subject)
    stats.resolve({buyer_pending_payment_count:999});wallet.resolve({available_cents:99900});await reading
    assert.equal(subject.store.state.user.id,'B');assert.equal(subject.page.wallet.value,null);assert.equal(subject.page.orderCount('buyer_pending_payment_count'),'')
  })
  await t.test('walletFailure',async()=>{
    const subject=harness(),reading=subject.page.load();await flush();subject.take('/me').resolve({...accountA});await flush()
    subject.take('/me/stats').resolve({buyer_pending_payment_count:2});subject.take('/wallet').reject(new Error('钱包连接失败'));await reading
    assert.equal(subject.page.walletAmount('available_cents'),'—');assert.equal(subject.page.orderCount('buyer_pending_payment_count'),'2');assert.match(subject.page.walletError.value,/暂未同步/)
  })
})

test('微信预登录票据不修改身份，只有同意协议后的登录建立会话', async () => {
  const subject = harness(), revision = subject.store.sessionRevision()
  const preparing = subject.store.prepareWechatLogin('wechat-code')
  const request = subject.take('/auth/wechat/prepare', 'POST')
  assert.equal(request.body.code, 'wechat-code')
  assert.equal(subject.store.sessionRevision(), revision)
  assert.equal(subject.storage.get('jijie_token'), 'token-A')
  const agreements = { user_agreement: { version: 'terms-v2' }, privacy_policy: { version: 'privacy-v3' } }
  request.resolve({ ticket: 'one-use-ticket', expires_in: 300, agreements })
  const prepared = await preparing
  assert.equal(prepared.ticket, 'one-use-ticket')
  assert.equal(subject.store.state.user.id, 'A')
  assert.equal(subject.store.sessionRevision(), revision)
  const signingIn = subject.store.wechatLogin(prepared.ticket, { agreed: true, user_agreement_version: 'terms-v2', privacy_policy_version: 'privacy-v3' })
  const loginRequest = subject.take('/auth/wechat', 'POST')
  assert.equal(loginRequest.body.ticket, 'one-use-ticket')
  assert.equal(loginRequest.body.agreed, true)
  assert.equal(loginRequest.body.user_agreement_version, 'terms-v2')
  assert.equal(loginRequest.body.privacy_policy_version, 'privacy-v3')
  loginRequest.resolve({ token: 'token-B', user: { ...accountB } })
  await signingIn
  assert.equal(subject.storage.get('jijie_token'), 'token-B')
  assert.equal(subject.store.state.user.id, 'B')
})

test('迟到的微信登录响应不能覆盖后来完成的身份切换', async () => {
  const subject = harness()
  const pending = subject.store.wechatLogin('old-ticket', { agreed: true, user_agreement_version: 'terms-v1', privacy_policy_version: 'privacy-v1' })
  const old = subject.take('/auth/wechat', 'POST')
  await loginB(subject)
  old.resolve({ token: 'old-wechat-token', user: { ...accountA } })
  const current = await pending
  assert.equal(current.id, 'B')
  assert.equal(subject.storage.get('jijie_token'), 'token-B')
  assert.equal(subject.store.state.user.id, 'B')
})

test('订单当前401切回可登录空态，旧账号响应不影响新账号', async t => {
  await t.test('expiredSession', async () => {
    const subject = harness('orders')
    const initial = subject.page.load()
    await flush()
    subject.take('/orders?role=buyer&status=all&limit=20&offset=0').resolve(Array.from({length:20},(_,index)=>({id:'old-order-'+index})))
    await initial
    assert.equal(subject.page.hasMore.value,true)
    const reading = subject.page.load(true)
    await flush()
    subject.take('/orders?role=buyer&status=all&limit=20&offset=20').reject(Object.assign(new Error('请先登录'), { status: 401 }))
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


test('首次资料弹层依据服务端注册标记，旧响应和退出不会复活标记',async()=>{
  const subject=harness()
  const old=subject.store.wechatLogin('old-ticket',{agreed:true}),oldRequest=subject.take('/auth/wechat','POST')
  const newer=subject.store.wechatLogin('new-ticket',{agreed:true});subject.take('/auth/wechat','POST').resolve({token:'token-new',user:{...accountB},first_login:false});await newer
  oldRequest.resolve({token:'old-token',user:{...accountA},first_login:true});await old
  assert.equal(subject.store.lastLoginWasNew.value,false)
  const registering=subject.store.wechatLogin('register-ticket',{agreed:true});subject.take('/auth/wechat','POST').resolve({token:'register-token',user:{...accountA},first_login:true});await registering
  assert.equal(subject.store.lastLoginWasNew.value,true)
  subject.store.clearSession();assert.equal(subject.store.lastLoginWasNew.value,false)
})
