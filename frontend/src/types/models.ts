/** 文件作用：声明前后端共享的游戏、身份、服务、订单、俱乐部与配置业务模型。 */

/** 用户角色。 */
export type UserRole = 'admin' | 'user' | 'player' | 'provider' | 'clubowner'
/** 官方身份与公开个人资料。 */
export interface User { id: string; nickname: string; avatar: string; bio: string; role: string; status: string; provider_enabled: boolean; profile_cover: string; phone_verified: boolean; phone_masked: string; created_at: string }
/** 玩家主页允许公开展示的个人资料。 */
export type PublicProfile = Pick<User, 'id' | 'nickname' | 'avatar' | 'bio' | 'profile_cover' | 'provider_enabled'>
/** 游戏及可配置主题。 */
export interface Game { id: string; name: string; short_name: string; subtitle: string; primary_color: string; bg_color: string; surface_color: string; text_color: string; icon_url: string; cover_url: string; covers: string[]; categories: string[]; enabled: boolean; sort_order: number }
/** 图片上的独立文案图层。 */
export interface CoverLabel { id?: string | number; text: string; color?: string; background?: string; position?: string; font_size?: number }
/** 已发布服务与公开卡片。 */
export interface Service { /** 服务端返回打手当前有效平台押金，退款处理中不展示。 */ provider_deposit_cents?: number; id: string; game_id: string; provider_id: string; provider_name: string; provider_avatar: string; club_id: string; club_name: string; club_verified: boolean; title: string; description: string; category: string; price_cents: number; unit: string; cover_url: string; labels: CoverLabel[]; tags: string[]; status: string; rating: number; completed_count: number; created_at: string; reviews?: Review[]; is_demo?: boolean }
/** 图文详情块。 */
export interface CommunityBlock { type: 'text' | 'image'; text?: string; image_url?: string }
/** 俱乐部及加入规则。 */
export interface Club { id: string; name: string; description: string; owner_id: string; owner_name: string; game_ids: string[]; status: string; verified: boolean; member_count: number; created_at: string; membership_status?: string; cover_url: string; detail_blocks: CommunityBlock[]; deposit_cents: number; assessment: string }
/** 已公开或允许管理的俱乐部成员。 */
export interface ClubMember { user_id: string; nickname: string; avatar?: string; club_id: string; status: string; certified?: boolean; created_at: string; application_note?: string; requirements_accepted?: boolean; applied_deposit_cents?: number; applied_assessment?: string; assessment_passed?: boolean; review_note?: string }
/** 订单与售后状态快照。 */
export interface Order { id: string; order_no: string; game_id: string; game_name: string; service_id: string; buyer_id: string; buyer_name: string; provider_id: string; provider_name: string; club_id: string; title: string; requirements: string; quantity: number; amount_cents: number; status: string; payment_status: string; delivery_note: string; dispute_reason: string; created_at: string; updated_at: string; refund_status: string; refund_reason: string; refund_reply: string; completed_at: string; service_description: string; unit: string; unit_price_cents: number; is_demo?: boolean; payment_channel?:string; balance_recharge_cents?:number; balance_income_cents?:number; wallet_accounted?:boolean; refund_final?:boolean }
/** 匿名接单大厅允许返回的需求字段。 */
export type PublicJob = Pick<Order, 'id' | 'game_id' | 'game_name' | 'title' | 'quantity' | 'amount_cents' | 'status' | 'payment_status' | 'requirements' | 'created_at' | 'unit' | 'unit_price_cents'>
/** 订单聊天室消息。 */
export interface Message { id: string; order_id: string; sender_id: string; sender_name: string; content: string; created_at: string; sender_role: string; image_url?: string; kind: string }
/** 订单公开评价。 */
export interface Review { id: string; order_id: string; rating: number; content: string; buyer_name: string; created_at: string }
/** 平台基础设置。 */
export interface Settings { site_name: string; announcement: string }
/** 管理员操作审计。 */
export interface Audit { id: string; actor_id: string; action: string; target_id: string; note: string; created_at: string }
/** 协议文档。 */
export interface Agreement { title: string; version: string; content: string; updated_at?: string }
/** 必须主动同意的两份协议。 */
export interface Agreements { user_agreement: Agreement; privacy_policy: Agreement }
/** 登录时传递的协议版本。 */
export interface Consent { agreed?: boolean; user_agreement_version?: string; privacy_policy_version?: string }
/** 一次性微信登录准备票据。 */
export interface WechatPreparation { ticket: string; expires_in: number; agreements: Agreements }
/** 身份建立响应。 */
export interface AuthSession { token: string; user: User }
/** 首次页面加载配置。 */
export interface Bootstrap extends Settings { games: Game[]; dev_mode: boolean }
/** 当前用户订单与收入统计。 */
export interface UserStats { buying_count: number; working_count: number; completed_count: number; earnings_cents: number; provider_working_count: number; provider_completed_count: number; services_count: number; buyer_order_count: number; completed_order_count: number }
/** 俱乐部详情响应。 */
export interface ClubDetail { club: Club; members: ClubMember[]; services: Service[]; my_membership: ClubMember | null; can_manage: boolean }
/** 玩家主页响应。 */
export interface PlayerProfile { profile: PublicProfile; services: Service[]; clubs: Club[] }
/** 上传后由服务端给出的安全图片地址。 */
export interface UploadedImage { id?: string; url: string; sha256?: string; width?: number; height?: number }
/** 微信支付参数，仅由后端签名生成。 */
export interface PaymentParams { timeStamp: string; nonceStr: string; package: string; signType: 'RSA'; paySign: string }
/** 订单支付准备结果。 */
export interface PaymentResult { order: Order; mode: 'dev' | 'wechat' | 'balance'; pay_params?: PaymentParams }
/** 管理端支付公开配置与配置状态。 */
export interface PaymentSettings { merchant_id: string; certificate_serial: string; platform_public_key_id: string; notify_url: string; refund_notify_url: string; api_v3_key_set: boolean; merchant_private_key_set: boolean; platform_public_key_set: boolean; configured: boolean; source: string; storage_available: boolean }
/** 仅在提交期间保留的支付配置秘密。 */
export interface PaymentSettingsInput { merchant_id?: string; certificate_serial?: string; platform_public_key_id?: string; notify_url?: string; refund_notify_url?: string; api_v3_key?: string; merchant_private_key_pem?: string; platform_public_key_pem?: string }
/** 分页和搜索参数。 */
export interface ListQuery { limit?: number; offset?: number; q?: string; game_id?: string; status?: string; refund_status?: string; refund?: string | number }
/** 当前账号与协议版本绑定快照。 */
export interface SessionSnapshot { revision: number; token: string; id: string }
