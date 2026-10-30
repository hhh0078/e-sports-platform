/** 文件作用：封装管理员运行模式配置与官方虚拟支付报价接口。 */
import { request } from './client'
import type { CommerceSettings, CommerceSettingsInput, VirtualQuote, PaymentPlatform } from '../types/commerce'
/** 读取管理员脱敏的双模式配置。 */
export function getCommerceSettings():Promise<CommerceSettings>{return request('/admin/commerce-settings')}
/** 加密保存运行模式、费率和本次输入的秘密。 */
export function updateCommerceSettings(body:CommerceSettingsInput):Promise<CommerceSettings>{return request('/admin/commerce-settings','PATCH',body)}

/** 确认前读取当前账号的消费、手续费和金币差额，失败时禁止付款。 */
export function getVirtualQuote(serviceCents:number,platform:PaymentPlatform,context:{order_id?:string;adjustment_id?:string}={}):Promise<VirtualQuote>{return request('/payments/virtual/quote','POST',{service_cents:serviceCents,platform,...context})}
