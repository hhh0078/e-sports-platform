/** 文件作用：绑定一次微信官方版本更新，协调公告与更新原生弹框，并在资金完整业务任务结束后才允许重启。 */

/** 微信官方更新实例的最小能力边界，不依赖H5中不存在的微信全局。 */
export interface WechatUpdateManager {
  onCheckForUpdate(callback: (result: { hasUpdate: boolean }) => void): void
  onUpdateReady(callback: () => void): void
  onUpdateFailed(callback: () => void): void
  applyUpdate(): void
}
/** 可注入的原生能力用于离线测试；生产使用uni对应的官方小程序接口。 */
export interface UpdateRuntime {
  getUpdateManager?: () => WechatUpdateManager
  showModal(options: { title: string; content: string; showCancel: false; confirmText: string; success: (result: { confirm?: boolean }) => void; fail: () => void }): unknown
  showToast?(options: { title: string; icon: 'none'; duration: number }): unknown
}
/** 公告和更新共享唯一系统弹框槽。 */
export type SystemPromptKind = 'announcement' | 'update'
/** 外部只读状态不会暴露支付参数、业务号或用户资料。 */
export interface UpdatePromptState { ready: boolean; prompting: boolean; applying: boolean; deferred: boolean; financialBusy: boolean; background: boolean; owner: SystemPromptKind | null }

const financialTasks = new Set<symbol>(), listeners = new Set<() => void>()
let initialized = false, visible = false, ready = false, prompting = false, applying = false, consented = false, suspended = false
let promptOwner: SystemPromptKind | null = null, promptLease = 0
let runtime: UpdateRuntime | undefined, manager: WechatUpdateManager | undefined, timer: ReturnType<typeof setTimeout> | undefined, failureNotice = ''

/** 调度到下一事件循环，资金回调和原单查单微任务之间不插入更新或重启。 */
function scheduleUpdate(): void {
  for (const listener of listeners) { try { listener() } catch { /* 观察者异常不能阻塞资金或更新状态。 */ } }
  if (!initialized || !visible || timer !== undefined) return
  timer = setTimeout(() => { timer = undefined; progressUpdate() }, 0)
}

/**
 * 保护一次完整资金任务；返回幂等释放函数，必须在整个创建、原生交互及回查finally中释放。
 * @returns 任务结束句柄；隐藏、切号或组件销毁不能代替请求完成而提前释放。
 */
export function beginFinancialActivity(): () => void {
  const lease = Symbol('financial-operation'); financialTasks.add(lease); scheduleUpdate()
  let released = false
  return () => { if (released) return; released = true; financialTasks.delete(lease); scheduleUpdate() }
}

/**
 * 抢占唯一公告／更新系统弹框槽，更新准备好时优先于尚未打开的公告。
 * @param kind - 可信组件声明的弹框类型。
 * @returns 释放句柄或null；关闭、隐藏和卸载都应幂等释放公告句柄。
 */
export function acquireSystemPrompt(kind: SystemPromptKind): (() => void) | null {
  if (!visible || promptOwner || financialTasks.size || applying || kind === 'announcement' && ready && !suspended) return null
  const lease = ++promptLease; promptOwner = kind; scheduleUpdate()
  let released = false
  return () => { if (released) return; released = true; if (lease === promptLease) { promptOwner = null; scheduleUpdate() } }
}

/** 返回不可变快照，公告组件可据此观察优先级，不能修改更新或资金状态。 */
export function getUpdatePromptState(): UpdatePromptState {
  return { ready, prompting, applying, deferred: ready && !prompting && !applying, financialBusy: financialTasks.size > 0, background: !visible, owner: promptOwner }
}

/** 订阅轻状态变化，供公告等待共享弹框槽释放；返回可在卸载时调用的清理句柄。 */
export function onUpdatePromptStateChange(callback: () => void): () => void { listeners.add(callback); return () => { listeners.delete(callback) } }

/** 原生能力异常只排队轻提示，不更改支付记录、清除原请求或锁住交易。 */
function updateFailed(message: string): void { failureNotice = message; scheduleUpdate() }

/** 当前安全时只执行一次更新步骤，下载和原生确认不与在途资金或其它弹框重叠。 */
function progressUpdate(): void {
  if (!visible || financialTasks.size) return
  if (failureNotice) { const title = failureNotice; failureNotice = ''; try { runtime?.showToast?.({ title, icon: 'none', duration: 3000 }) } catch { /* 失败提示也不能影响业务。 */ } }
  if (!ready || applying || prompting || suspended || !manager || !runtime || promptOwner) return
  if (consented) {
    applying = true; scheduleUpdate()
    try { manager.applyUpdate() } catch { applying = false; consented = false; suspended = true; updateFailed('暂时无法重启更新，请稍后重新进入小程序') }
    return
  }
  const release = acquireSystemPrompt('update'); if (!release) return
  prompting = true; scheduleUpdate(); let answered = false
  /** 原生回调只处理一次，确认后的实际重启仍由安全条件重新判定。 */
  const finish = (confirmed: boolean, failed = false): void => {
    if (answered) return; answered = true; prompting = false
    if (confirmed && ready) consented = true
    else suspended = true
    release()
    if (failed) updateFailed('更新提示暂不可用，稍后重新进入小程序再试')
    scheduleUpdate()
  }
  try { runtime.showModal({ title: '发现新版本', content: '新版本已准备好，重启后即可使用。当前资金操作结束后再更新。', showCancel: false, confirmText: '立即更新', success: result => finish(result.confirm === true), fail: () => finish(false, true) }) }
  catch { finish(false, true) }
}

/** App前后台变化只改变提示可见性；原生支付隐藏不会清除仍在运行的资金任务。 */
export function setUpdateAppVisible(next: boolean): void {
  const resumed = !visible && next; visible = next
  if (resumed) suspended = false
  if (!next && timer !== undefined) { clearTimeout(timer); timer = undefined }
  scheduleUpdate()
}

/**
 * 微信App启动时初始化一次官方更新监听；H5不调用，缺少能力或失败均不阻塞现有业务。
 * @param injected - 测试替换的原生边界；生产省略使用uni官方封装。
 */
export function initializeUpdateManager(injected?: UpdateRuntime): void {
  if (initialized) return
  initialized = true
  runtime = injected || (typeof uni === 'undefined' ? undefined : uni as unknown as UpdateRuntime)
  if (!runtime?.getUpdateManager) return
  try {
    manager = runtime.getUpdateManager()
    manager.onCheckForUpdate(() => { /* 下载由微信官方管理，只有onUpdateReady才可以提示重启。 */ })
    manager.onUpdateReady(() => { ready = true; scheduleUpdate() })
    manager.onUpdateFailed(() => { ready = false; consented = false; suspended = false; updateFailed('新版本下载失败，可稍后重新进入小程序再试') })
  } catch { suspended = true; updateFailed('暂时无法检查更新，可稍后重新进入小程序') }
}

/**
 * 识别真实资金读写与原单同步，保护其它调用页没有显式任务锁时的在途请求。
 * @param path - 领域API提供的固定路径。
 * @param method - 实际请求方法。
 */
export function isFinancialRequest(path: string, method: string): boolean {
  return /^\/wallet(?:\/|\?|$)/.test(path) || /^\/clubs\/[^/]+\/(?:wallet|entries|withdrawals|deposits)(?:\/|\?|$)/.test(path) || /^\/admin\/(?:wallet(?:\/|\?|$)|wallet-settings(?:\?|$)|financial-settings(?:\?|$))/.test(path) || /^\/orders\/[^/]+\/(?:pay|sync-payment|actions|adjustments)(?:\/|\?|$)/.test(path) || /^\/admin\/orders\/[^/]+\/(?:resolve|retry-refund)(?:\?|$)/.test(path) || method !== 'GET' && ['/orders', '/requests'].includes(path)
}
