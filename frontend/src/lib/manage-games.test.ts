/** 文件作用：运行游戏管理真实页面脚本，验证独立编辑草稿、官方选图生命周期和跨账号保存隔离。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { ref, shallowRef, reactive, computed, watch, effectScope, nextTick } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, DeferredUpload, PageLifecycle } from './sfc-test'
import type { Game } from '../types/models'

const storeSource = readFileSync(new URL('./store.ts', import.meta.url), 'utf8'), resourceSource = readFileSync(new URL('./manage-resource.ts', import.meta.url), 'utf8')
const adminA = { id: 'A', role: 'admin', nickname: '管理员A' }, adminB = { id: 'B', role: 'admin', nickname: '管理员B' }
const gameA: Game = { id: 'delta', name: '三角洲行动', short_name: '三角洲', subtitle: '组队出发', primary_color: '#83e7b7', bg_color: '#0a1110', surface_color: '#15201e', text_color: '#f2f5f3', icon_url: '/delta-icon.png', cover_url: '/delta-cover.png', covers: ['/delta-cover.png'], enabled: true, sort_order: 1, categories: ['组队陪玩'] }

/** 用实际业务API和Vue状态搭建游戏编辑环境，只替换网络与官方图片选择边界。 */
function harness(role = 'admin') {
  const storage = new Map([['jijie_token', 'token-A']]), requests: DeferredRequest[] = [], uploads: DeferredUpload[] = [], notices: unknown[] = [], scope = effectScope()
  const lifecycle: PageLifecycle = { load: () => {}, show: () => {}, hide: () => {}, unload: () => {} }
  const context = vm.createContext({ ref, shallowRef, reactive, computed, watch, nextTick,
    uni: { getStorageSync: (key: string) => storage.get(key), setStorageSync: (key: string, value: string) => storage.set(key, value), removeStorageSync: (key: string) => storage.delete(key) },
    token: () => storage.get('jijie_token') || '', notify: (value: unknown) => notices.push(value),
    /** 记录页面实际发送的请求及独立提交快照。 */
    api(path: string, method = 'GET', body: Record<string, unknown> = {}) { return new Promise<unknown>((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) },
    /** 原生选择完成时机由测试控制，覆盖先隐藏后返回两种时序。 */
    chooseUploadImage(purpose: string, options?: object) { return new Promise<string>((resolve, reject) => uploads.push({ purpose, options, resolve, reject })) },
    onShow: (callback: () => void) => { lifecycle.show = callback }, onHide: (callback: () => void) => { lifecycle.hide = callback }, onUnload: (callback: () => void) => { lifecycle.unload = callback },
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource) + '\nglobalThis.store = { state, login, sessionRevision }', context)
  context.store.state.user = { ...adminA, role }; context.store.state.ready = true; context.store.state.games = [structuredClone(gameA)]
  scope.run(() => vm.runInContext(executable(resourceSource) + '\n' + executable(pageScript('manage/games.vue')) + '\nglobalThis.page = { editGame, closeEditor, toggleCover, uploadCover, saveGame, gameForm, editing, newGame, saving, uploading, games, loadData, showPage }', context))
  return { page: context.page, store: context.store, context, requests, uploads, notices, lifecycle, scope,
    /** 取最新的同路径请求，账号轮换时更早请求仍保留以便主动验证迟到。 */
    take(path: string, method = 'GET') { const request = [...requests].reverse().find(item => !item.taken && item.path === path && item.method === method); assert.ok(request, `缺少请求 ${method} ${path}`); request.taken = true; return request },
  }
}
/** 将当前await推进到下一网络边界。 */
async function flush(): Promise<void> { await new Promise(resolve => setImmediate(resolve)) }
/** 加载真实游戏列表并等待页面可编辑。 */
async function loadGames(subject: ReturnType<typeof harness>): Promise<void> { const showing = subject.page.showPage(); await flush(); subject.take('/admin/games?limit=20&offset=0').resolve([structuredClone(gameA)]); await showing }
/** 从真实登录流程切换管理员。 */
async function loginB(subject: ReturnType<typeof harness>): Promise<void> { const pending = subject.store.login('B'); subject.take('/auth/dev', 'POST').resolve({ token: 'token-B', user: { ...adminB } }); await pending; await flush() }

test('编辑游戏创建独立快照，取消和封面排序不改变共享游戏', async () => {
  const subject = harness(); await loadGames(subject); subject.page.editGame(subject.page.games.value[0])
  subject.page.gameForm.name = '未保存游戏名'; subject.page.toggleCover('/second.png'); subject.page.toggleCover('/delta-cover.png')
  assert.deepEqual(Array.from(subject.page.gameForm.covers), ['/second.png']); assert.equal(subject.store.state.games[0].name, gameA.name); assert.deepEqual(Array.from(subject.store.state.games[0].covers), gameA.covers)
  subject.page.closeEditor(); assert.equal(subject.page.editing.value, false); subject.scope.stop()
})

test('无效颜色、分类或排序阻止发送游戏写请求', async () => {
  const subject = harness(); await loadGames(subject); subject.page.editGame(subject.page.games.value[0])
  for (const patch of [{ primary_color: 'green' }, { categoriesText: ' ,， ' }, { sort_order: '1.5' }]) {
    subject.page.editGame(subject.page.games.value[0]); Object.assign(subject.page.gameForm, patch); await subject.page.saveGame()
  }
  assert.equal(subject.requests.filter(item => item.method !== 'GET').length, 0); assert.equal(subject.notices.length, 3); subject.scope.stop()
})

test('创建游戏保留全部主题字段并使用第一张封面，刷新真实平台主题', async () => {
  const subject = harness(); await loadGames(subject); subject.page.editGame()
  Object.assign(subject.page.gameForm, { ...structuredClone(gameA), id: 'new-game', name: ' 新游戏 ', short_name: ' 新游戏 ', categoriesText: '陪玩，教学,陪玩', covers: ['/first.png', '/second.png'], sort_order: '9', enabled: false })
  const saving = subject.page.saveGame(), write = subject.take('/admin/games', 'POST')
  assert.equal(write.body.cover_url, '/first.png'); assert.equal(write.body.sort_order, 9); assert.equal(write.body.enabled, false); assert.equal(write.body.name, '新游戏'); assert.equal('categoriesText' in write.body, false)
  assert.deepEqual(Array.from(write.body.categories as string[]), ['陪玩', '教学']); assert.equal(write.body.icon_url, gameA.icon_url)
  for (const key of ['primary_color', 'bg_color', 'surface_color', 'text_color'] as const) assert.equal(write.body[key], gameA[key])
  write.resolve(write.body); await flush(); subject.take('/bootstrap').resolve({ games: [write.body], dev_mode: true }); await flush(); subject.take('/me').resolve({ ...adminA }); await flush(); subject.take('/admin/games?limit=20&offset=0').resolve([write.body]); await saving
  assert.equal(subject.page.editing.value, false); assert.equal(subject.store.state.games[0].id, 'new-game'); assert.equal(subject.notices.at(-1), '游戏主题已更新'); subject.scope.stop()
})

test('原生图标和封面上传隐藏再显示均保留草稿，并恢复后台列表', async t => {
  for (const kind of ['icon', 'cover']) for (const showBeforeUpload of [true, false]) await t.test(kind + '-' + String(showBeforeUpload), async () => {
    const subject = harness(); await loadGames(subject); subject.page.editGame(subject.page.games.value[0]); subject.page.gameForm.subtitle = '尚未保存的标语'
    const uploading = subject.page.uploadCover(kind); assert.equal(subject.uploads[0].purpose, kind === 'icon' ? 'game_icon' : 'game_cover'); subject.lifecycle.hide()
    if (showBeforeUpload) { subject.lifecycle.show(); await flush() }
    subject.uploads[0].resolve('/uploaded-' + kind + '.png'); await uploading
    if (!showBeforeUpload) { subject.lifecycle.show(); await flush() }
    subject.take('/admin/games?limit=20&offset=0').resolve([structuredClone(gameA)]); await flush()
    assert.equal(subject.page.editing.value, true); assert.equal(subject.page.gameForm.subtitle, '尚未保存的标语'); assert.equal(subject.page.games.value.length, 1)
    if (kind === 'icon') assert.equal(subject.page.gameForm.icon_url, '/uploaded-icon.png'); else assert.ok(subject.page.gameForm.covers.includes('/uploaded-cover.png'))
    assert.equal(subject.page.uploading.value, false); subject.scope.stop()
  })
})

test('跨管理员迟到上传不能回填新游戏草稿或发出成功提示', async () => {
  const subject = harness(); await loadGames(subject); subject.page.editGame(subject.page.games.value[0]); const uploading = subject.page.uploadCover('cover'); subject.lifecycle.hide()
  await loginB(subject); subject.lifecycle.show(); await flush(); subject.take('/admin/games?limit=20&offset=0').resolve([structuredClone(gameA)]); await flush(); subject.page.editGame(); subject.page.gameForm.name = 'B的草稿'
  subject.uploads[0].resolve('/A-stale.png'); await uploading
  assert.equal(subject.page.gameForm.name, 'B的草稿'); assert.ok(!subject.page.gameForm.covers.includes('/A-stale.png')); assert.equal(subject.notices.length, 0); subject.scope.stop()
})

test('跨管理员迟到保存不能关闭新草稿或刷新旧身份主题', async () => {
  const subject = harness(); await loadGames(subject); subject.page.editGame(subject.page.games.value[0]); const saving = subject.page.saveGame(), old = subject.take('/admin/games/delta', 'PATCH')
  await loginB(subject); subject.take('/admin/games?limit=20&offset=0').resolve([structuredClone(gameA)]); await flush(); subject.page.editGame(); subject.page.gameForm.name = 'B的草稿'
  const count = subject.requests.length; old.resolve(structuredClone(gameA)); await saving
  assert.equal(subject.page.editing.value, true); assert.equal(subject.page.gameForm.name, 'B的草稿'); assert.equal(subject.requests.length, count); assert.equal(subject.notices.length, 0); subject.scope.stop()
})

test('选图取消不丢草稿，不显示失败或成功提示', async () => {
  const subject = harness(); await loadGames(subject); subject.page.editGame(subject.page.games.value[0]); subject.page.gameForm.subtitle = '保留草稿'
  const uploading = subject.page.uploadCover('cover'); subject.lifecycle.hide(); subject.lifecycle.show(); await flush(); subject.uploads[0].reject({ cancelled: true }); await uploading
  subject.take('/admin/games?limit=20&offset=0').resolve([structuredClone(gameA)]); await flush()
  assert.equal(subject.page.gameForm.subtitle, '保留草稿'); assert.equal(subject.page.editing.value, true); assert.equal(subject.notices.length, 0); subject.scope.stop()
})
