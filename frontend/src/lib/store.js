/** 文件作用：统一游戏主题、平台配置与登录身份的响应式状态。 */
import { reactive, computed } from 'vue'
import { api, token } from './api'

export const state = reactive({ user: null, games: [], gameId: uni.getStorageSync('jijie_game') || 'delta', devMode: false, announcement: '', siteName: '集结', ready: false })
export const game = computed(() => state.games.find(item => item.id === state.gameId) || state.games[0] || { id: 'delta', name: '三角洲行动', short_name: '三角洲', primary_color: '#00e18a', bg_color: '#0a1110', surface_color: '#15201e', text_color: '#f2f5f3', covers: [], categories: [] })
export const themeStyle = computed(() => ({ '--accent': game.value.primary_color || '#00e18a', '--bg': game.value.bg_color || '#0a1110', '--surface': game.value.surface_color || '#15201e', '--text': game.value.text_color || '#f2f5f3' }))
let pending, sessionVersion = 0

/** 返回会话变更序号，页面据此丢弃切换账号前发出的异步结果。 */
export function sessionRevision() { return sessionVersion }

/** 清除当前本地会话，并让此前读取身份的请求全部失效。 */
export function clearSession() { sessionVersion++; uni.removeStorageSync('jijie_token'); state.user = null }

/** 加载平台配置与已有会话，复用并发初始化请求。 */
export async function init(force = false) {
  if (pending) return pending
  if (state.ready && !force) return
  pending = (async () => {
    const data = await api('/bootstrap')
    state.games = data.games || []; state.devMode = !!data.dev_mode; state.announcement = data.announcement || ''; state.siteName = data.site_name || '集结'
    if (!state.games.some(item => item.id === state.gameId)) state.gameId = state.games[0]?.id || 'delta'
    if (token()) { try { await refreshMe() } catch (error) { if (error.status !== 401) throw error } }
    state.ready = true
  })()
  try { await pending } finally { pending = null }
}

/** 从服务端刷新当前登录用户，避免使用过期角色信息。 */
export async function refreshMe() {
  const version = sessionVersion, requestToken = token()
  try {
    const user = await api('/me')
    if (version === sessionVersion && requestToken === token()) state.user = user
    return state.user
  } catch (error) {
    if (version !== sessionVersion || requestToken !== token()) return state.user
    if (error.status === 401) clearSession()
    throw error
  }
}

/** 切换本地测试身份，真实生产环境由服务端禁用此接口。 */
export async function login(userID, consent = {}) { const version = ++sessionVersion; const data = await api('/auth/dev', 'POST', { user_id: userID, ...consent }); if (version !== sessionVersion) return state.user; uni.setStorageSync('jijie_token', data.token); state.user = data.user; return data.user }

/** 仅交换微信临时登录票据，不创建账号或提前写入会话。 */
export async function prepareWechatLogin(code) { return api('/auth/wechat/prepare', 'POST', { code }) }

/** 用户主动同意当前协议后，用一次性票据建立真实会话。 */
export async function wechatLogin(ticket, consent) { const version = ++sessionVersion; const data = await api('/auth/wechat', 'POST', { ticket, ...consent }); if (version !== sessionVersion) return state.user; uni.setStorageSync('jijie_token', data.token); state.user = data.user; return data.user }

/** 退出当前账号，并撤销服务端会话。 */
export async function logout() {
  const version = ++sessionVersion, requestToken = token()
  try { await api('/auth/logout', 'POST', {}) }
  catch (error) { if (version !== sessionVersion || requestToken !== token()) return; if (error.status !== 401) throw error }
  if (version === sessionVersion && requestToken === token()) clearSession()
}

/** 选择游戏并记住用户偏好。 */
export function selectGame(id) { if (state.games.some(item => item.id === id)) { state.gameId = id; uni.setStorageSync('jijie_game', id) } }
