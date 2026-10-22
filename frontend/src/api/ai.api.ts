/** 文件作用：封装平台管理员AI配置和后端真实兼容接口测试，页面不直接拼接网络请求。 */
import { request } from './client'
import type { AISettings,AISettingsInput,AIConnectionResult } from '../types/ai'
/** 读取公开AI配置及密钥存在状态，秘密不回显。 */
export function getAISettings():Promise<AISettings>{return request('/admin/ai-settings')}
/** 保存本次配置，API Key为空时服务端保留已有秘密。 */
export function saveAISettings(body:AISettingsInput):Promise<AISettings>{return request('/admin/ai-settings','PATCH',body)}
/** 由后端使用已保存配置测试真实连接，不在客户端伪造连通结果。 */
export function testAIConnection():Promise<AIConnectionResult>{return request('/admin/ai-settings/test','POST',{})}
