# 集结后端

文件作用：说明 Go API 的启动、数据布局、安全边界与自动验收方式。

## 本地启动

需要 PostgreSQL 及 Redis；开发库专用名称为 `jijie_dev`，不读取或修改其它项目数据库。

```sh
cd backend
DEV_MODE=true go run ./cmd/server
```

默认监听 `127.0.0.1:8090`，PostgreSQL 使用 `postgres://postgres:postgres@127.0.0.1:5432/jijie_dev?sslmode=disable`，Redis 使用 `127.0.0.1:56379`。可通过 `ADDR`、`DATABASE_URL`、`REDIS_ADDR`、`REDIS_PASSWORD` 覆盖，`REDIS_PREFIX` 可隔离共用 Redis 的环境。

`APP_ENV=production` 禁止同时设置 `DEV_MODE=true`。所有本地测试身份与测试付款均要求显式设置 `DEV_MODE=true`，真实登录需要 `WECHAT_APP_ID` 和 `WECHAT_APP_SECRET`。正式部署使用独立数据库、Redis 凭证与 HTTPS 域名，不能复用演示库。

## 数据设计

- PostgreSQL JSONB 保存有明确 Go 类型及服务端验证的领域记录，各表、字段均带中文 COMMENT；订单按买家、接单人、状态建立索引，服务按游戏和状态建立索引。
- 每个订单的状态、退款、系统事件均在同一事务提交；状态变更先获得订单行锁。评价具有订单唯一索引。
- 账户和权限来自服务器。Redis 保存 30 天登录会话，每次请求再读取 PostgreSQL 用户状态，冻结账号立即影响已登录会话。
- 管理员身份不会扩展普通订单列表的查询范围；管理所有订单必须经过管理员专用接口。俱乐部负责人只访问锁定到该俱乐部的历史订单。
- 交易金额为整数分。服务价格、打手、组织在下单时快照。重复支付不产生重复事件，抢单只有一个胜者，收入来自最终完成且未在退款处理中的订单聚合，不维护易失真的重复计数余额。
- 图片以原文件和 SHA-256 摘要保存在 PostgreSQL。订单图片使用 15 分钟 HMAC 签名链接；管理员上传的封面单独标记为公开，私密证据不能从公开入口读取。PNG/JPEG/WebP 校验魔数、尺寸和 5 MiB 大小。
- 普通服务编辑进入重新审核；离开俱乐部立即撤销展示认证，历史订单保持原组织参与权限。

## 自动测试

```sh
# 不依赖服务的状态机与边界测试
go test ./...
# 真实 PostgreSQL + Redis，多身份与并发交易测试
TEST_INTEGRATION=true go test -race -count=1 -v ./...
# 静态检查
go vet ./...
```

集成测试在 `jijie_dev` 内建立随机独立 schema 和 Redis 键前缀，结束后清理，不覆盖开发演示记录。包含买家/打手/俱乐部负责人/管理员四方沟通、越权隔离、16 次并发付款、20 次并发抢单、退款平台介入、售后收入回冲、图片证据签名、用户冻结、封面自定义发布、开发入口生产关闭等。

## 退款状态

`none → requested → approved/rejected`，买家可从 `requested/rejected` 申请 `escalated`。平台可退款或驳回后继续履约；只有已经交付的订单才可裁决完成。`requested/escalated/processing` 期间停止交付和结算。交易完成后 7 天内可申请售后；成功退款冲回对应收入。微信退款处理中用 `processing` 表示，确认回调前不把资金标记为已退。

接口与前端协定见 `../docs/API.md`；正式微信支付配置和回调说明由支付适配器与项目交付文档补充。

## 列表与长聊天

管理列表支持 `q`、`limit`、`offset`，关键词在 PostgreSQL 中先搜索再分页；订单额外支持 `refund=1` 只查询待处理退款。默认最多返回 100 条，排序增加 ID 作为稳定次序。

消息默认返回最近 100 条并按时间升序排列。`before_id` 获取某条消息之前最近的一页；`after_id` 获取某条消息之后最早的一页，可逐页补齐休眠期间的新增消息。两个游标不能同时传，且必须属于当前订单。通过 `/orders/{id}/evidence-link?evidence_id=...` 为旧图片重新签发访问链接。

`/clubs?mine=1` 的 `membership_status` 区分 `owner/active/pending/rejected/left`；退出仍保留历史申请记录，重新申请回到 pending。领取公开需求可携带 `club_id`，必须为当前游戏的有效负责人/已通过成员；不传则独立接单。
