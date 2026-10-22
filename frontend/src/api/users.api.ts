/** 文件作用：封装个人资料、统计与公开玩家主页接口。 */
import { request } from './client'
import type { User, UserStats, PlayerProfile } from '../types/models'
/** 当前账号资料修改载荷。 */
export type ProfileInput = Partial<Pick<User, 'nickname' | 'bio' | 'avatar' | 'profile_cover'>>
/** 读取当前真实会话用户。 */
export function getMe(): Promise<User> { return request('/me') }
/** 保存当前资料。 */
export function updateProfile(body: ProfileInput): Promise<User> { return request('/me/profile', 'PATCH', body) }
/** 读取本人订单、收入和服务统计。 */
export function getMyStats(): Promise<UserStats> { return request('/me/stats') }
/** 读取公开玩家主页。 */
export function getPlayerProfile(id: string): Promise<PlayerProfile> { return request('/players/' + encodeURIComponent(id)) }
