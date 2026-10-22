/** 文件作用：统一交易前手机号验证等待与跨账号保护，关闭不丢失表单。 */
import { reactive } from 'vue'
import { state, refreshMe, sessionRevision } from './store'
import { token } from '../api/client'
import { go } from './format'
export const phoneGate = reactive({ visible: false, busy: false, error: '', epoch: 0 })
let resolvePending: ((value: boolean) => void) | undefined
let pending: Promise<boolean> | undefined
let gateToken = ''
let gateUserID = '', gateSessionRevision = -1
/** 检查弹层代次和真实会话归属；旧绑定请求不得结束或修改后来的手机号验证。 */
export function isCurrentPhoneGate(epoch: number): boolean { return phoneGate.visible && epoch === phoneGate.epoch && !!gateUserID && gateUserID === state.user?.id && gateToken === token() && gateSessionRevision === sessionRevision() }
/** 检查官方认证状态，未认证时等待用户授权或关闭。 */
export async function requirePhone(): Promise<boolean> {
  if (!state.user) { go('/pages/account/login'); return false }
  const current = token(), userID = state.user.id, revision = sessionRevision()
  await refreshMe()
  if (!state.user || current !== token() || userID !== state.user.id || revision !== sessionRevision()) return false
  if (state.user.phone_verified) return true
  if (pending && isCurrentPhoneGate(phoneGate.epoch)) return pending
  if (pending) finishPhone(false, phoneGate.epoch)
  gateToken = current; gateUserID = userID; gateSessionRevision = revision
  phoneGate.epoch++; phoneGate.error = ''; phoneGate.busy = false; phoneGate.visible = true
  pending = new Promise(resolve => { resolvePending = resolve })
  return pending
}
/** 只结束指定代次的手机号验证，省略代次仅供当前主动关闭操作兼容调用。 */
export function finishPhone(verified = false, epoch = phoneGate.epoch): void {
  if (epoch !== phoneGate.epoch) return
  const accepted = verified && isCurrentPhoneGate(epoch) && !!state.user?.phone_verified, resolve = resolvePending
  phoneGate.epoch++; phoneGate.visible = false; phoneGate.busy = false; phoneGate.error = ''
  resolvePending = undefined; pending = undefined; gateToken = ''; gateUserID = ''; gateSessionRevision = -1
  resolve?.(accepted)
}
