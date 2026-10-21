/** 文件作用：执行设置页真实脚本，验证俱乐部金额和图文校验、支付秘密隔离及原生选图时的资料草稿保护。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, DeferredUpload, PageLifecycle } from './sfc-test'
import type { CommunityBlock } from '../types/models'

const storeSource = readFileSync(new URL('./store.ts', import.meta.url), 'utf8')
const sources = { 'club-edit': pageScript('community/clubs/edit.vue'), 'admin-settings': pageScript('manage/settings.vue'), profile: pageScript('account/profile.vue') }
const accountA = { id: 'A', role: 'admin', nickname: '原始昵称', bio: '原始个人介绍', avatar: '/avatar-A.png', profile_cover: '/cover-A.png' }
const accountB = { id: 'B', role: 'admin', nickname: '另一位用户', bio: '另一位用户介绍', avatar: '/avatar-B.png', profile_cover: '/cover-B.png' }

/** 创建实际 Vue 状态，记录页面发出的请求及注册的生命周期。 */
function harness(pageName: keyof typeof sources) {
  const storage = new Map<string, string>([['jijie_token', 'token-A']]), requests: DeferredRequest[] = [], uploads: DeferredUpload[] = [], notices: unknown[] = [], navigations: string[] = []
  const lifecycle: PageLifecycle = { load: () => {}, show: () => {}, hide: () => {}, unload: () => {} }
  const context = vm.createContext({
    reactive, ref, computed,
    uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: string) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key) },
    token: () => storage.get('jijie_token') || '',
    /** 网络完成顺序由测试明确控制。 */
    api(path: string, method = 'GET', body: Record<string, unknown> = {}) { return new Promise<unknown>((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) },
    /** 上传模拟保留原生选图可能先隐藏再显示页面的时间窗口。 */
    chooseUploadImage(purpose: string, options?: object) { return new Promise<string>((resolve, reject) => uploads.push({ purpose, options, resolve, reject })) },
    notify: (value: unknown) => notices.push(value), go: (value: string) => navigations.push(value),
    onLoad: (callback: PageLifecycle['load']) => { lifecycle.load = callback }, onShow: (callback: () => void) => { lifecycle.show = callback }, onHide: (callback: () => void) => { lifecycle.hide = callback }, onUnload: (callback: () => void) => { lifecycle.unload = callback },
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource) + '\nglobalThis.store = { state, login, sessionRevision }', context)
  context.store.state.user = { ...accountA }
  context.store.state.ready = true
  const exposed = { 'club-edit': 'load, save, form, depositCents, canManage', 'admin-settings': 'load, savePayment, saveAgreements, payment, secrets, paymentStatus, agreements, loaded', profile: 'load, upload, form, uploading' }[pageName]
  vm.runInContext(executable(sources[pageName]) + '\nglobalThis.page = { ' + exposed + ' }', context)
  return {
    store: context.store, page: context.page, storage, requests, uploads, notices, navigations, lifecycle,
    /** 取得确实发出的单个请求，重复或缺失调用会直接失败。 */
    take(path: string, method = 'GET') {
      const request = requests.find(item => !item.taken && item.path === path && item.method === method)
      assert.ok(request, `缺少请求 ${method} ${path}`)
      request.taken = true
      return request
    },
  }
}

/** 让当前 await 推进到下一次受控网络请求。 */
async function flush() { await new Promise(resolve => setImmediate(resolve)) }

/** 通过真实共享登录函数切换账号，使令牌和会话序号同时变化。 */
async function loginB(subject: ReturnType<typeof harness>) {
  const pending = subject.store.login('B')
  subject.take('/auth/dev', 'POST').resolve({ token: 'token-B', user: { ...accountB } })
  await pending
}

/** 提供内容有效的两份协议，方便区分旧管理员和新管理员的响应。 */
function agreementData(label = 'A') {
  return Object.fromEntries(['user_agreement', 'privacy_policy'].map(key => [key, { title: label + '发布的完整协议', version: label + '-v1', content: label + '完整协议正文内容'.repeat(6), updated_at: '2026-10-18T00:00:00Z' }]))
}

/** 提供支付公开配置，秘密字段仅在特定测试主动注入。 */
function paymentData(label = 'A') {
  return { merchant_id: 'merchant-' + label, certificate_serial: 'serial-' + label, platform_public_key_id: 'key-' + label, notify_url: 'https://example.com/pay', refund_notify_url: 'https://example.com/refund', configured: true, storage_available: true, source: 'database', api_v3_key_set: true, merchant_private_key_set: true, platform_public_key_set: true }
}

/** 完成已开始的管理员配置读取，避免复制加载逻辑。 */
async function resolveAdmin(subject: ReturnType<typeof harness>, label = 'A', extraPayment: object = {}) {
  await flush()
  subject.take('/admin/agreements').resolve(agreementData(label))
  subject.take('/admin/payment-settings').resolve({ ...paymentData(label), ...extraPayment })
  await flush()
}

/** 建立已加载资料页，保证后续测试针对真实已保存快照。 */
async function loadProfile(subject: ReturnType<typeof harness>, user = accountA) {
  const pending = subject.page.load()
  await flush()
  subject.take('/me').resolve({ ...user })
  await pending
}

test('俱乐部押金按十进制分计算，并拒绝超额及非法小数', () => {
  const subject = harness('club-edit')
  for (const [input, expected] of [['', 0], ['0', 0], ['0.01', 1], ['1.1', 110], [' 8.20 ', 820], ['99999.99', 9999999], ['100000', 10000000], ['100000.00', 10000000]]) {
    assert.equal(subject.page.depositCents(input), expected, String(input))
  }
  for (const input of ['-1', '1.001', '100000.01', '999999', '1e2', 'NaN', '.1', 'Infinity', undefined, null]) assert.throws(() => subject.page.depositCents(input), /押金门槛/, String(input))
})

test('俱乐部空图文段落阻止保存，合法图文去除本地编号并保存准确金额', async () => {
  const subject = harness('club-edit')
  subject.lifecycle.load({ id: 'club-A' })
  const loading = subject.page.load()
  await flush()
  subject.take('/clubs/club-A').resolve({ can_manage: true, club: { name: '集结俱乐部', description: '这是完整的俱乐部介绍文字', game_ids: ['delta'], deposit_cents: 820, detail_blocks: [] } })
  await loading
  subject.page.form.detail_blocks = [{ key: 1, type: 'text', text: '  \n ' }]
  await subject.page.save()
  assert.match(String(subject.notices.at(-1)), /空段落可以删除/)
  assert.equal(subject.requests.filter(item => item.method === 'PATCH').length, 0)
  subject.page.form.detail_blocks = [{ key: 2, type: 'text', text: '  正式俱乐部介绍  ' }, { key: 3, type: 'image', image_url: '/uploads/club-detail.png' }]
  const saving = subject.page.save(), request = subject.take('/clubs/club-A', 'PATCH')
  const blocks = request.body.detail_blocks as CommunityBlock[]
  assert.equal(request.body.deposit_cents, 820)
  assert.equal(blocks[0].text, '正式俱乐部介绍')
  assert.equal(blocks[1].image_url, '/uploads/club-detail.png')
  assert.ok(blocks.every(block => !('key' in block)))
  request.resolve({})
  await saving
  assert.equal(subject.navigations.at(-1), '/pages/community/clubs/detail?id=club-A')
})

test('管理员只接收支付公开字段，隐藏页面立即清除秘密且不持久缓存', async () => {
  const subject = harness('admin-settings'), loading = subject.page.load()
  await resolveAdmin(subject, 'A', { api_v3_key: 'server-api-secret', merchant_private_key_pem: 'server-private-secret', platform_public_key_pem: 'server-public-secret', unexpected: 'ignored' })
  await loading
  assert.equal(subject.page.payment.merchant_id, 'merchant-A')
  assert.deepEqual(Object.keys(subject.page.payment).sort(), ['certificate_serial', 'merchant_id', 'notify_url', 'platform_public_key_id', 'refund_notify_url'])
  assert.ok(Object.values(subject.page.secrets).every(value => value === ''))
  assert.equal(subject.page.paymentStatus.api_v3_key_set, true)
  subject.page.payment.merchant_id = 'unsaved-public-draft'
  Object.assign(subject.page.secrets, { api_v3_key: 'a'.repeat(32), merchant_private_key_pem: 'private-draft', platform_public_key_pem: 'public-draft' })
  const requestCount = subject.requests.length
  subject.lifecycle.hide()
  assert.ok(Object.values(subject.page.secrets).every(value => value === ''))
  subject.lifecycle.show()
  await flush()
  assert.equal(subject.page.payment.merchant_id, 'unsaved-public-draft')
  assert.equal(subject.requests.length, requestCount)
  assert.deepEqual([...subject.storage.keys()], ['jijie_token'])
})

test('管理员跨身份迟到的配置读取不能写入新身份页面', async () => {
  const subject = harness('admin-settings'), loading = subject.page.load()
  await flush()
  const oldAgreements = subject.take('/admin/agreements'), oldPayment = subject.take('/admin/payment-settings')
  await loginB(subject)
  oldAgreements.resolve(agreementData('A'))
  oldPayment.resolve(paymentData('A'))
  await loading
  assert.equal(subject.page.loaded.value, false)
  assert.equal(subject.page.payment.merchant_id, '')
  assert.equal(subject.page.agreements.user_agreement.version, '')
  assert.equal(subject.store.state.user.id, 'B')
})

test('管理员跨身份迟到的支付或协议保存不能覆盖新配置', async t => {
  for (const kind of ['payment', 'agreements']) {
    await t.test(kind, async () => {
      const subject = harness('admin-settings'), loading = subject.page.load()
      await resolveAdmin(subject)
      await loading
      subject.page.payment.merchant_id = 'A-unsaved'
      subject.page.secrets.api_v3_key = 'a'.repeat(32)
      subject.page.agreements.user_agreement.version = 'A-v2'
      const saving = kind === 'payment' ? subject.page.savePayment() : subject.page.saveAgreements()
      const old = subject.take(kind === 'payment' ? '/admin/payment-settings' : '/admin/agreements', 'PATCH')
      if (kind === 'payment') assert.equal(old.body.api_v3_key, 'a'.repeat(32))
      await loginB(subject)
      subject.lifecycle.hide()
      subject.lifecycle.show()
      await resolveAdmin(subject, 'B')
      old.resolve(kind === 'payment' ? paymentData('old-A') : agreementData('old-A'))
      await saving
      assert.equal(subject.page.payment.merchant_id, 'merchant-B')
      assert.equal(subject.page.agreements.user_agreement.version, 'B-v1')
      assert.ok(Object.values(subject.page.secrets).every(value => value === ''))
      assert.equal(subject.notices.length, 0)
      if (kind === 'payment') assert.equal(old.body.api_v3_key, '')
    })
  }
})

test('原生选图隐藏再显示时保留未保存的昵称和简介', async t => {
  for (const timing of ['showBeforeUpload', 'showAfterUpload']) {
    await t.test(timing, async () => {
      const subject = harness('profile')
      await loadProfile(subject)
      Object.assign(subject.page.form, { nickname: '尚未保存昵称', bio: '尚未保存的个人介绍' })
      const requestCount = subject.requests.length, uploading = subject.page.upload('avatar')
      assert.equal(subject.uploads[0].purpose, 'avatar')
      subject.lifecycle.hide()
      if (timing === 'showBeforeUpload') subject.lifecycle.show()
      subject.uploads[0].resolve('/uploads/new-avatar.png')
      await uploading
      if (timing === 'showAfterUpload') subject.lifecycle.show()
      await flush()
      assert.equal(subject.page.form.nickname, '尚未保存昵称')
      assert.equal(subject.page.form.bio, '尚未保存的个人介绍')
      assert.equal(subject.page.form.avatar, '/uploads/new-avatar.png')
      assert.equal(subject.requests.length, requestCount)
      assert.equal(subject.store.state.user.avatar, accountA.avatar)
      assert.equal(subject.page.uploading.value, false)
    })
  }
})

test('原生选图迟到响应不能回填另一身份的资料草稿', async () => {
  const subject = harness('profile')
  await loadProfile(subject)
  const uploading = subject.page.upload('profile_cover')
  subject.lifecycle.hide()
  await loginB(subject)
  subject.lifecycle.show()
  await flush()
  subject.take('/me').resolve({ ...accountB })
  await flush()
  subject.uploads[0].resolve('/uploads/old-A-cover.png')
  await uploading
  assert.equal(subject.page.form.nickname, accountB.nickname)
  assert.equal(subject.page.form.bio, accountB.bio)
  assert.equal(subject.page.form.profile_cover, accountB.profile_cover)
  assert.equal(subject.store.state.user.id, 'B')
  assert.equal(subject.notices.length, 0)
})
