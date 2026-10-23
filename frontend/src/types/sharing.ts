/** 文件作用：定义五类公开内容的分享记录、分享人及安全路由模型。 */
/** 服务端允许公开分享的内容类型。 */
export type ShareKind = 'home' | 'service' | 'request' | 'player' | 'club'
/** 创建分享记录时仅传目标，分享人始终由服务端从会话取得。 */
export interface ShareInput { kind: ShareKind; target_id: string; game_id?: string }
/** 可公开展示的分享人资料，不含手机号和微信身份。 */
export interface SharePerson { id: string; nickname: string; avatar_url: string }
/** 真实分享元数据，path无首斜线且query无问号。 */
export interface ShareMetadata { id: string; kind: ShareKind; target_id: string; title: string; description: string; game_id: string; cover_url: string; sharer: SharePerson; path: string; query: string; code_url: string; env_version: string; amount_cents?: number; unit?: string }
/** 一次实时绘制得到的三个用途不同的图片。 */
export interface ShareImages { poster: string; friend: string; timeline: string }
