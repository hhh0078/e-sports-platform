/** 文件作用：统一五类公开分享路由、朋友圈单页限制与官方分享图片菜单。 */
import type { ShareInput, ShareKind, ShareMetadata } from '../types/sharing'
/** 原生微信中与分享有关的官方能力。 */
interface SharingRuntime { getEnterOptionsSync?: () => { scene?: number }; getLaunchOptionsSync?: () => { scene?: number }; canIUse?: (name: string) => boolean; showShareMenu?: (options: { menus: Array<'shareAppMessage' | 'shareTimeline'> }) => void; showShareImageMenu?: (options: { path: string; needShowEntrance: boolean; entrancePath?: string; success: () => void; fail: (error: unknown) => void }) => void }
/** 返回运行时微信对象，H5中不存在时不伪造原生能力。 */
function sharingRuntime(): SharingRuntime | undefined { return (globalThis as unknown as { wx?: SharingRuntime }).wx }
/** 朋友圈单页只能公开浏览，不进行微信登录或交易。 */
export function isTimelinePreview(): boolean { const runtime = sharingRuntime(); const options = runtime?.getEnterOptionsSync?.() || runtime?.getLaunchOptionsSync?.(); return options?.scene === 1154 }
/** 对未知路由值校验五类内容，不接受订单等私有内容。 */
export function isShareKind(value: unknown): value is ShareKind { return typeof value === 'string' && ['home', 'service', 'request', 'player', 'club'].includes(value) }
/** 将原生scene或普通share_id严格解析为32位记录编号。 */
export function readShareID(options: Record<string, string | undefined> = {}): string { const raw = options.scene || options.share_id || ''; let decoded = raw; try { decoded = decodeURIComponent(raw) } catch { return '' } return /^[a-f0-9]{32}$/.test(decoded) ? decoded : '' }
/** 仅允许后端分享合同中的五个公开页面，防止扫码转入任意路由。 */
export function shareTargetPath(metadata: Pick<ShareMetadata, 'kind' | 'path' | 'query'>): string { const paths: Record<ShareKind, string> = { home: 'pages/main/index', service: 'pages/services/detail', request: 'pages/requests/detail', player: 'pages/community/players/detail', club: 'pages/community/clubs/detail' }; if (!isShareKind(metadata.kind) || metadata.path !== paths[metadata.kind] || /[\r\n#]/.test(metadata.query)) throw new Error('分享目标无效'); return '/' + metadata.path + (metadata.query ? '?' + metadata.query : '') }
/** 为分享创建页生成只含业务目标的编码路由。 */
export function shareCreatePath(input: ShareInput): string { if (!isShareKind(input.kind)) throw new Error('不支持分享此内容'); return '/pages/shares/create?kind=' + input.kind + '&target_id=' + encodeURIComponent(input.target_id) + (input.game_id ? '&game_id=' + encodeURIComponent(input.game_id) : '') }
/** 打开微信官方图片分享菜单，包含朋友圈入口；取消不视为失败。 */
export function showPosterShareMenu(path: string, entrancePath?: string): Promise<void> { const runtime = sharingRuntime(); if (!runtime?.showShareImageMenu || runtime.canIUse?.('showShareImageMenu') === false) return Promise.reject(new Error('请在新版微信中打开海报，或保存后分享到朋友圈')); return new Promise((resolve, reject) => runtime.showShareImageMenu!({ path, needShowEntrance: true, entrancePath, success: () => resolve(), fail: error => { const text = typeof error === 'object' && error !== null && 'errMsg' in error ? String(error.errMsg) : ''; if (/cancel/i.test(text)) resolve(); else reject(error) } })) }
/** 直接分享公开内容时构造安全内部链接，访客无需建立带分享人的海报记录。 */
export function shareInputTargetPath(input: ShareInput): string { const paths: Record<ShareKind, string> = { home: 'pages/main/index', service: 'pages/services/detail', request: 'pages/requests/detail', player: 'pages/community/players/detail', club: 'pages/community/clubs/detail' }; if (!isShareKind(input.kind)) throw new Error('分享类型无效'); if (input.kind === 'home') return '/'+paths.home+'?tab=player'+(input.game_id?'&game_id='+encodeURIComponent(input.game_id):''); if (!input.target_id) throw new Error('分享目标不存在'); return '/'+paths[input.kind]+'?id='+encodeURIComponent(input.target_id) }

/** 在完整微信页面启用官方好友和朋友圈菜单；不读取身份，H5与1154单页不调用该原生能力。 */
export function enablePublicShareMenu(): void {
  // #ifdef MP-WEIXIN
  if (isTimelinePreview()) return
  const runtime=sharingRuntime()
  if (runtime?.showShareMenu && runtime.canIUse?.('showShareMenu') !== false) runtime.showShareMenu({menus:['shareAppMessage','shareTimeline']})
  // #endif
}
