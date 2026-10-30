/** 文件作用：声明小程序虚拟币和公众号普通支付的配置、报价与临时签名契约。 */
/** 平台选择的登录及支付运行模式。 */
export type CommerceMode = 'mini_program' | 'official_account'
/** 微信渠道手续费承担方。 */
export type FeeBearer = 'platform' | 'buyer'
/** 客户端平台仅参与报价，官方支付结果仍由服务端核验。 */
export type PaymentPlatform = 'android' | 'ios'
/** 可公开的交易方式和手续费，不包含任何签名秘密。 */
export interface CommercePublic { mode:CommerceMode; fee_bearer:FeeBearer; android_fee_bps:number; ios_fee_bps:number; virtual_configured?:boolean; official_configured?:boolean; sms_configured?:boolean; callback_configured?:boolean }
/** 管理员可编辑的公开配置，秘密仅显示是否已保存。 */
export interface CommerceSettings extends CommercePublic { offer_id:string; virtual_env:0|1; official_app_id:string; official_redirect_url:string; app_key_set:boolean; sandbox_app_key_set:boolean; official_secret_set:boolean; storage_available:boolean }
/** 密钥输入留空表示保留，离开配置页必须清空。 */
export interface CommerceSettingsInput extends CommercePublic { offer_id:string; virtual_env:0|1; official_app_id:string; official_redirect_url:string; app_key?:string; sandbox_app_key?:string; official_secret?:string }
/** 虚拟币面额和用户现金实付分开记录，单位均为整数分。 */
export interface VirtualQuote { service_cents:number; fee_cents:number; total_cents:number; topup_cents:number; coin_balance_cents:number; fee_bearer:FeeBearer; fee_bps:number; platform:PaymentPlatform }
/** 微信客户端拉起签名，不包含 session_key 或 AppKey。 */
export interface VirtualPaymentParams { mode:'short_series_coin'; signData:string; paySig:string; signature:string }
