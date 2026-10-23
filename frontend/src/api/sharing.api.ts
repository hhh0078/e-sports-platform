/** 文件作用：封装分享记录和匿名公开需求接口，页面不接触传输层。 */
import { request, requestPublic } from './client'
import type { ShareInput, ShareMetadata } from '../types/sharing'
import type { PublicJob } from '../types/models'
/** 为当前登录人创建一份公开内容分享；服务端校验目标可见性。 */
export function createShare(input: ShareInput): Promise<ShareMetadata> { return request('/shares', 'POST', input) }
/** 读取经目标可见性复核的公开分享元数据。 */
export function getShare(id: string): Promise<ShareMetadata> { return requestPublic('/shares/' + encodeURIComponent(id)) }
/** 读取白名单字段公开需求，绝不读取私有订单详情。 */
export function getPublicRequest(id: string): Promise<PublicJob> { return requestPublic('/jobs/' + encodeURIComponent(id)) }
