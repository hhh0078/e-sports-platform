/** 文件作用：集中处理交易前的手机号验证弹层，取消时保留当前表单。 */
import { reactive } from 'vue'
import { state, refreshMe } from './store'
import { token } from './api'
import { go } from './format'

export const phoneGate = reactive({ visible: false, busy: false, error: '' })
let resolvePending, pending, gateToken = ''

/** 刷新认证状态，未认证时等待用户明确验证或关闭，返回是否可继续交易。 */
export async function requirePhone() {
  if (!state.user) { go('/pages/login/index'); return false }
  const current = token()
  await refreshMe()
  if (!state.user || current !== token()) return false
  if (state.user.phone_verified) return true
  if (pending) return pending
  gateToken = current
  phoneGate.error = ''; phoneGate.visible = true
  pending = new Promise(resolve => { resolvePending = resolve })
  return pending
}

/** 关闭验证面板并解除等待；跨会话结果一律不允许继续旧交易。 */
export function finishPhone(verified = false) {
  const accepted = verified && gateToken === token() && !!state.user?.phone_verified
  phoneGate.visible = false; phoneGate.busy = false; phoneGate.error = ''
  resolvePending?.(accepted); resolvePending = undefined; pending = undefined; gateToken = ''
}
