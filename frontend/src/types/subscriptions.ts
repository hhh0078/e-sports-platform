/** 文件作用：定义微信订阅场景、用户偏好、模板配置与脱敏发送记录的前后端契约。 */
/** 服务端硬编码的可订阅业务场景。 */
export type SubscriptionScene='provider_order'|'provider_refund'|'buyer_accepted'|'club_member'|'club_dispute'|'admin_club'
/** 微信本次原生授权结果，fail 表示请求本身没有完成。 */
export type SubscriptionAuthorizationStatus='accept'|'reject'|'ban'|'filter'|'fail'
/** 用户可见场景，不包含商户或微信身份秘密。 */
export interface SubscriptionSceneOption {scene:SubscriptionScene;label:string;description:string;template_id:string;enabled:boolean;available:boolean;reason:string;preference:boolean}
/** 当前账号真实可用场景和偏好；不是剩余发送额度保证。 */
export interface SubscriptionOverview {enabled:boolean;scenes:SubscriptionSceneOption[];preferences:Partial<Record<SubscriptionScene,boolean>>;authorization_note:string}
/** 管理员可配置的固定安全字段来源。 */
export type SubscriptionFieldSource='title'|'status'|'amount'|'time'|'order_no'|'target_name'|'count'|'summary'|'service_name'|'actor_name'|'club_name'|'created_time'
/** 单个业务场景的微信模板映射。 */
export interface SubscriptionSceneSettings {scene:SubscriptionScene;enabled:boolean;template_id:string;fields:Record<string,SubscriptionFieldSource>}
/** 管理员配置读写主体。 */
export interface SubscriptionSettingsBody {enabled:boolean;scenes:SubscriptionSceneSettings[]}
/** 管理员配置读取附带固定来源和场景说明。 */
export interface SubscriptionSettings extends SubscriptionSettingsBody {sources:SubscriptionFieldSource[];scene_catalog:Array<{scene:SubscriptionScene;label:string;description:string}>}
/** 服务端发送状态，未知状态仍只读展示。 */
export type SubscriptionDeliveryStatus='pending'|'preparing'|'sending'|'sent'|'skipped_disabled'|'skipped_config'|'skipped_preference'|'skipped_no_authorization'|'skipped_identity'|'expired'|'failed_retryable'|'failed_final'|'uncertain'
/** 发送记录不含 OpenID、令牌、模板原文或支付凭证。 */
export interface SubscriptionDelivery {id:string;user_id:string;scene:SubscriptionScene;title:string;event:string;status:SubscriptionDeliveryStatus;reason:string;error_code:number;attempts:number;created_at:string;updated_at:string;sent_at?:string;expires_at:string;can_retry:boolean}
/** 原生授权仅上报实际返回结果与本次关联场景。 */
export interface SubscriptionAuthorization {request_id:string;results:Array<{scene:SubscriptionScene;template_id:string;status:SubscriptionAuthorizationStatus}>}

/** 当前小程序官方可用模板，不包含示例个人数据。 */
export interface SubscriptionWechatTemplate {template_id:string;title:string;type:number|string;content:string;fields:Array<{key:string;label:string;type:string}>}
