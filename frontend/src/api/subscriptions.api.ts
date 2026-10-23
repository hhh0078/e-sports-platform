/** 文件作用：封装用户微信提醒与管理员模板、发送记录接口，页面不直接调用传输层。 */
import { request,queryString } from './client'
import type { SubscriptionOverview,SubscriptionScene,SubscriptionAuthorization,SubscriptionSettings,SubscriptionSettingsBody,SubscriptionDelivery,SubscriptionWechatTemplate } from '../types/subscriptions'
/** 获取当前账号可订阅场景和接收偏好。 */
export function getSubscriptions():Promise<SubscriptionOverview>{return request('/subscriptions')}
/** 保存本次微信原生授权回包，不将本地选择视作发送保证。 */
export function recordSubscriptionAuthorization(body:SubscriptionAuthorization):Promise<{ok:boolean}>{return request('/subscriptions/authorizations','POST',body)}
/** 只修改本人选择的接收偏好，关闭偏好不修改微信系统设置。 */
export function updateSubscriptionPreferences(preferences:Partial<Record<SubscriptionScene,boolean>>):Promise<SubscriptionOverview>{return request('/subscriptions/preferences','PATCH',{preferences})}
/** 仅管理员可读取当前模板与固定字段来源。 */
export function getSubscriptionSettings():Promise<SubscriptionSettings>{return request('/admin/subscriptions/settings')}
/** 仅提交主开关与模板映射，不回传只读目录或任意秘密。 */
export function saveSubscriptionSettings(body:SubscriptionSettingsBody):Promise<SubscriptionSettings>{return request('/admin/subscriptions/settings','PATCH',body)}
/** 按服务端分页获取脱敏发送记录，不在客户端读取微信身份。 */
export function listSubscriptionDeliveries(query:{offset?:number;limit?:number;status?:string;scene?:SubscriptionScene}={}):Promise<{items:SubscriptionDelivery[];total:number}>{return request('/admin/subscriptions/deliveries'+queryString(query))}
/** 只重试服务端明确标记安全的记录，不确定结果不能盲目再次发送。 */
export function retrySubscriptionDelivery(id:string):Promise<SubscriptionDelivery>{return request('/admin/subscriptions/deliveries/'+encodeURIComponent(id)+'/retry','POST',{})}

/** 从服务端读取当前小程序已添加的官方模板，调用凭据不进入客户端。 */
export function listSubscriptionTemplates():Promise<{items:SubscriptionWechatTemplate[]}>{return request('/admin/subscriptions/templates')}
