/** 文件作用：执行钱包管理页真实脚本，验证来源审核入口、分页隔离和公开转账配置边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, watch } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'
import type { WalletSettingsResult, WalletWithdrawal } from '../types/wallet'

/** 建立真实脚本和业务API的受控边界，不复制页面资金逻辑。 */
function harness(devMode = false, realGestures = false) {
  const requests: DeferredRequest[] = [], notices: unknown[] = [], routes:string[]=[]
  const state = reactive({ ready: true, devMode, user: { id: 'admin-A', role: 'admin' } })
  let authToken = 'token-A', revision = 1, bottom=()=>{}
  const lifecycle = { show: () => {}, hide: () => {}, unload: () => {} }
  const context = vm.createContext({ reactive, ref, watch, state, onReachBottom:(callback:()=>void)=>{bottom=callback}, game: { primary_color: '#83e7b7' }, token: () => authToken, sessionRevision: () => revision, init: async () => {}, notify: (value: unknown) => notices.push(value), go: (url:string) => routes.push(url), onShow: (callback: () => void) => { lifecycle.show = callback }, onHide: (callback: () => void) => { lifecycle.hide = callback }, onUnload: (callback: () => void) => { lifecycle.unload = callback }, api(path: string, method = 'GET', body: Record<string, unknown> = {}) { return new Promise<unknown>((resolve, reject) => requests.push({ path, method, body, resolve, reject, taken: false })) } })
  injectBusinessAPIs(context)
  if(realGestures)vm.runInContext(executable(readFileSync(new URL('./native-page-refresh.ts',import.meta.url),'utf8')),context)
  vm.runInContext(executable(pageScript('manage/wallet.vue')) + '\nglobalThis.page = { showPage, refreshPage, retryFailed, retrySettingsFailed, loadList, loadSettings, chooseStatus, openAudit, settingsBody, saveSettings, form, withdrawals, loaded, transferReady }', context)
  return { page: context.page, requests, notices, routes, lifecycle,
    /** 触发页面实际注册的原生触底，验证错误恢复路径与HTTP分页一致。 */
    bottom(){bottom()},
    /** 取得确实发出的请求，缺失与重复读取立即失败。 */
    take(path: string, method = 'GET') { const request = requests.find(item => !item.taken && item.path === path && item.method === method); assert.ok(request, `缺少请求 ${method} ${path}`); request.taken = true; return request },
    /** 在隐藏页面后更换管理员，认证版本与令牌同时变化。 */
    switchAdmin() { lifecycle.hide(); authToken = 'token-B'; revision++; state.user = { id: 'admin-B', role: 'admin' } },
  }
}

/** 推进到明确发出的下一次异步请求。 */
async function flush() { await new Promise(resolve => setImmediate(resolve)) }
/** 创建不含任何秘密的真实配置返回结构。 */
function configuration(label = 'A'): WalletSettingsResult { return { settings: { withdrawal_enabled: true, transfer_scene_id: label, transfer_scene_report_infos: [{ info_type: '业务类型', info_content: '游戏服务收入' }], transfer_notify_url: 'https://example.com/transfer', transfer_remark: '服务收入提现', user_recv_perception: '服务报酬' }, transfer_ready: true, configuration_notice: '保存不代表场景获批' } }
/** 创建小额申请，不编造昵称、OpenID或收款确认凭据。 */
function withdrawal(id = 'request-A', status: WalletWithdrawal['status'] = 'pending_review'): WalletWithdrawal { return { id, user_id: 'user-' + id, amount_cents: 100, status, created_at: '2026-10-22T00:00:00Z', updated_at: '2026-10-22T00:00:00Z' } }
/** 完成一次已发出的列表和配置请求。 */
async function resolveLoad(subject: ReturnType<typeof harness>, label = 'A') { await flush(); subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve([withdrawal('request-' + label)]); subject.take('/admin/wallet-settings').resolve(configuration(label)); await flush() }
/** 进入已经读取真实公开配置的管理页面。 */
async function ready(devMode = false) { const subject = harness(devMode), loading = subject.page.showPage(); await resolveLoad(subject); await loading; return subject }

test('提现筛选迟到的响应不能覆盖新状态列表', async () => {
  const subject = await ready(), old = subject.page.loadList()
  const first = subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0')
  subject.page.chooseStatus('processing')
  const latest = subject.take('/admin/wallet/withdrawals?status=processing&limit=20&offset=0')
  latest.resolve([withdrawal('processing-B', 'processing')]); await flush()
  first.resolve([withdrawal('old-A')]); await old
  assert.equal(subject.page.withdrawals.value[0].id, 'processing-B')
})

test('列表去重后仍按服务端已读取条数分页，不重复读取旧偏移', async () => {
  const subject = harness(), loading = subject.page.showPage(); await flush()
  subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve(Array.from({ length: 20 }, (_, index) => withdrawal('r' + index)))
  subject.take('/admin/wallet-settings').resolve(configuration()); await loading
  const appending = subject.page.loadList(true)
  subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=20').resolve(Array.from({ length: 20 }, (_, index) => withdrawal('r' + (index + 10))))
  await appending; assert.equal(subject.page.withdrawals.value.length, 30)
  const next = subject.page.loadList(true)
  subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=40').resolve([]); await next
})

test('更换管理员后旧列表与配置不能覆盖新会话', async () => {
  const subject = harness(), oldLoading = subject.page.showPage(); await flush()
  const oldList = subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0'), oldConfig = subject.take('/admin/wallet-settings')
  subject.switchAdmin(); const newLoading = subject.page.showPage(); await resolveLoad(subject, 'B'); await newLoading
  oldList.resolve([withdrawal('old-A')]); oldConfig.resolve(configuration('old-A')); await oldLoading
  assert.equal(subject.page.withdrawals.value[0].id, 'request-B')
  assert.equal(subject.page.form.transfer_scene_id, 'B')
})

test('旧提现列表只能跳转来源审核，不再直接转账或拒绝', async () => {
  const subject=await ready()
  subject.page.openAudit('request/A')
  assert.equal(subject.routes[0],'/pages/manage/finance/detail?id=request%2FA')
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
  subject.lifecycle.hide();subject.page.openAudit('hidden')
  assert.equal(subject.routes.length,1)
})

test('公开配置严格限制字段、金额外参数和报备数量，不把秘密带入保存', async () => {
  const subject = await ready()
  assert.equal(Object.keys(subject.page.form).length, 6)
  subject.page.form.transfer_notify_url = 'https://example.com/notify?token=123'; assert.throws(() => subject.page.settingsBody(), /HTTPS/)
  subject.page.form.transfer_notify_url = 'https://example.com/notify'; subject.page.form.user_recv_perception = '字'.repeat(11); assert.throws(() => subject.page.settingsBody(), /最多10字/)
  subject.page.form.user_recv_perception = '服务报酬'; subject.page.form.transfer_scene_report_infos = Array.from({ length: 11 }, () => ({ info_type: '类型', info_content: '内容' })); assert.throws(() => subject.page.settingsBody(), /最多10项/)
  subject.page.form.transfer_scene_report_infos = [{ info_type: '', info_content: '内容' }]; assert.throws(() => subject.page.settingsBody(), /1—15字/)
  subject.page.form.transfer_scene_report_infos = [{ info_type: '类'.repeat(16), info_content: '内容' }]; assert.throws(() => subject.page.settingsBody(), /1—15字/); subject.page.form.transfer_scene_id='a'.repeat(36); subject.page.form.transfer_scene_report_infos = [{ info_type: ' 类型 ', info_content: ' 内容 ' }]
  const saving = subject.page.saveSettings(), request = subject.take('/admin/wallet-settings', 'PATCH')
  assert.deepEqual(Object.keys(request.body).sort(), Object.keys(configuration().settings).sort())
  assert.equal((request.body.transfer_scene_report_infos as Array<{info_type:string}>)[0].info_type, '类型')
  request.resolve(configuration()); await saving
})

test('旧管理员迟到的配置保存不会污染新管理员草稿', async () => {
  const subject = await ready(); subject.page.form.transfer_scene_id = 'unsaved-A'
  const saving = subject.page.saveSettings(), request = subject.take('/admin/wallet-settings', 'PATCH')
  subject.switchAdmin(); const newLoading = subject.page.showPage(); await resolveLoad(subject, 'B'); await newLoading
  request.resolve(configuration('old-A')); await saving
  assert.equal(subject.page.form.transfer_scene_id, 'B')
  assert.equal(subject.notices.length, 0)
})


test('钱包管理下拉只GET列表和通道状态，绝不覆盖未保存配置草稿',async()=>{
  const s=await ready();s.page.form.transfer_scene_id='未保存的场景';s.page.form.transfer_remark='未保存的备注';const before=s.requests.length
  const refreshing=s.page.refreshPage();await flush()
  s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve([withdrawal('new-record')])
  s.take('/admin/wallet-settings').resolve({...configuration('server-changed'),transfer_ready:false});await refreshing
  assert.equal(s.page.form.transfer_scene_id,'未保存的场景');assert.equal(s.page.form.transfer_remark,'未保存的备注');assert.equal(s.page.transferReady.value,false);assert.equal(s.page.withdrawals.value[0].id,'new-record')
  assert.ok(s.requests.slice(before).every(item=>item.method==='GET'))
})

test('管理钱包刷新失败且旧列表仍满页时，重试刷新不跳offset20',async()=>{
  const s=harness(),initial=s.page.showPage();await flush();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>withdrawal('old-'+i)));s.take('/admin/wallet-settings').resolve(configuration());await initial
  const refresh=s.page.refreshPage();await flush();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').reject(new Error('刷新失败'));s.take('/admin/wallet-settings').resolve(configuration());await refresh
  const retry=s.page.retryFailed();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>withdrawal('new-'+i)));await retry
  assert.equal(s.requests.some(item=>item.path.includes('offset=20')),false)
  const more=s.page.loadList(true);s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=20').reject(new Error('追加失败'));await more
  const again=s.page.retryFailed();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=20').resolve([]);await again
  assert.ok(s.requests.every(item=>item.method==='GET'))
})

test('管理钱包下拉配置GET失败后的重试也保留未保存草稿',async()=>{
  const s=await ready();s.page.form.transfer_scene_id='未保存的场景';const refresh=s.page.refreshPage();await flush();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve([]);s.take('/admin/wallet-settings').reject(new Error('状态读取失败'));await refresh
  const retry=s.page.retrySettingsFailed();s.take('/admin/wallet-settings').resolve(configuration('server'));await retry
  assert.equal(s.page.form.transfer_scene_id,'未保存的场景');assert.ok(s.requests.every(item=>item.method==='GET'))
})

test('管理钱包原生触底在刷新和追加失败时沿原offset重试',async()=>{const s=harness(false,true),initial=s.page.showPage();await flush();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>withdrawal('r-'+i)));s.take('/admin/wallet-settings').resolve(configuration());await initial;const refresh=s.page.refreshPage();await flush();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').reject(new Error('刷新失败'));s.take('/admin/wallet-settings').resolve(configuration());await refresh;s.bottom();await flush();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>withdrawal('n-'+i)));await flush();const more=s.page.loadList(true);s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=20').reject(new Error('追加失败'));await more;s.bottom();await flush();s.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=20').resolve([]);await flush();assert.ok(s.requests.every(item=>item.method==='GET'))})
