/** 文件作用：封装俱乐部目录、详情、装修、申请、退出和成员审核接口。 */
import { request, queryString } from './client'
import type { Club, ClubDetail, ClubMember, ListQuery, Order } from '../types/models'
/** 俱乐部列表筛选。 */
export interface ClubQuery extends ListQuery { mine?: 0 | 1 }
/** 新俱乐部或装修资料。 */
export type ClubInput = Pick<Club, 'name' | 'description' | 'game_ids'> & Partial<Pick<Club, 'cover_url' | 'detail_blocks' | 'deposit_cents' | 'assessment'>>
/** 读取公开俱乐部或本人挂靠。 */
export function listClubs(query: ClubQuery = {}): Promise<Club[]> { return request('/clubs' + queryString(query)) }
/** 读取俱乐部资料及有权限的成员信息。 */
export function getClub(id: string): Promise<ClubDetail> { return request('/clubs/' + encodeURIComponent(id)) }
/** 提交新俱乐部审核。 */
export function createClub(body: ClubInput): Promise<Club> { return request('/clubs', 'POST', body) }
/** 保存俱乐部装修。 */
export function updateClub(id: string, body: ClubInput): Promise<Club> { return request('/clubs/' + encodeURIComponent(id), 'PATCH', body) }
/** 提交挂靠申请，保留已阅读的加入条件。 */
export function joinClub(id: string, body: { application_note: string; requirements_accepted: boolean }): Promise<{ ok: boolean }> { return request('/clubs/' + encodeURIComponent(id) + '/join', 'POST', body) }
/** 退出当前挂靠。 */
export function leaveClub(id: string): Promise<void> { return request('/clubs/' + encodeURIComponent(id) + '/leave', 'POST', {}) }
/** 审核成员并记录考核结果。 */
export function reviewClubMember(id: string, userID: string, body: { status: string; assessment_passed: boolean; review_note: string }): Promise<{ ok: boolean }> { return request('/clubs/' + encodeURIComponent(id) + '/members/' + encodeURIComponent(userID), 'POST', body) }
/** 读取管理员有权查看的团队订单。 */
export function listClubOrders(id: string, query: ListQuery = {}): Promise<Order[]> { return request('/clubs/' + encodeURIComponent(id) + '/orders' + queryString(query)) }
