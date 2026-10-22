/** 文件作用：封装平台管理员的审核、游戏配置、协议和支付配置接口。 */
import { request, queryString } from './client'
import type { Agreements, Game, ListQuery, Settings, PaymentSettings, PaymentSettingsInput, User, Service, Club, Order, Audit } from '../types/models'
/** 后台统计。 */
export type AdminOverview = Record<string, number>
/** 后台资源类型对应业务模型。 */
export interface AdminResources { users: User; services: Service; clubs: Club; orders: Order; games: Game; audit: Audit }
/** 可审核的资源。 */
export type ModerationResource = 'users' | 'services' | 'clubs'
/** 读取平台统计。 */
export function getAdminOverview(): Promise<AdminOverview> { return request('/admin/overview') }
/** 按资源类型分页搜索，返回对应模型。 */
export function listAdminResource<K extends keyof AdminResources>(resource: K, query: ListQuery = {}): Promise<AdminResources[K][]> { return request('/admin/' + resource + queryString(query)) }
/** 审核用户、服务或俱乐部。 */
export function moderateResource<K extends ModerationResource>(resource: K, id: string, body: { status?: string; verified?: boolean }): Promise<AdminResources[K]> { return request('/admin/' + resource + '/' + encodeURIComponent(id), 'PATCH', body) }
/** 新建游戏主题。 */
export function createGame(body: Game): Promise<Game> { return request('/admin/games', 'POST', body) }
/** 保存游戏主题。 */
export function updateGame(id: string, body: Game): Promise<Game> { return request('/admin/games/' + encodeURIComponent(id), 'PATCH', body) }
/** 保存平台公告和名称。 */
export function updateSettings(body: Settings): Promise<Settings> { return request('/admin/settings', 'PATCH', body) }
/** 读取后台平台设置。 */
export function getAdminSettings(): Promise<Settings> { return request('/admin/settings') }
/** 平台裁定订单售后。 */
export function resolveOrder(id: string, resolution: string, note = ''): Promise<Order> { return request('/admin/orders/' + encodeURIComponent(id) + '/resolve', 'POST', { resolution, note }) }
/** 复核微信资金状态并重试同退款单。 */
export function retryOrderRefund(id: string): Promise<Order> { return request('/admin/orders/' + encodeURIComponent(id) + '/retry-refund', 'POST', {}) }
/** 读取管理员协议编辑稿。 */
export function getAdminAgreements(): Promise<Agreements> { return request('/admin/agreements') }
/** 发布新版本协议。 */
export function updateAgreements(body: Agreements): Promise<Agreements> { return request('/admin/agreements', 'PATCH', body) }
/** 仅返回商户公开字段及秘密配置状态。 */
export function getPaymentSettings(): Promise<PaymentSettings> { return request('/admin/payment-settings') }
/** 保存商户配置；秘密为空表示保留已有配置。 */
export function updatePaymentSettings(body: PaymentSettingsInput): Promise<PaymentSettings> { return request('/admin/payment-settings', 'PATCH', body) }

/** 读取按上海自然日核实的服务成交与成功退款曲线，不包含充值和押金。 */
export function getAdminAnalytics(query:{start?:string;end?:string}={}):Promise<import('../types/admin-analytics').AdminAnalytics>{return request('/admin/analytics'+queryString(query))}
