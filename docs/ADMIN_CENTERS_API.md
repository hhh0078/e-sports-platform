# 平台用户与俱乐部成员管理接口

文件作用：2026-10-26本轮前后端共同契约；所有地址以`/api`为前缀，金额为整数分，服务端鉴权优先于客户端显示。

## 身份与权限

```ts
type PlatformAdminRole = '' | 'root' | 'deputy'
type ClubManagementRole = '' | 'owner' | 'deputy'
interface ManagementPermissions {
  can_edit: boolean
  can_manage_admins: boolean
  can_send_message: boolean
}
interface ManagementPage<T> {
  items: T[]; total: number; limit: number; offset: number; has_more: boolean
}
```

User增加`platform_admin_role`。两种平台管理员仍返回`role:'admin'`，历史管理员默认`root`，普通用户返回空值。副管理员不能授撤任何管理员、修改负责人、冻结或编辑平台管理员。根管理员不能通过副管理员接口修改根角色。

Club及ClubDetail补`management_role`、`can_manage_admins`；现有`can_manage`继续使用。`GET /clubs?managed=1&q=&limit=20&offset=0`仅列本人负责人／有效副管理员组织，包含自有待缴押金草稿；全局平台管理员身份不等于本人管理组织。俱乐部副管理员必须是该组织已认证成员，退会／解除认证同步撤销管理权。

## 平台用户管理

- 现有`GET /admin/users?q=&status=&limit=&offset=`仍返回数组，q增加手机号匹配，搜索在数据库完成；列表不含openid、完整手机号或简介。
- `GET /admin/users/{id}` → `{user: User, permissions: ManagementPermissions}`。user只含公开展示资料、掩码手机号、冻结状态和平台角色，不返回微信私密身份。
- `PATCH /admin/users/{id}/profile` → User。仅接受`nickname?`、`avatar?`、`bio?`、`profile_cover?`；图片走现有官方裁剪压缩／上传及后置审核。角色、余额、手机号、openid等未知字段返回400。
- 既有`PATCH /admin/users/{id}`仅接收`{status:'active'|'frozen'}`；副管理员不得改变管理员状态。
- `PATCH /admin/users/{id}/deputy`接收`{enabled:boolean}` → User，仅根管理员；不能修改根管理员。
- `POST /admin/users/{id}/messages`接收`{title:string,body:string}` → `{recipient_count:1}`，定向站内通知，不能覆盖接收人。
- `GET /admin/users/{id}/wallet` → 现有Wallet展示结构，只读查询，不触发结算或增减资金。
- `GET /admin/users/{id}/wallet/entries` → `ManagementPage<WalletEntry>`。
- `GET /admin/users/{id}/orders?role=all|buyer|provider&status=` → `ManagementPage<Order>`。
- `GET /admin/users/{id}/moderation?status=` → `ManagementPage<ModerationRecord>`，该用户所属／操作的审核记录；不含AI密钥及微信身份。

## 俱乐部成员管理

- `GET /clubs/{id}/members?q=&status=&limit=&offset=` → `ManagementPage<ManagedClubMember>`，仅当前组织管理者或平台管理员。q支持昵称、用户ID、掩码／私密手机号匹配；返回字段仍只有掩码手机号。
- `GET /clubs/{id}/members/{user_id}` → `{user:UserSummary,membership:ClubMember,management_role:ClubManagementRole,permissions:ManagementPermissions,scope_finance:ScopedMemberFinance}`。
- 现有`POST /clubs/{id}/members/{user_id}`保留审核／移除；副管理员不能通过移除另一副管理员间接撤权。不允许俱乐部管理员修改成员全局个人资料。
- `PATCH /clubs/{id}/deputies/{user_id}`接收`{enabled:boolean}`，仅该组织负责人或平台根管理员；副管理员不授撤管理员。
- `POST /clubs/{id}/members/{user_id}/messages`接收`{title,body}`，只给该组织真实成员发站内通知。
- `GET /clubs/{id}/members/{user_id}/orders?status=` → `ManagementPage<Order>`，严格匹配`club_id`及该成员`provider_id`。
- `GET /clubs/{id}/members/{user_id}/finance/entries` → `ManagementPage<ScopedMemberEntry>`，只含本俱乐部订单收入变动及本组织托管押金。
- `GET /clubs/{id}/members/{user_id}/moderation?status=` → `ManagementPage<ModerationRecord>`，只含本组织与该成员关联的审核摘要，交易原证据不作为审核预览返回。

```ts
interface ScopedMemberFinance {
  club_id: string; user_id: string
  settled_income_cents: number // 本俱乐部累计已结算收入，不代表个人钱包剩余可提现金额
  ongoing_cents: number; settling_cents: number
  club_deposit_cents: number; club_deposit_refunding_cents: number
}
interface ScopedMemberEntry extends WalletEntry {
  scope: 'club_order_income' | 'club_member_deposit'
  order_id?: string
}
interface ManagedClubMember extends ClubMember {
  avatar: string; phone_masked: string; management_role: ClubManagementRole
}
interface ModerationRecord {
  id: string; target_type: string; target_id: string; field_label: string
  kind: string; status: string; reason: string; decision_source: string
  attempts: number; created_at: string; updated_at: string
}
```

关联列表均先应用用户／组织范围，再计算total及分页；客户端传入其它user_id、club_id、role或账户ID不能扩大范围。订单、流水与审核只读，不提供修改余额、改账或编辑订单证据接口。

## 组织资金与原负责人绑定

组织副管理员可以查看组织佣金、平台押金、历史并发起提现／押金管理。提现新增`requested_by`保存真实操作者；`user_id`、收款openid与实名始终绑定创建申请时的当前原负责人。请求体不能指定受益人。只有负责人响应可以包含`confirmation`，副管理员和平台管理读取不含官方收款确认参数；确认收款仍由负责人完成。出款前复核负责人未变化，来源审核、每日限制、幂等与原路退款状态机保持。

所有管理写入保留真实操作者审计。资金、身份、认证字段不进入图文审核；可编辑文案和图片继续后置审核。站内管理消息不新增微信订阅场景，仍只保留已有六类业务提醒。
