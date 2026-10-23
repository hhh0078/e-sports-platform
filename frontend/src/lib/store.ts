/** 文件作用：统一游戏主题、平台配置与带版本保护的登录身份状态。 */
import { reactive, computed, ref } from 'vue'
import { token, errorStatus } from '../api/client'
import { getBootstrap } from '../api/games.api'
import { getMe } from '../api/users.api'
import { loginDev, prepareWechat, signInWechat, signOut as revokeSession } from '../api/auth.api'
import type { User, Game, Consent, WechatPreparation } from '../types/models'

/** 共享页面状态。 */
interface PlatformState { support_qr_url: string; support_phone: string; support_wechat: string; user: User | null; games: Game[]; gameId: string; devMode: boolean; announcement: string; siteName: string; ready: boolean }
export const state = reactive<PlatformState>({ support_qr_url: '', support_phone: '', support_wechat: '', user: null, games: [], gameId: String(uni.getStorageSync('jijie_game') || 'delta'), devMode: false, announcement: '', siteName: '集结', ready: false })
/** 配置尚未读取时用于稳定首次布局的游戏主题。 */
const fallbackGame: Game = { id: 'delta', name: '三角洲行动', short_name: '三角洲', subtitle: '', primary_color: '#00e18a', bg_color: '#0a1110', surface_color: '#15201e', text_color: '#f2f5f3', icon_url: '', cover_url: '', covers: [], categories: [], enabled: true, sort_order: 0 }
export const game = computed<Game>(() => state.games.find(item => item.id === state.gameId) || state.games[0] || fallbackGame)
/** 页面局部主题变量；与主游戏选择及本地缓存独立。 */
export type PageTheme = Record<'--accent' | '--bg' | '--surface' | '--text', string>
/** 只读取指定游戏的主题，未知游戏回退当前主游戏，调用不会改变用户选择或写入缓存。 */
export function pageTheme(gameId?: string): PageTheme { const selected = state.games.find(item => item.id === gameId) || game.value; return { '--accent': selected.primary_color || '#00e18a', '--bg': selected.bg_color || '#0a1110', '--surface': selected.surface_color || '#15201e', '--text': selected.text_color || '#f2f5f3' } }
export const themeStyle = computed(() => pageTheme())
let pending: Promise<void> | null = null
let publicPending: Promise<void> | null = null
const sessionVersion = ref(0)
/** 返回会话修订号，供页面丢弃旧身份的异步结果。 */
export function sessionRevision(): number { return sessionVersion.value }
/** 清除会话并使旧请求失效。 */
export function clearSession(): void { sessionVersion.value++; uni.removeStorageSync('jijie_token'); state.user = null }
/** 仅加载公开平台配置；朋友圈单页不准备登录或读取本人资料，同类请求合并。 */
export async function initPublic(force = false): Promise<void> {
  if (publicPending) return publicPending
  if (state.ready && !force) return
  publicPending = (async () => {
    const data = await getBootstrap()
    state.games = data.games || []; state.devMode = !!data.dev_mode; state.announcement = data.announcement || ''; state.siteName = data.site_name || '集结';state.support_qr_url=data.support_qr_url||'';state.support_phone=data.support_phone||'';state.support_wechat=data.support_wechat||''
    if (!state.games.some(item => item.id === state.gameId)) state.gameId = state.games[0]?.id || 'delta'
    state.ready = true
  })()
  try { await publicPending } finally { publicPending = null }
}
/** 正常初始化补齐本人身份，公共页面已读配置不能跳过后续完整会话恢复。 */
export async function init(force = false, publicOnly = false): Promise<void> {
  if (publicOnly) return initPublic(force)
  if (pending) return pending
  if (state.ready && !force && (!token() || state.user)) return
  pending = (async () => {
    await initPublic(force)
    if (token()) { try { await refreshMe() } catch (error) { if (errorStatus(error) !== 401) throw error } }
  })()
  try { await pending } finally { pending = null }
}
/** 刷新本人资料，账号切换后拒绝旧请求回写。 */
export async function refreshMe(): Promise<User | null> {
  const version = sessionVersion.value, requestToken = token()
  try { const user = await getMe(); if (version === sessionVersion.value && requestToken === token()) state.user = user; return state.user }
  catch (error) { if (version !== sessionVersion.value || requestToken !== token()) return state.user; if (errorStatus(error) === 401) clearSession(); throw error }
}
/** 登录仅限本地的体验身份并处理并发身份切换。 */
export async function login(userID: string, consent: Consent = {}): Promise<User | null> { const version = ++sessionVersion.value; const data = await loginDev(userID, consent); if (version !== sessionVersion.value) return state.user; uni.setStorageSync('jijie_token', data.token); state.user = data.user; return data.user }
/** 只准备微信票据，不写入用户身份。 */
export function prepareWechatLogin(code: string): Promise<WechatPreparation> { return prepareWechat(code) }
/** 主动同意协议后建立官方身份。 */
export async function wechatLogin(ticket: string, consent: Consent): Promise<User | null> { const version = ++sessionVersion.value; const data = await signInWechat(ticket, consent); if (version !== sessionVersion.value) return state.user; uni.setStorageSync('jijie_token', data.token); state.user = data.user; return data.user }
/** 撤销会话，迟到的退出响应不能退出新账号。 */
export async function logout(): Promise<void> { const version = ++sessionVersion.value, requestToken = token(); try { await revokeSession() } catch (error) { if (version !== sessionVersion.value || requestToken !== token()) return; if (errorStatus(error) !== 401) throw error } if (version === sessionVersion.value && requestToken === token()) clearSession() }
/** 保存玩家当前选择的游戏。 */
export function selectGame(id: string): void { if (state.games.some(item => item.id === id)) { state.gameId = id; uni.setStorageSync('jijie_game', id) } }
