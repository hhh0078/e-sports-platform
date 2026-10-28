# 开机电竞 · 接口协定

文件作用：约定 Go 服务与 uni-app 客户端的数据、角色及交易接口。

## 基础约定

- API 地址 `http://127.0.0.1:8090/api`。JSON 请求。成功 `{ "data": ... }`，失败 `{ "error": "中文说明" }` 并使用正确 HTTP 状态。
- ID 全部为字符串。金额为整数分。时间为 RFC3339 字符串。数据库 PostgreSQL，Redis 存放会话/限流。分页 GET 采用 limit/offset，列表统一返回数组（最大100条）。
- Bearer token 登录。普通人同时拥有玩家和打手功能，服务发布资料必须完整。管理员权限只来自服务端。
- `GET /health` 返回数据库、Redis健康状态。

## 实体

User: `id,nickname,avatar,bio,role` (user/admin), `status` (active/frozen), `provider_enabled` boolean, `created_at`。登录附 `token,user`。

Game: `id,name,short_name,subtitle,primary_color,bg_color,surface_color,text_color,cover_url,covers` (string[]), `categories` (string[]), `enabled,sort_order`。7个 id: delta,honor,peace,arena,valorant,cfm,tft。封面 `/static/covers/{id}-1.jpg`、`{id}-2.jpg`。

CoverLabel: `id,text,color,background,position`（top-left/top-center/top-right/middle-left/middle-center/middle-right/bottom-left/bottom-center/bottom-right）, `font_size`（12~24）。最多6项，每项最多12字；默认 bottom-left。

Service: `id,game_id,provider_id,provider_name,provider_avatar,club_id,club_name,club_verified,title,description,category,price_cents,unit,cover_url,labels` (CoverLabel[]), `tags` (string[]), `status` (pending/active/rejected/paused), `rating,completed_count,created_at`。

Club: `id,name,description,owner_id,owner_name,game_ids` (string[]), `status` (pending/active/rejected), `verified,member_count,created_at`。`mine=1` 时增加当前用户的 `membership_status` (owner/active/pending/rejected/left)。ClubMember: `user_id,nickname,status` (pending/active/rejected/left), `created_at`。

Order: `id,order_no,game_id,service_id,buyer_id,buyer_name,provider_id,provider_name,club_id,title,requirements,quantity,amount_cents,status,payment_status,delivery_note,dispute_reason,created_at,updated_at`。status: pending_payment/open/accepted/in_progress/delivered/completed/cancelled/disputed/refunded。payment_status: unpaid/paid/refunded。订单有 `game_name`、`completed_at`。服务约定快照为 `service_description,unit,unit_price_cents`：指定服务下单时从服务端保存描述、计价单位和单价；公开需求使用创建时说明、单位“次”及需求金额。旧订单缺失快照时保持空值，不用当前商品资料补写。微信 OpenID、支付和退款流水仅在服务器持久化，所有订单 API 响应隐藏这些字段。

退款增补：Order 增加 `refund_status` (none/requested/rejected/escalated/processing/approved), `refund_reason,refund_reply`。申请退款不抹去原交易阶段；requested/escalated/processing 期间暂停交付及结算。真实退款先持久化稳定退款号及 processing，再请求微信；未知、超时和异常结果保持冻结，只有已核验成功才标记 refunded。拒绝后买家可再次申请（补证据）或申请平台介入。交易完成后7天内允许申请售后，退款成功扣除对应已结算收入。

PublicJob: 匿名接单大厅仅返回 `id,game_id,game_name,title,quantity,amount_cents,status,payment_status,requirements,created_at,unit,unit_price_cents`。其中 requirements 固定提示接单后查看详情，不返回实际需求、服务说明快照、买家/打手身份、订单编号或支付私密字段。

Message: `id,order_id,sender_id,sender_name,content,created_at`。Review: `id,order_id,rating,content,buyer_name,created_at`。

Message 增加 `sender_role` (buyer/provider/club_admin/admin/system), `image_url` 可选，`kind` (text/image/system)，不可编辑或删除。所有关键订单状态自动写入系统消息；俱乐部负责人以订单下单时锁定的 club_id 判断权限，当前挂靠变化不改变历史订单证据归属。

## 公共与身份

- GET `/bootstrap` -> `{games,dev_mode,site_name,announcement}`
- GET `/dev/accounts` -> User[]，仅 DEV_MODE 下启用；种子账户 ids: player,worker,clubowner,admin,outsider。
- POST `/auth/dev` `{user_id}` -> `{token,user}`；仅 DEV_MODE=true。
- POST `/auth/wechat` `{code}` -> `{token,user}`；服务端使用配置的 appid/secret 调用微信 code2session，未配置明确返回503。
- POST `/auth/logout` -> `{ok:true}`
- GET `/me` -> User
- PATCH `/me` `{nickname,bio,provider_enabled}` -> User；不能修改 role/status。
- GET `/me/stats` -> `{buying_count,working_count,completed_count,earnings_cents}`

## 游戏与服务

- GET `/games` -> Game[]（公共只返回启用项目）
- GET `/services?game_id=&category=&q=&mine=1` -> Service[]，公共仅 active，mine 要登录。
- GET `/services/{id}` -> Service（reviews 为实际完成且不在退款处理中的订单评价；无评价时可能省略）
- POST `/services` 与 PATCH `/services/{id}` body 包括 game_id,title,description,category,price_cents,unit,cover_url,labels,tags,club_id。只有本人/管理员。修改需要重新审核。
- POST `/services/{id}/pause` -> Service（所有者暂停服务）

## 订单与交易

- POST `/orders` `{service_id,quantity,requirements}` -> Order，价格从服务端读取；禁止自己下单。
- POST `/requests` `{game_id,title,requirements,amount_cents}` -> Order（自定义需求，先 pending_payment，支付后 open）。
- GET `/orders?role=buyer|provider|all&status=` -> Order[]，只能见自己的订单（admin 也经单独后台接口）。
- GET `/jobs?game_id=&limit=&offset=` -> PublicJob[]，只返回已支付、待抢单且未在退款处理中的公开需求。使用固定字段白名单，不序列化私密需求及参与人信息。
- GET `/orders/{id}` -> Order，仅买家/接单人/所属俱乐部主人/管理员。
- POST `/orders/{id}/pay` `{}` -> `{order,mode,pay_params?}`。开发支付事务原子、重复幂等；真实支付在配置校验通过后持订单锁创建预支付，提交意图后才返回支付参数，以签名回调或验签查询为准。创建预支付与取消关单互斥。
- POST `/orders/{id}/sync-payment` `{}` -> Order，仅买家/管理员。查询微信补偿付款或退款回调延迟，开发模式仅返回当前订单。
- POST `/orders/{id}/actions` `{action,note,club_id?}` -> Order。action: claim（抢公开需求）、accept（接指定服务订单）、start、deliver（note必填）、confirm、cancel、dispute（note必填）。严格状态机、行锁、权限和付款约束。claim 可选 club_id，仅允许代表自己为负责人或已通过挂靠、已启用且支持当前游戏的俱乐部接单，并快照归属；不传则独立接单。其它 action 不接受 club_id。
- 同一 actions 接口增加 request_refund（买方申请，note必填）、approve_refund（卖方同意）、reject_refund（卖方拒绝，note必填）、escalate（买方申请平台介入）、withdraw_refund（买方撤销未裁决申请）。取消只允许未支付或尚未接单，不允许卖家绕过买家确认直接完成。
- GET/POST `/orders/{id}/messages` -> Message[] / Message，POST `{content,image_url?}`。参与者可见，可发文字/图片证据。
- GET messages 支持 `limit/offset`，默认取最近最多 100 条并按 `(created_at,id)` 升序返回。`before_id` 取游标之前最近一页，`after_id` 取游标之后最早一页（可循环补齐超过 100 条的新增消息）；两个游标不能同时传，游标必须属于当前订单。
- POST `/orders/{id}/evidence` multipart `file` -> `{url,sha256}`，仅订单参与者/关联俱乐部负责人/管理员。图片限 PNG/JPEG/WebP，5MiB，魔数校验。GET 返回文件需授权，通过客户端下载鉴权或安全的短期访问链接显示。POST messages 可附 `image_url`，服务器验证归属该订单，图片证据不可跨订单引用。
- GET `/orders/{id}/evidence-link?evidence_id=` -> `{url}`。订单参与者、关联俱乐部负责人或管理员可重新签发 15 分钟有效图片链接；必须与该订单一致，拒绝跨单及公开封面。
- GET `/evidence/{id}?expires=&signature=` 返回图片原文件；有效短期签名或订单参与者 Bearer 身份可读取。
- POST `/orders/{id}/review` `{rating,content}` -> Review，仅完成后买家，一单一次。
- GET `/orders/{id}/reviews` -> Review[]。

## 俱乐部

- GET `/clubs?mine=1` -> Club[]，需登录，返回自己创建或申请过挂靠的俱乐部及 membership_status；包含待审、拒绝和退出记录。公共 `/clubs` 仅返回 active。发布服务或代表组织接单应筛选 `status=active` 且 `membership_status=owner/active`。
- POST `/clubs` `{name,description,game_ids}` -> Club（pending）
- GET `/clubs/{id}` -> `{club,members,my_membership}`；普通用户可见已通过成员及自己的申请记录，管理员/owner可见全部申请和历史状态。
- PATCH `/clubs/{id}` `{name,description,game_ids}` -> Club，仅本人/管理员。
- POST `/clubs/{id}/join` -> `{ok:true}`，创建挂靠申请；退出或拒绝后重新申请回到 pending。
- POST `/clubs/{id}/leave` -> `{ok:true}`，退出后记录保留为 left，服务认证随之撤销，历史订单俱乐部归属保留；负责人不能退出自己创建的俱乐部。
- POST `/clubs/{id}/members/{user_id}` `{status}` -> `{ok:true}`，owner/admin审核申请或移除（rejected）。
- GET `/clubs/{id}/orders` -> Order[]，仅 owner/admin，限当前俱乐部。

## 管理后台（全部需要 admin）

- GET `/admin/overview` -> `{users,services,orders,clubs,revenue_cents,pending_services,pending_clubs,disputed_orders}`
- GET `/admin/users|services|clubs|orders|games|audit?q=&limit=&offset=` -> 对应数组，默认最多 100 条。q 最多 100 字，在数据库搜索后分页，覆盖全部记录而非前端已加载记录。用户按昵称/ID/简介/角色/状态，服务按标题/说明/分类/游戏/提供者/俱乐部，订单按订单号/标题/游戏/参与人/状态/需求，审计按操作者/动作/目标/说明搜索；名称大小写不敏感，关键词按字面量匹配。
- GET `/admin/orders?refund=1` 只返回 refund_status 为 requested/escalated/processing 或旧 status=disputed 的订单，可与 q、limit、offset 组合。
- PATCH `/admin/users/{id}` `{status}`；禁止冻结自己，冻结立即禁用请求。
- PATCH `/admin/services/{id}` `{status}`（active/rejected/paused）。
- PATCH `/admin/clubs/{id}` `{status,verified}`；只有 active 可以认证。
- POST `/admin/games` 或 PATCH `/admin/games/{id}` 完整 Game；验证颜色、封面路径、分类。
- POST `/admin/covers` multipart `file` -> `{url,sha256}`，管理员上传 PNG/JPEG/WebP，最大 5 MiB 并校验图片尺寸。返回相对 `/api/media/{id}`，可加入 games.covers/cover_url；客户端展示时补 API origin，不将 localhost 固化到数据库。GET `/media/{id}` 仅公开此类封面，不能读取私密订单证据。
- POST `/admin/orders/{id}/resolve` `{resolution,note}` -> Order，resolution 为 refund/resume/complete，仅处理 escalated 或旧 disputed。resume 驳回退款并保留原履约阶段；complete 仅对 delivered/completed 已交付订单可用。真实退款先进入 processing，不将通道受理视为到账成功。
- POST `/admin/orders/{id}/retry-refund` `{}` -> Order，仅对真实支付、processing 且付款已核验订单重试既有退款号。不会生成第二个退款号；未知或异常结果仍冻结，可继续通过 sync-payment 查询。
- PATCH `/admin/settings` `{announcement,site_name}`，GET `/admin/settings`。

## 姓名身份证核验

- GET `/me/identity` → `{is_minor,verified,verified_at?}`，仅当前登录用户。`is_minor` 默认 `true`，包括尚未核验；只有成年人二要素核验成功后置 `false`，支付仍需同时检查私密通过事实。
- POST `/me/identity/verify` `{name,idcard,consent:true}` → 同上。先验证身份证和满18周岁；未成年返回403及 `code:"IDENTITY_UNDERAGE"`，不发起上游调用。非200业务码显示提供方 `msg`；不一致不能通过。
- GET `/admin/identity-settings` → `{has_api_key,configured,storage_available,api_url}`，仅平台管理员，永不返回密钥。
- PUT `/admin/identity-settings` `{api_key}` → 同上。空密钥保留，加密保存且审计不包含秘密。
- 未核验用户发起下单、订单支付、补款、充值及押金付款返回428及 `code:"IDENTITY_REQUIRED"`。查询、退款及已确认收款回调保持原流程。详见[身份核验规则](IDENTITY_VERIFICATION.md)。

## 微信资金通知

- POST `/payments/wechat/notify`：微信 APIv3 付款回调，先 RSA 验签和 AES-GCM 解密，再核验商户、AppID、买家、订单、金额、币种及唯一支付流水。
- POST `/payments/wechat/refund-notify`：退款回调，核验事件/状态、商户、原支付流水、退款号及全额金额。ABNORMAL/CLOSED 保持 processing 冻结等待核查；成功回调原子更新订单、统计及系统消息。
- 通知成功返回 HTTP 204；错误按微信格式 `{code:"FAIL",message}` 返回相应状态。开发模式拒绝真实资金通知。

## AI 审查与系统消息

- GET `/admin/ai-settings` → `{base_url,model,prompt,has_api_key,ready,mode,configuration_notice}`，仅平台管理员；`ready`只代表配置完整，不代表已经连通或支持图片。
- PATCH `/admin/ai-settings` 可选 `{base_url,model,prompt,api_key,clear_api_key}`；空密钥保留，显式清除与写入不能同时使用。秘密加密存储，读取从不返回密钥。
- POST `/admin/ai-settings/test` `{}` → `{decision,reason,mode:"ai"}`，使用已保存配置和固定电竞文字真实请求，不存在模拟测试成功。
- 正式环境所有用户图文写入都须审核。拒绝返回422 `{error,code:"moderation_rejected"}`；缺少配置/超时/协议错误返回503 `{error,code:"moderation_unavailable"}`；失败不提交业务数据。审核不占用订单/资金行锁，图片发送实际验证字节。
- GET `/messages?limit=&offset=` → `{items:[{id,user_id,title,body,created_at}],total}`，仅当前用户未读服务器副本，GET不会标记已读或删除。
- DELETE `/messages/{id}` → `{ok:true}`，仅删除当前用户自己的服务器副本，重复请求幂等。客户端必须先持久化全文和已读，存储失败不可调用；此接口不操作订单聊天室。
- POST `/admin/messages` `{recipient_id?,title,body}` → `{recipient_count}`，仅平台管理员，未传接收人则向全部活跃真实账号广播；标题1—60字、正文1—5000字，操作保留审计。
- 本机消息缓存按用户隔离；已读历史不再依赖服务器正文，换设备无法恢复已删除副本。见[审核与消息规则](AI_MODERATION_MESSAGES.md)。

## 钱包与经营统计

钱包、充值、余额支付、收入提现、押金原路退还与管理员审核接口以 `wallet_routes.go` 及[钱包配置与验收](WALLET_SETUP_AND_TEST.md)为准。金额均为整数分，充值不可提现，验收七天无售后才释放服务收入，未知远端结果保持原单查询。

GET `/admin/analytics?start=YYYY-MM-DD&end=YYYY-MM-DD` 返回上海自然日周期、真实服务实付/成功退款汇总及逐日序列；默认七天、自定义最多366天。充值、押金、提现及演示资金不计GMV，缺少可靠资金时间的历史订单单独待核账。字段及口径见[后台控制台](ADMIN_CONSOLE.md)。

## 安全与测试

服务端校验所有字符串长度、整数金额范围、封面合法来源及标签配置；用户不能伪造价格、身份、认证状态。所有审计落库。订单收入依据真实托管账本及售后期释放，不能仅凭completed状态虚增可提现余额；退款不能重复计入。开发账号、开发支付和模拟审核在生产必须关闭，配置不全不允许静默模拟成功。


## 最低押金与补缴（2026-10-23）

文件作用：补充个人及俱乐部平台押金的当前接口契约，金额均为整数分。

- `GET /api/financial-rules` 返回两档平台费率、`personal_platform_deposit_cents` 和 `club_creation_deposit_cents`，后两项分别是个人和俱乐部最低额。
- 管理员 `GET/PATCH /api/admin/financial-settings` 管理同一配置。旧客户端省略新增个人最低额字段时保留原值；明确传0才清零。
- 本人 `GET /api/wallet` 返回实际托管 `deposit_cents`、`deposit_required_cents`、`deposit_shortfall_cents` 和 `deposit_qualified`；公开服务/个人主页仅返回达标的展示金额。
- `GET /api/clubs/{id}/deposits` 的 `platform` 返回 `amount_cents`、`required_cents`、`shortfall_cents`、`qualified`；成员押金保留独立 `membership` 分区。
- 缴纳仍用 `POST /api/wallet/topups`：个人用途 `deposit`、俱乐部用途 `club_platform` 并携带 `club_id`。新意图须让累计托管金额达到当前最低额；差额允许到分。
- 门槛变化只影响资格与新意图校验，不自动移动已有押金。旧意图恢复和已核验付款回调继续按原金额入账；退款仍走原有条件、原渠道与原单幂等。
