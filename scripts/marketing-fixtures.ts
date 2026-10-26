/** 文件作用：为宣传截图的浏览器请求拦截提供独立虚构数据，不写入数据库、不生成真实登录或资金凭据。 */
import type { Bootstrap, Club, ClubDetail, Game, Message, Order, Service, User } from '../frontend/src/types/models'
import type { AdminArticle, AdminPublicContent } from '../frontend/src/types/content'
import type { AdminAnalytics } from '../frontend/src/types/admin-analytics'
import type { ModerationTask } from '../frontend/src/types/moderation'
import type { Wallet, WalletEntry, WalletTopup, WalletWithdrawal } from '../frontend/src/types/wallet'

/** 只读取公开资源，用于选择本地游戏封面，不继承真实身份或业务编号。 */
interface MarketingResources { users: unknown[]; services: unknown[]; clubs: unknown[]; orders: unknown[] }

/** 将外部输入缩窄为普通对象；无法识别的输入使用空对象。 */
function record(value: unknown): Record<string, unknown> { return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {} }
/** 只保留无查询参数和路径穿越的本地静态图片地址。 */
function image(value: unknown, fallback = ''): string { return typeof value === 'string' && /^\/static\/[a-zA-Z0-9_\-/.]+\.(png|jpg|jpeg|webp|svg)$/i.test(value) && !value.includes('..') ? value : fallback }
/** 只接收非空文字字段，避免把未知对象转成宣传文案。 */
function text(value: unknown, fallback: string): string { return typeof value === 'string' && value.trim() ? value : fallback }
/** 通过实际分页参数裁切固定演示列表，防止页面触底后重复追加。 */
function page<T>(items: T[], url: URL): T[] { const offset = Math.max(0, Number(url.searchParams.get('offset')) || 0), limit = Math.max(1, Math.min(100, Number(url.searchParams.get('limit')) || 20)); return items.slice(offset, offset + limit) }
/** 按页面申请的自然日区间构造虚构曲线，所有数字仅用于演示。 */
function analytics(url?: URL): AdminAnalytics {
  const end = url?.searchParams.get('end') || '2026-10-26', start = url?.searchParams.get('start') || '2026-10-20'
  const first = Date.parse(start + 'T00:00:00Z'), last = Date.parse(end + 'T00:00:00Z'), days = Math.max(1, Math.min(366, Math.round((last - first) / 86400000) + 1))
  const amounts = [128000, 196000, 174000, 268000, 215000, 325000, 296000]
  const series = Array.from({ length: days }, (_, index) => ({ date: new Date(first + index * 86400000).toISOString().slice(0, 10), paid_cents: amounts[index % amounts.length], refund_cents: index % 3 === 1 ? 9800 : 0, paid_orders: [16, 23, 21, 31, 26, 38, 35][index % 7], refunded_orders: index % 3 === 1 ? 1 : 0 }))
  return { period: { start, end, timezone: 'Asia/Shanghai' }, summary: { gmv_cents: series.reduce((sum, item) => sum + item.paid_cents, 0), refund_cents: series.reduce((sum, item) => sum + item.refund_cents, 0), paid_order_count: series.reduce((sum, item) => sum + item.paid_orders, 0), refunded_order_count: series.reduce((sum, item) => sum + item.refunded_orders, 0) }, series, legacy_untracked_orders: 0, dev_mode: true, data_mode: 'local', statistics_note: '宣传页面演示数据，全部人物、订单和金额为虚构示例，不代表真实经营成果。' }
}

/**
 * 构造宣传截图专用数据，只复用公开游戏主题和本地图片，不复制真实用户、订单或联系方式。
 * @param bootstrap - 本地公开初始化响应。
 * @param resources - 本地公开资源列表，私人记录不会被复制。
 * @returns 路由拦截所需的虚构身份、页面内容和关联资源。
 */
export function buildMarketingFixtures(bootstrap: unknown, resources: MarketingResources) {
  const source = record(bootstrap), rawGames = Array.isArray(source.games) ? source.games : []
  const fallbackCover = image(record(resources.services[0]).cover_url, '/static/covers/official/delta-1.jpg')
  const games: Game[] = rawGames.map((raw, index) => { const item = record(raw); const cover = image(item.cover_url, fallbackCover); return { id: text(item.id, 'delta'), name: text(item.name, '三角洲行动'), short_name: text(item.short_name, '三角洲'), subtitle: '', primary_color: text(item.primary_color, '#00e18a'), bg_color: text(item.bg_color, '#0a1110'), surface_color: text(item.surface_color, '#15201e'), text_color: '#f2f5f3', icon_url: image(item.icon_url), cover_url: cover, covers: Array.isArray(item.covers) ? item.covers.map(value => image(value, cover)) : [cover], categories: Array.isArray(item.categories) ? item.categories.filter((value): value is string => typeof value === 'string') : ['陪练', '教学'], enabled: item.enabled !== false, sort_order: index } })
  if (!games.length) games.push({ id: 'delta', name: '三角洲行动', short_name: '三角洲', subtitle: '', primary_color: '#00e18a', bg_color: '#0a1110', surface_color: '#15201e', text_color: '#f2f5f3', icon_url: '', cover_url: fallbackCover, covers: [fallbackCover], categories: ['陪练', '教学'], enabled: true, sort_order: 0 })
  const selected = games.find(item => item.id === 'delta') || games[0], cover = selected.cover_url, stamp = '2026-10-26T10:30:00+08:00'
  const me: User = { id: 'marketing-admin', nickname: '开机电竞·演示', avatar: selected.icon_url || cover, bio: '这里是源码功能演示，展示游戏服务、团队协作与订单管理。', role: 'admin', platform_admin_role: 'root', status: 'active', provider_enabled: true, profile_cover: cover, phone_verified: true, phone_masked: '138****0000', created_at: '2026-10-01T09:00:00+08:00' }
  const names = ['北辰', '云间旅人', '山海主理人', '清风', '星河', '晚风']
  const users: User[] = [me, ...names.map((name, index) => ({ ...me, id: 'marketing-member-' + String(index + 1).padStart(2, '0'), nickname: name + '·演示', role: 'user', platform_admin_role: '' as const, avatar: games[index % games.length].icon_url || cover, bio: '源码演示账号 · 耐心沟通，提前约定服务内容。', phone_masked: '138****' + String(index + 1).padStart(4, '0') }))]
  const clubs: Club[] = [{ id: 'marketing-club-01', name: '山海俱乐部·演示', description: '多游戏服务团队，支持成员管理、订单协作与独立资金管理。', owner_id: me.id, owner_name: me.nickname, game_ids: games.slice(0, 3).map(item => item.id), status: 'active', verified: true, member_count: 12, created_at: stamp, membership_status: 'active', cover_url: cover, avatar_url: selected.icon_url || cover, detail_blocks: [{ type: 'text', text: '一起组队，也一起成长。\n本页面人物、订单及金额均为虚构演示。' }, { type: 'image', image_url: cover, image_width: 1600, image_height: 900 }, { type: 'text', text: '服务约定：提前沟通游戏模式与在线时间，确认需求后再开始服务。' }], deposit_cents: 20000, assessment: '熟悉对应游戏规则，沟通耐心，完成一次团队考核。', commission_rate_bps: 1000, join_note: '欢迎认真负责的队友加入，申请时请说明擅长的游戏。', platform_deposit_cents: 100000, platform_deposit_refunding_cents: 0, platform_deposit_required_cents: 100000, platform_deposit_shortfall_cents: 0, deposit_ready: true, management_role: 'owner', can_manage_admins: true, contact_phone: '', contact_wechat: '', contact_qr_url: '' }]
  const services: Service[] = Array.from({ length: 12 }, (_, index) => {
    const original = record(resources.services[index]), game = index < 8 ? selected : games[(index - 7) % games.length], titles = ['轻松开黑 · 语音陪玩', '地图路线 · 护航协作', '技术陪练 · 新手教学', '周末组队 · 快乐搭子']
    return { id: 'marketing-service-' + String(index + 1).padStart(2, '0'), game_id: game.id, provider_id: users[index % names.length + 1].id, provider_name: users[index % names.length + 1].nickname, provider_avatar: users[index % names.length + 1].avatar, club_id: clubs[0].id, club_name: clubs[0].name, club_verified: true, title: titles[index % titles.length], description: '源码功能演示：支持按游戏、分类筛选可交易服务。\n提前沟通区服、在线时段与本次服务范围，耐心解答游戏问题，重视沟通与团队配合。', category: game.categories[[3, 0, 2, 1][index % 4] % Math.max(1, game.categories.length)] || '陪练', price_cents: [1200, 2900, 1900, 1500][index % 4], unit: '小时', cover_url: image(original.game_id === game.id ? original.cover_url : '', game.covers[index % Math.max(1, game.covers.length)] || game.cover_url), labels: [{ id: 'marketing-label-01', text: ['语音陪玩', '护航协作', '耐心陪练', '快乐组队'][index % 4], color: '#FFFFFF', position: 'top-left', font_size: 18 }, { id: 'marketing-label-02', text: '提前约定时间', color: '#FFFFFF', position: 'top-left', font_size: 12 }], tags: ['演示数据', '耐心沟通', '预约服务'], status: 'active', rating: 4.9, completed_count: [28, 36, 18, 42][index % 4], created_at: stamp, provider_deposit_cents: 50000, provider_club_deposit_cents: 20000, is_demo: true, reviews: [{ id: 'marketing-review-' + index, order_id: 'marketing-order-03', rating: 5, content: '演示评价：需求说明清楚，沟通及时，合作过程很顺利。', buyer_name: '云间旅人·演示', created_at: stamp }] }
  })
  const states = ['in_progress', 'delivered', 'completed', 'pending_payment', 'open', 'disputed']
  const orders: Order[] = states.map((status, index) => { const service = services[index]; return { id: 'marketing-order-' + String(index + 1).padStart(2, '0'), order_no: 'DEMO20261026' + String(index + 1).padStart(4, '0'), game_id: service.game_id, game_name: selected.name, service_id: service.id, buyer_id: me.id, buyer_name: '云间旅人·演示', provider_id: service.provider_id, provider_name: service.provider_name, club_id: clubs[0].id, title: service.title, requirements: '演示需求：今晚20:00开始，约两小时。先熟悉地图，再一起练习团队配合。', quantity: 2, amount_cents: service.price_cents * 2, status, payment_status: status === 'pending_payment' ? 'unpaid' : 'paid', delivery_note: status === 'delivered' || status === 'completed' ? '已完成两小时约定服务，地图要点和战术记录已在聊天中确认。' : '', dispute_reason: status === 'disputed' ? '演示售后：双方对服务时长存在分歧，申请平台协助核对。' : '', created_at: '2026-10-26T19:' + String(index * 5).padStart(2, '0') + ':00+08:00', updated_at: stamp, refund_status: status === 'disputed' ? 'escalated' : 'none', refund_reason: '', refund_reply: '', completed_at: status === 'completed' ? stamp : '', service_description: service.description, unit: '小时', unit_price_cents: service.price_cents, is_demo: true, payment_channel: 'wechat', wallet_accounted: true, settlement: { platform_rate_bps: 500, club_rate_bps: 1000, platform_fee_cents: service.price_cents / 10, club_fee_cents: service.price_cents / 5, provider_income_cents: service.price_cents * 1.7, club_id: clubs[0].id, version: 1 } } })
  const members = users.slice(1).map((user, index) => ({ user_id: user.id, nickname: user.nickname, avatar: user.avatar, club_id: clubs[0].id, status: index === 5 ? 'pending' : 'active', certified: index !== 5, created_at: stamp, phone_masked: user.phone_masked, management_role: index === 0 ? 'deputy' : '', application_note: '擅长团队协作，希望加入俱乐部共同成长。', requirements_accepted: true, applied_deposit_cents: 20000, applied_assessment: clubs[0].assessment, assessment_passed: index !== 5, review_note: index === 5 ? '' : '已完成团队考核。' }))
  const wallet: Wallet = { available_cents: 685000, recharge_cents: 180000, spendable_cents: 865000, pending_cents: 246000, ongoing_cents: 126000, settling_cents: 120000, withdrawal_frozen_cents: 30000, deposit_cents: 50000, deposit_refunding_cents: 0, deposit_required_cents: 50000, deposit_shortfall_cents: 0, deposit_qualified: true, currency: 'CNY', settlement_hold_days: 7, dev_mode: true, withdrawal_min_cents: 100, daily_withdrawal_available: true }
  const entries: WalletEntry[] = [['order_settlement', 7800, '演示 · 服务收入结算'], ['topup', 50000, '演示 · 缴纳平台押金'], ['order_payment', -9800, '演示 · 服务订单付款'], ['order_settlement', 11800, '演示 · 服务收入结算'], ['withdrawal_freeze', -30000, '演示 · 提现申请']].map(([kind, amount, note], index) => ({ id: 'marketing-entry-' + index, kind: String(kind), reference_id: orders[index].id, amount_cents: Number(amount), note: String(note), created_at: stamp, recharge_delta: kind === 'order_payment' ? Number(amount) : 0, available_delta: kind === 'order_settlement' ? Number(amount) : 0, pending_delta: 0, withdrawal_frozen_delta: kind === 'withdrawal_freeze' ? -Number(amount) : 0, deposit_delta: index === 1 ? 50000 : 0, deposit_refunding_delta: 0 }))
  const topups: WalletTopup[] = [{ id: 'marketing-topup-01', user_id: me.id, purpose: 'deposit', amount_cents: 50000, status: 'paid', refunded_cents: 0, created_at: stamp }, { id: 'marketing-topup-02', user_id: me.id, purpose: 'member_club', club_id: clubs[0].id, amount_cents: 20000, status: 'paid', refunded_cents: 0, created_at: stamp }]
  const withdrawals: WalletWithdrawal[] = [{ id: 'marketing-withdrawal-01', user_id: me.id, amount_cents: 30000, status: 'pending_review', created_at: stamp, updated_at: stamp }]
  const publicContent: AdminPublicContent = { revision: 1, global_slides: [{ id: 'marketing-banner-01', kind: 'image', image_url: cover, labels: [{ id: 'marketing-banner-label-01', text: '找到合拍的游戏队友', color: '#FFFFFF', position: 'bottom-left', font_size: 21 }, { id: 'marketing-banner-label-02', text: '服务预约 · 团队协作', color: '#FFFFFF', position: 'bottom-left', font_size: 14 }], target: { kind: 'article', id: 'marketing-article-01' } }, { id: 'marketing-game-slot', kind: 'game_slot', image_url: '', labels: [], target: null }], game_slides: { [selected.id]: [{ id: 'marketing-banner-02', kind: 'image', image_url: selected.covers[1] || cover, labels: [{ id: 'marketing-banner-label-03', text: '沟通需求，轻松组队', color: '#FFFFFF', position: 'bottom-left', font_size: 21 }], target: null }] }, announcement: { id: 'home', version: 1, enabled: false, frequency: 'once', mode: 'text', image_url: '', title: '平台服务公告', body: '演示公告：下单前请确认服务范围、区服与预约时间。', target: null }, page_tips: { player: { text: '下单前请确认服务范围与预约时间', article_id: '' }, worker: { text: '耐心沟通，认真服务', article_id: '' }, orders: { text: '订单进度与沟通记录，一处管理', article_id: '' }, me: { text: '本页面为源码功能演示', article_id: '' } }, service_notice: '请提前确认游戏区服、服务内容与在线时间。', request_notice: '请清楚说明需求和预约时段。', updated_at: stamp }
  const articles: AdminArticle[] = ['平台服务指南', '俱乐部入驻说明', '订单与售后规则', '新手使用说明'].map((title, index) => ({ id: 'marketing-article-' + String(index + 1).padStart(2, '0'), title, summary: ['了解预约服务、订单沟通与验收流程。', '团队入驻、成员管理与订单协作。', '下单前确认需求，完成后及时验收。', '从浏览服务到发布需求，快速上手。'][index], cover_url: games[index % games.length].cover_url, status: index === 3 ? 'draft' : 'published', revision: 1, created_at: stamp, updated_at: stamp, published_at: stamp, body_markdown: '## ' + title + '\n\n本文章为源码功能演示内容。\n\n- 提前沟通服务范围与预约时间\n- 在订单内保留沟通记录\n- 完成服务后进行验收\n\n遇到问题可通过平台客服获得协助。' }))
  const moderation: ModerationTask[] = ['服务简介', '俱乐部介绍', '个人简介', '文章正文'].map((field, index) => ({ id: 'marketing-moderation-' + String(index + 1).padStart(2, '0'), target_type: ['services', 'clubs', 'users', 'articles'][index], target_id: [services[0].id, clubs[0].id, users[1].id, articles[0].id][index], field_code: ['description', 'description', 'bio', 'body_markdown'][index], field_label: field, kind: 'text', status: 'manual_pending', actor_id: users[1].id, owner_id: users[1].id, created_at: stamp, updated_at: stamp, reason: '', preview: ['耐心陪练，提前沟通游戏区服与时间，注重团队配合。', clubs[0].description, users[1].bio, articles[0].summary][index], content_version: 1, attempts: 0, last_error: '', decision_source: '' }))
  const messages: Message[] = [
    { id: 'marketing-chat-01', order_id: orders[0].id, sender_id: 'system', sender_name: '系统', sender_role: 'system', content: '演示订单已创建，请在这里确认服务需求。', kind: 'system', created_at: '2026-10-26T19:50:00+08:00' },
    { id: 'marketing-chat-02', order_id: orders[0].id, sender_id: me.id, sender_name: '云间旅人·演示', sender_role: 'buyer', content: '你好，今晚想练习一下地图路线和团队配合。', kind: 'text', created_at: '2026-10-26T19:52:00+08:00' },
    { id: 'marketing-chat-03', order_id: orders[0].id, sender_id: users[1].id, sender_name: users[1].nickname, sender_role: 'provider', content: '没问题，我们先熟悉地图，再一起练习。按约定20:00开始。', kind: 'text', created_at: '2026-10-26T19:53:00+08:00' },
    { id: 'marketing-chat-04', order_id: orders[0].id, sender_id: 'marketing-club-manager', sender_name: '山海主理人·演示', sender_role: 'club_admin', content: '队友已安排好，有需要可以在订单内随时沟通。', kind: 'text', created_at: '2026-10-26T19:55:00+08:00' },
    { id: 'marketing-chat-05', order_id: orders[0].id, sender_id: me.id, sender_name: '云间旅人·演示', sender_role: 'buyer', content: '好的，需求和时间都确认了，我们准时开始。', kind: 'text', created_at: '2026-10-26T19:56:00+08:00' }
  ]
  const safeBootstrap: Bootstrap = { site_name: '开机电竞', announcement: '', games, dev_mode: true, support_phone: '', support_wechat: '', support_qr_url: '' }
  return { bootstrap: safeBootstrap, me, users, services, clubs, orders, members, wallet, entries, topups, withdrawals, publicContent, articles, moderation, messages, analytics: analytics(), overview: { users: 1286, services: 128, clubs: 18, orders: 896, pending_services: 6, pending_clubs: 2, disputed_orders: 3 }, stats: { buying_count: 5, working_count: 3, completed_count: 28, earnings_cents: 685000, provider_working_count: 3, provider_completed_count: 36, services_count: 12, buyer_order_count: 42, completed_order_count: 28, buyer_pending_payment_count: 1, buyer_working_count: 2, buyer_delivered_count: 1, buyer_refund_count: 1 } }
}

/**
 * 按真实前端接口契约返回宣传演示结果；未知路径由调用方拦截，不能回源私人接口。
 * @param url - 浏览器请求的完整地址。
 * @param fixtures - 当前截图会话使用的虚构数据。
 * @returns 未包装的接口数据；未匹配接口返回 undefined。
 */
export function resolveMarketingResponse(url: URL, fixtures: ReturnType<typeof buildMarketingFixtures>): unknown | undefined {
  const path = url.pathname.replace(/^\/api(?=\/)/, ''), f = fixtures, query = url.searchParams.get('q') || ''
  if (path === '/bootstrap') return f.bootstrap
  if (path === '/me') return f.me
  if (path === '/me/stats') return f.stats
  if (path === '/me/announcement-state') return { id: 'home', version: 1, seen: true }
  if (path === '/subscriptions') return { enabled: false, scenes: [], preferences: {}, authorization_note: '宣传演示，不触发真实微信订阅。' }
  if (path === '/messages') return { items: [], total: 0 }
  if (path === '/admin/overview') return f.overview
  if (path === '/admin/analytics') return analytics(url)
  if (path === '/admin/users') return page(f.users.filter(item => !query || item.nickname.includes(query)), url)
  if (path === '/admin/orders' || path === '/orders') { const status = url.searchParams.get('status'); return page(f.orders.filter(item => (!status || status === 'all' || status === 'working' && ['open', 'accepted', 'in_progress'].includes(item.status) || status === 'refund' && item.refund_status !== 'none' || item.status === status) && (!query || item.title.includes(query))), url) }
  if (path === '/admin/services' || path === '/services') return page(f.services.filter(item => (!url.searchParams.get('game_id') || item.game_id === url.searchParams.get('game_id')) && (!url.searchParams.get('category') || item.category === url.searchParams.get('category')) && (!query || item.title.includes(query))), url)
  if (path === '/admin/clubs' || path === '/clubs') return page(f.clubs, url)
  if (path === '/admin/games') return f.bootstrap.games
  if (path === '/admin/public-content' || path === '/admin/content/public') return f.publicContent
  if (path === '/public-content') { const gameID = url.searchParams.get('game_id') || f.bootstrap.games[0].id; return { ...f.publicContent, game_id: gameID, slides: f.publicContent.global_slides.flatMap(item => item.kind === 'game_slot' ? f.publicContent.game_slides[gameID] || [] : [item]) } }
  if (path === '/admin/articles' || path === '/articles') { const items = f.articles.filter(item => (!query || item.title.includes(query)) && (path.startsWith('/admin') || item.status === 'published')); return { items: page(items, url), total: items.length } }
  if (path.startsWith('/admin/articles/') || path.startsWith('/articles/')) { const item = f.articles.find(article => article.id === path.split('/').pop()); return item ? { ...item, body_html: '<h2>' + item.title + '</h2><p>本文章为源码功能演示内容。</p><p>提前沟通服务范围与预约时间，在订单内保留沟通记录，完成服务后及时验收。</p>' } : undefined }
  if (path === '/admin/moderation') return { items: f.moderation, next_cursor: '', counts: { total: 4, manual_pending: 4, ai_pending: 0, ai_processing: 0, allowed: 0, rejected: 0, superseded: 0 } }
  if (path.startsWith('/admin/moderation/')) return f.moderation.find(item => item.id === path.split('/').pop())
  if (path === '/wallet' || path === '/me/wallet') return f.wallet
  if (path === '/wallet/entries') return page(f.entries, url)
  if (path === '/wallet/topups') return page(f.topups.filter(item => (!url.searchParams.get('purpose') || item.purpose === url.searchParams.get('purpose')) && (!url.searchParams.get('club_id') || item.club_id === url.searchParams.get('club_id'))), url)
  if (path === '/wallet/withdrawals' || path === '/admin/wallet/withdrawals') return page(f.withdrawals, url)
  if (path === '/wallet/deposit-refunds') return []
  if (path === '/financial-rules' || path === '/admin/financial-settings') return { free_platform_rate_bps: 800, club_platform_rate_bps: 500, personal_platform_deposit_cents: 50000, club_creation_deposit_cents: 100000 }
  if (path === '/jobs') return page(f.orders.slice(0, 3).map(item => ({ ...item, status: 'open' })), url)
  const serviceMatch = path.match(/^\/services\/([^/]+)$/)
  if (serviceMatch) return f.services.find(item => item.id === serviceMatch[1])
  const orderMatch = path.match(/^\/orders\/([^/]+)(?:\/(messages|adjustments|reviews))?$/)
  if (orderMatch) { const order = f.orders.find(item => item.id === orderMatch[1]); if (!order) return undefined; if (orderMatch[2] === 'messages') return url.searchParams.has('after_id') || url.searchParams.has('before_id') ? [] : f.messages.map(item => ({ ...item, order_id: order.id })); if (orderMatch[2]) return []; return order }
  const clubMatch = path.match(/^\/clubs\/([^/]+)(?:\/(members|orders|deposits|wallet|entries|withdrawals))?$/)
  if (clubMatch) {
    const club = f.clubs.find(item => item.id === clubMatch[1]); if (!club) return undefined
    if (!clubMatch[2]) { const detail: ClubDetail = { club, members: page(f.members, url), services: page(f.services, url), my_membership: { user_id: f.me.id, nickname: f.me.nickname, club_id: club.id, status: 'active', certified: true, created_at: club.created_at }, can_manage: true, management_role: 'owner', can_manage_admins: true }; return detail }
    if (clubMatch[2] === 'members') return { items: page(f.members, url), total: f.members.length, limit: 20, offset: Number(url.searchParams.get('offset')) || 0, has_more: false }
    if (clubMatch[2] === 'orders') return page(f.orders, url)
    if (clubMatch[2] === 'deposits') return { platform: { amount_cents: 100000, refunding_cents: 0, required_cents: 100000, shortfall_cents: 0, qualified: true, can_manage: true }, membership: { amount_cents: 20000, refunding_cents: 0, required_cents: 20000, shortfall_cents: 0, qualified: true } }
    if (clubMatch[2] === 'wallet') return { ...f.wallet, club_id: club.id, available_cents: 128000, recharge_cents: 0, spendable_cents: 128000, pending_cents: 36000 }
    if (clubMatch[2] === 'entries') return page(f.entries.filter(item => item.kind === 'order_settlement'), url)
    if (clubMatch[2] === 'withdrawals') return []
  }
  const userMatch = path.match(/^\/admin\/users\/([^/]+)(?:\/(wallet|orders|moderation)(?:\/(entries))?)?$/)
  if (userMatch) { const user = f.users.find(item => item.id === userMatch[1]); if (!user) return undefined; if (!userMatch[2]) return { user, permissions: { can_edit: true, can_manage_admins: true, can_send_message: true } }; if (userMatch[2] === 'wallet' && !userMatch[3]) return f.wallet; const items = userMatch[2] === 'orders' ? f.orders : userMatch[2] === 'moderation' ? f.moderation : f.entries; return { items: page<unknown>(items, url), total: items.length, limit: 20, offset: 0, has_more: false } }
  const playerMatch = path.match(/^\/(players|users)\/([^/]+)$/)
  if (playerMatch) { const profile = f.users.find(item => item.id === playerMatch[2]); return profile ? { profile, services: f.services.filter(item => item.provider_id === profile.id), clubs: f.clubs } : undefined }
  return undefined
}
