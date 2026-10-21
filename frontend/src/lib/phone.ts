/** 文件作用：统一交易前手机号验证等待与跨账号保护，关闭不丢失表单。 */
import { reactive } from 'vue'
import { state, refreshMe } from './store'
import { token } from '../api/client'
import { go } from './format'
export const phoneGate = reactive({ visible: false, busy: false, error: '' })
let resolvePending: ((value: boolean) => void) | undefined
let pending: Promise<boolean> | undefined
let gateToken = ''
/** 检查官方认证状态，未认证时等待用户授权或关闭。 */
export async function requirePhone(): Promise<boolean> { if (!state.user) { go('/pages/account/login'); return false } const current = token(); await refreshMe(); if (!state.user || current !== token()) return false; if (state.user.phone_verified) return true; if (pending) return pending; gateToken = current; phoneGate.error = ''; phoneGate.visible = true; pending = new Promise(resolve => { resolvePending = resolve }); return pending }
/** 结束手机号验证，仅允许原账号继续原交易。 */
export function finishPhone(verified = false): void { const accepted = verified && gateToken === token() && !!state.user?.phone_verified; phoneGate.visible = false; phoneGate.busy = false; phoneGate.error = ''; resolvePending?.(accepted); resolvePending = undefined; pending = undefined; gateToken = '' }
