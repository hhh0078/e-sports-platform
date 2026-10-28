/** 文件作用：执行真实版本协调器，验证一次绑定、公告互斥、前后台、资金完整任务与原生失败恢复；不模拟真实微信下载或资金到账。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { executable } from './sfc-test'
import type { UpdateRuntime, WechatUpdateManager } from './update-manager'

/** 每个用例使用独立VM及受控事件循环，避免测试状态污染真实全局单例。 */
function harness(options: { unavailable?: boolean; modalThrows?: boolean; applyThrows?: boolean } = {}) {
  const timers = new Map<number, () => void>(), modals: Parameters<UpdateRuntime['showModal']>[0][] = [], toasts: string[] = []
  const callbacks = { check: [] as Parameters<WechatUpdateManager['onCheckForUpdate']>[0][], ready: [] as Array<() => void>, failed: [] as Array<() => void> }
  let nextTimer = 0, instances = 0, applies = 0
  const native: UpdateRuntime = {
    ...(options.unavailable ? {} : { getUpdateManager: () => { instances++; return { onCheckForUpdate: callback => callbacks.check.push(callback), onUpdateReady: callback => callbacks.ready.push(callback), onUpdateFailed: callback => callbacks.failed.push(callback), applyUpdate: () => { applies++; if (options.applyThrows) throw new Error('原生重启失败') } } as WechatUpdateManager } }),
    showModal: value => { if (options.modalThrows) throw new Error('原生弹框失败'); modals.push(value) }, showToast: value => { toasts.push(value.title) },
  }
  const context = vm.createContext({ native, setTimeout: (callback: () => void) => { timers.set(++nextTimer, callback); return nextTimer }, clearTimeout: (key: number) => timers.delete(key) })
  vm.runInContext(executable(readFileSync(new URL('./update-manager.ts', import.meta.url), 'utf8')) + '\nglobalThis.subject={initializeUpdateManager,setUpdateAppVisible,beginFinancialActivity,acquireSystemPrompt,getUpdatePromptState,onUpdatePromptStateChange,isFinancialRequest}', context)
  const subject = context.subject as typeof import('./update-manager')
  /** 执行真实调度队列，循环保护可以发现重复弹框或重启导致的无穷重试。 */
  function flush(): void { let loops = 0; while (timers.size) { assert.ok(loops++ < 30, '更新调度产生无穷重试'); const pending = [...timers.values()]; timers.clear(); for (const callback of pending) callback() } }
  subject.initializeUpdateManager(native); subject.setUpdateAppVisible(true); flush()
  return { subject, context, callbacks, modals, toasts, flush, applies: () => applies, instances: () => instances }
}

test('官方更新仅绑定一次，检查到版本不直接重启，ready只弹一个无取消确认', () => {
  const h = harness(); h.subject.initializeUpdateManager(); assert.equal(h.instances(), 1)
  assert.equal(h.callbacks.check.length, 1); assert.equal(h.callbacks.ready.length, 1); assert.equal(h.callbacks.failed.length, 1)
  h.callbacks.check[0]({ hasUpdate: true }); h.flush(); assert.equal(h.modals.length, 0); assert.equal(h.applies(), 0)
  h.callbacks.ready[0](); h.callbacks.ready[0](); h.flush(); assert.equal(h.modals.length, 1)
  assert.equal(h.modals[0].showCancel, false); assert.equal(h.modals[0].confirmText, '立即更新')
  h.modals[0].success({ confirm: true }); h.modals[0].success({ confirm: true }); h.flush(); assert.equal(h.applies(), 1)
  h.callbacks.ready[0](); h.flush(); assert.equal(h.modals.length, 1); assert.equal(h.applies(), 1)
})

test('资金busy期间ready延后，确认时新增资金任务也必须结束后才重启', () => {
  const h = harness(), finish = h.subject.beginFinancialActivity()
  h.callbacks.ready[0](); h.flush(); assert.equal(h.modals.length, 0); assert.equal(h.subject.getUpdatePromptState().deferred, true)
  finish(); finish(); h.flush(); assert.equal(h.modals.length, 1)
  const later = h.subject.beginFinancialActivity(); h.modals[0].success({ confirm: true }); h.flush(); assert.equal(h.applies(), 0)
  later(); h.flush(); assert.equal(h.applies(), 1); assert.equal(h.modals.length, 1)
})

test('身份强制核验在资金锁下可占空槽，核验期间微信更新不能抢占',()=>{
  const h=harness(),finish=h.subject.beginFinancialActivity(),release=h.subject.acquireSystemPrompt('identity');assert.ok(release)
  h.callbacks.ready[0]();h.flush();assert.equal(h.modals.length,0);assert.equal(h.subject.acquireSystemPrompt('announcement'),null)
  finish();h.flush();assert.equal(h.modals.length,0);assert.equal(h.subject.getUpdatePromptState().owner,'identity')
  release();release();h.flush();assert.equal(h.modals.length,1);assert.equal(h.subject.getUpdatePromptState().owner,'update')
})

test('现有公告先结束，ready优先于尚未打开公告，旧释放句柄不能关闭新的弹框槽', () => {
  const h = harness(), first = h.subject.acquireSystemPrompt('announcement'); assert.ok(first)
  h.callbacks.ready[0](); h.flush(); assert.equal(h.modals.length, 0)
  first(); assert.equal(h.subject.acquireSystemPrompt('announcement'), null); h.flush(); assert.equal(h.modals.length, 1)
  first(); assert.equal(h.subject.getUpdatePromptState().owner, 'update')
  h.modals[0].success({ confirm: false }); h.flush(); const second = h.subject.acquireSystemPrompt('announcement'); assert.ok(second)
  first(); assert.equal(h.subject.getUpdatePromptState().owner, 'announcement'); second()
})

test('后台不弹框不重启，隐藏不会清除资金任务，回前台才恢复安全检查', () => {
  const h = harness(), finish = h.subject.beginFinancialActivity(); h.subject.setUpdateAppVisible(false)
  h.callbacks.ready[0](); h.flush(); assert.equal(h.modals.length, 0); assert.equal(h.subject.getUpdatePromptState().financialBusy, true)
  h.subject.setUpdateAppVisible(true); h.flush(); assert.equal(h.modals.length, 0)
  finish(); h.flush(); assert.equal(h.modals.length, 1)
  h.subject.setUpdateAppVisible(false); h.modals[0].success({ confirm: true }); h.flush(); assert.equal(h.applies(), 0)
  h.subject.setUpdateAppVisible(true); h.flush(); assert.equal(h.applies(), 1)
})

test('下载、弹框和重启失败均只轻提示且释放业务协调，不产生循环弹框', () => {
  const download = harness(); download.callbacks.failed[0](); download.flush(); assert.equal(download.toasts.length, 1); assert.equal(download.subject.getUpdatePromptState().ready, false)
  assert.ok(download.subject.acquireSystemPrompt('announcement'))
  const modal = harness({ modalThrows: true }); modal.callbacks.ready[0](); modal.flush(); assert.equal(modal.subject.getUpdatePromptState().prompting, false); assert.equal(modal.subject.getUpdatePromptState().owner, null); assert.equal(modal.toasts.length, 1); assert.ok(modal.subject.acquireSystemPrompt('announcement'))
  const apply = harness({ applyThrows: true }); apply.callbacks.ready[0](); apply.flush(); apply.modals[0].success({ confirm: true }); apply.flush(); assert.equal(apply.subject.getUpdatePromptState().applying, false); assert.equal(apply.applies(), 1); assert.equal(apply.toasts.length, 1); assert.ok(apply.subject.acquireSystemPrompt('announcement'))
})

test('原生fail回调不释放别的资金任务，状态订阅卸载后不再收到变化', () => {
  const h = harness(); let changes = 0; const unsubscribe = h.subject.onUpdatePromptStateChange(() => changes++)
  const first = h.subject.beginFinancialActivity(), second = h.subject.beginFinancialActivity(); first(); first(); assert.equal(h.subject.getUpdatePromptState().financialBusy, true)
  unsubscribe(); const before = changes; second(); assert.equal(changes, before); assert.equal(h.subject.getUpdatePromptState().financialBusy, false)
  h.callbacks.ready[0](); h.flush(); h.modals[0].fail(); h.flush(); assert.equal(h.subject.getUpdatePromptState().owner, null)
  h.callbacks.ready[0](); h.flush(); assert.equal(h.modals.length, 1, '失败后本次前台不能反复弹框')
})

test('不支持更新能力仍可展示公告，后台下载失败轻提示在前台安全时才显示', () => {
  const old = harness({ unavailable: true }); assert.equal(old.instances(), 0); assert.ok(old.subject.acquireSystemPrompt('announcement'))
  const h = harness(); h.subject.setUpdateAppVisible(false); h.callbacks.failed[0](); h.flush(); assert.equal(h.toasts.length, 0)
  const finish = h.subject.beginFinancialActivity(); h.subject.setUpdateAppVisible(true); h.flush(); assert.equal(h.toasts.length, 0)
  finish(); h.flush(); assert.equal(h.toasts.length, 1)
})

test('真实微信付款及商家收款回调在完整任务期间维持忙碌，失败也会幂等释放', async () => {
  const h = harness(), payments: Array<{ success: () => void; fail: (error: unknown) => void }> = [], transfers: Array<{ success: () => void; fail: (error: unknown) => void }> = []
  h.context.uni = { requestPayment: (options: typeof payments[number]) => payments.push(options) }
  h.context.wx = { canIUse: () => true, requestMerchantTransfer: (options: typeof transfers[number]) => transfers.push(options) }
  vm.runInContext(executable(readFileSync(new URL('./payment.ts', import.meta.url), 'utf8')) + '\nglobalThis.payments={requestWechatPayment,confirmWechatTransfer}', h.context)
  const pay = h.context.payments.requestWechatPayment({}); assert.equal(h.subject.getUpdatePromptState().financialBusy, true)
  h.callbacks.ready[0](); h.flush(); assert.equal(h.modals.length, 0)
  const reconcile = h.subject.beginFinancialActivity(); payments[0].success(); await pay; h.flush(); assert.equal(h.modals.length, 0, '原生回调后原单查询尚未结束')
  reconcile(); h.flush(); assert.equal(h.modals.length, 1); h.modals[0].success({ confirm: false }); h.flush()
  const transfer = h.context.payments.confirmWechatTransfer({ mch_id: 'demo', app_id: 'demo', package_info: 'demo' }); assert.equal(h.subject.getUpdatePromptState().financialBusy, true)
  transfers[0].fail(new Error('取消官方收款')); await assert.rejects(transfer); assert.equal(h.subject.getUpdatePromptState().financialBusy, false)
})

test('原生付款与转账同步抛错不会遗留资金锁，能力不足也不锁业务',async()=>{
  const h=harness();h.context.uni={requestPayment:()=>{throw new Error('付款接口异常')}}
  h.context.wx={canIUse:()=>true,requestMerchantTransfer:()=>{throw new Error('转账接口异常')}}
  vm.runInContext(executable(readFileSync(new URL('./payment.ts',import.meta.url),'utf8'))+'\nglobalThis.payments={requestWechatPayment,confirmWechatTransfer}',h.context)
  await assert.rejects(h.context.payments.requestWechatPayment({}),/付款接口异常/);assert.equal(h.subject.getUpdatePromptState().financialBusy,false)
  await assert.rejects(h.context.payments.confirmWechatTransfer({}),/转账接口异常/);assert.equal(h.subject.getUpdatePromptState().financialBusy,false)
  h.context.wx={canIUse:()=>false};await assert.rejects(h.context.payments.confirmWechatTransfer({}),/新版微信/);assert.equal(h.subject.getUpdatePromptState().financialBusy,false)
  h.callbacks.ready[0]();h.flush();assert.equal(h.modals.length,1)
})

test('资金路由识别包含退款调价和审核转账，普通公开内容与聊天不会占资金槽', () => {
  const h = harness()
  for (const path of ['/wallet', '/wallet/deposit-refunds/x/sync', '/clubs/a/withdrawals', '/admin/wallet/withdrawals/x/execute', '/orders/a/pay', '/orders/a/adjustments/x/respond', '/orders/a/actions', '/admin/orders/a/resolve']) assert.equal(h.subject.isFinancialRequest(path, 'POST'), true, path)
  for (const path of ['/games', '/articles/a', '/orders/a/messages', '/admin/users', '/clubs/a/members']) assert.equal(h.subject.isFinancialRequest(path, 'GET'), false, path)
  assert.equal(h.subject.isFinancialRequest('/orders', 'POST'), true)
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8'); assert.match(source, /#ifdef MP-WEIXIN\s*\n\s*initializeUpdateManager\(\)/)
})
