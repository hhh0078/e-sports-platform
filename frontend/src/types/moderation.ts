/** 文件作用：约束平台图文审核任务、分页汇总及人工处理结果，不包含登录和支付秘密。 */
export type ModerationStatus = 'ai_pending' | 'ai_processing' | 'manual_pending' | 'allowed' | 'rejected' | 'superseded'
/** 内容任务与业务对象字段绑定，版本用于判断已被新内容替代的历史任务。 */
export interface ModerationTask {
  id: string
  target_type: string
  target_id: string
  field_code: string
  field_label: string
  kind: 'text' | 'image'
  status: ModerationStatus
  actor_id: string
  owner_id: string
  created_at: string
  updated_at: string
  reason: string
  preview: string
  image_preview_url?: string
  content_version: number
  attempts: number
  last_error: string
  decision_source: 'ai' | 'manual' | 'inherited' | 'source' | 'system' | ''
}
/** 服务端按实际状态聚合的数量；不由本页已加载行数推导。 */
export type ModerationCounts = Record<ModerationStatus | 'total', number>
/** 游标分页不会把新任务插入误算成旧页偏移。 */
export interface ModerationInbox { items: ModerationTask[]; next_cursor: string; counts: ModerationCounts }
/** 只向后台发送支持的状态枚举。 */
export interface ModerationQuery { status?: ModerationStatus; cursor?: string; limit?: number }
