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
/** 用户主动同意协议后创建正式会话。 */
export function signInWechat(ticket: string, consent: Consent): Promise<AuthSession> { return request('/auth/wechat', 'POST', { ticket, ...consent }) }
/** 撤销当前服务端会话。 */
export function signOut(): Promise<void> { return request('/auth/logout', 'POST', {}) }
/** 兑换官方手机号授权 code。 */
export function verifyWechatPhone(code: string): Promise<User> { return request('/me/phone/wechat', 'POST', { code }) }
/** 仅用于本地演示的手机号验证。 */
export function verifyDevPhone(phone: string): Promise<User> { return request('/me/phone/dev', 'POST', { phone_number: phone }) }
