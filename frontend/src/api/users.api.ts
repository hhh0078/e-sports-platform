/** 文件作用：封装个人资料、统计与公开玩家主页接口。 */
import { request, requestPublic, queryString } from './client'
import type { User, UserStats, PlayerProfile, ListQuery, IdentityStatus, IdentityVerificationInput } from '../types/models'
/** 当前账号资料修改载荷。 */
export type ProfileInput = Partial<Pick<User, 'nickname' | 'bio' | 'avatar' | 'profile_cover'>>
/** 读取当前真实会话用户。 */
export function getMe(): Promise<User> { return request('/me') }
/** 实时读取本人身份核验结果，交易前不使用本地缓存代替。 */
export function getIdentityStatus(): Promise<IdentityStatus> { return request('/me/identity') }
/** 提交本人主动同意的姓名和证件，服务端先判断成年再调用核验。 */
export function verifyIdentity(body: IdentityVerificationInput): Promise<IdentityStatus> { return request('/me/identity/verify', 'POST', body) }
/** 保存当前资料。 */
export function updateProfile(body: ProfileInput): Promise<User> { return request('/me/profile', 'PATCH', body) }
/** 读取本人订单、收入和服务统计。 */
export function getMyStats(): Promise<UserStats> { return request('/me/stats') }
/** 读取公开玩家主页。 */
export function getPlayerProfile(id: string, publicOnly = false, query: ListQuery = {}): Promise<PlayerProfile> { return (publicOnly ? requestPublic : request)('/players/' + encodeURIComponent(id) + queryString(query)) }
