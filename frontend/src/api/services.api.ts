/** 文件作用：封装服务搜索、详情、发布编辑和暂停接口。 */
import { request, queryString } from './client'
import type { ListQuery, Service } from '../types/models'
/** 服务筛选参数。 */
export interface ServiceQuery extends ListQuery { category?: string; mine?: 0 | 1; sort?: string }
/** 服务发布载荷。 */
export type ServiceInput = Pick<Service, 'game_id' | 'title' | 'description' | 'category' | 'price_cents' | 'unit' | 'cover_url' | 'labels' | 'tags' | 'club_id'>
/** 分页搜索服务。 */
export function listServices(query: ServiceQuery = {}): Promise<Service[]> { return request('/services' + queryString(query)) }
/** 读取单条可见服务。 */
export function getService(id: string): Promise<Service> { return request('/services/' + encodeURIComponent(id)) }
/** 发布新服务，进入审核。 */
export function createService(body: ServiceInput): Promise<Service> { return request('/services', 'POST', body) }
/** 保存服务修改并重新审核。 */
export function updateService(id: string, body: ServiceInput): Promise<Service> { return request('/services/' + encodeURIComponent(id), 'PATCH', body) }
/** 暂停本人服务。 */
export function pauseService(id: string): Promise<Service> { return request('/services/' + encodeURIComponent(id) + '/pause', 'POST', {}) }
