/** 文件作用：提供微信身份准备、协议、登录、手机号授权与登出接口。 */
import { request } from './client'
import type { Agreements, AuthSession, Consent, User, WechatPreparation } from '../types/models'
/** 读取当前版本用户协议和隐私政策。 */
export function getAgreements(): Promise<Agreements> { return request('/agreements') }
/** 读取仅限开发环境的体验身份。 */
export function listDevAccounts(): Promise<User[]> { return request('/dev/accounts') }
/** 创建开发身份会话，正式环境服务端拒绝。 */
export function loginDev(userID: string, consent: Consent = {}): Promise<AuthSession> { return request('/auth/dev', 'POST', { user_id: userID, ...consent }) }
/** 仅换取一次性票据，不建立会话。 */
export function prepareWechat(code: string): Promise<WechatPreparation> { return request('/auth/wechat/prepare', 'POST', { code }) }
/** 用户主动同意协议后创建正式会话，返回服务端新注册标记而非按头像推测。 */
export function signInWechat(ticket: string, consent: Consent): Promise<AuthSession> { return request('/auth/wechat', 'POST', { ticket, ...consent }) }
/** 撤销当前服务端会话。 */
export function signOut(): Promise<void> { return request('/auth/logout', 'POST', {}) }
/** 兑换官方手机号授权 code。 */
export function verifyWechatPhone(code: string): Promise<User> { return request('/me/phone/wechat', 'POST', { code }) }
/** 仅用于本地演示的手机号验证。 */
export function verifyDevPhone(phone: string): Promise<User> { return request('/me/phone/dev', 'POST', { phone_number: phone }) }
/** 由服务端创建绑定协议同意和站内返回地址的公众号授权。 */
export function startOfficialLogin(returnPath:string,consent:Consent):Promise<{url:string;expires_in:number}>{return request('/auth/official/start','POST',{return_path:returnPath,...consent})}
/** 兑换公众号一次性code/state，返回原授权的站内路径。 */
export function prepareOfficialLogin(code:string,state:string):Promise<WechatPreparation&{return_path:string}>{return request('/auth/official/prepare','POST',{code,state})}
/** 为本人发送真实短信验证码，服务端未配置时明确失败。 */
export function sendPhoneSMS(phone:string):Promise<{expires_in:number;retry_after:number}>{return request('/me/phone/sms/send','POST',{phone_number:phone})}
/** 核验本人短信验证码，不接受客户端模拟结果。 */
export function verifyPhoneSMS(phone:string,code:string):Promise<User>{return request('/me/phone/sms/verify','POST',{phone_number:phone,code})}
/** 小程序重新登录只刷新本人的支付会话密钥，不向客户端返回密钥。 */
export function refreshWechatPaymentSession(code:string):Promise<{expires_in:number}>{return request('/auth/wechat/session','POST',{code})}
/** 仅为已配置公众号同源的当前页面取得JSSDK签名，不返回微信票据。 */
export function getOfficialSDKConfig(url:string):Promise<{appId:string;timestamp:number;nonceStr:string;signature:string}>{return request('/auth/official/js-sdk','POST',{url})}
